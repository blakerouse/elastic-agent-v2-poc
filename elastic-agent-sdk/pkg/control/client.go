// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package control

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"google.golang.org/grpc/credentials"
	"io"
	"sync"
	"time"

	"google.golang.org/grpc"

	"github.com/blakerouse/elastic-agent-sdk/pkg/proto"
	ujson "github.com/blakerouse/elastic-agent-sdk/pkg/utils/json"
)

// CheckinMinimumTimeout is the amount of time the client must send a new checkin even if the status has not changed.
const CheckinMinimumTimeout = time.Second * 30

// InitialConfigIdx is the initial configuration index the client starts with. 0 represents no config state.
const InitialConfigIdx = 0

// ActionResponseInitID is the initial ID sent to Agent on first connect.
const ActionResponseInitID = "init"

// ActionErrUndefined is returned to Elastic Agent as result to an action request
// when the request action is not registered in the client.
var ActionErrUndefined = ujson.MustMarshal(map[string]string{
	"error": "action undefined",
})

// ActionErrUnmarshableParams is returned to Elastic Agent as result to an action request
// when the request params could not be un-marshaled to send to the action.
var ActionErrUnmarshableParams = ujson.MustMarshal(map[string]string{
	"error": "action params failed to be un-marshaled",
})

// ActionErrInvalidParams is returned to Elastic Agent as result to an action request
// when the request params are invalid for the action.
var ActionErrInvalidParams = ujson.MustMarshal(map[string]string{
	"error": "action params invalid",
})

// ActionErrUnmarshableResult is returned to Elastic Agent as result to an action request
// when the action was performed but the response could not be marshalled to send back to
// the agent.
var ActionErrUnmarshableResult = ujson.MustMarshal(map[string]string{
	"error": "action result failed to be marshaled",
})

// Action is an action the client exposed to the Elastic Agent.
type Action interface {
	// Name of the action.
	Name() string

	// Execute performs the action.
	Execute(map[string]interface{}) (map[string]interface{}, error)
}

// StateInterface defines how to handle config and stop requests.
type StateInterface interface {
	// OnConfig is called when the Elastic Agent is requesting that the configuration
	// be set to the provided new value.
	OnConfig(string)

	// OnStop is called when the Elastic Agent is requesting the application to stop.
	OnStop()

	// OnError is called when an errors occurs communicating with Elastic Agent.
	//
	// These error messages are not given by the Elastic Agent, they are just errors exposed
	// from the client-side GRPC connection.
	OnError(error)
}

// Client manages the state and communication to the Elastic Agent.
type Client interface {
	// Run runs the client.
	Run(ctx context.Context) error
	// Status updates the status and sents it to the Elastic Agent.
	Status(status proto.StateObserved_Status, message string, payload map[string]interface{}) error
}

// client manages the state and communication to the Elastic Agent.
type client struct {
	conn            *proto.Agent
	impl            StateInterface
	actions         map[string]Action
	cfgIdx          uint64
	cfg             string
	expected        proto.StateExpected_State
	observed        proto.StateObserved_Status
	observedMessage string
	observedPayload string

	ctx     context.Context
	wg      sync.WaitGroup
	client  proto.ControlClient
	cfgLock sync.RWMutex
	obsLock sync.RWMutex

	// overridden in tests to make fast
	minCheckTimeout time.Duration
}

// New creates a control connection to Elastic Agent.
func New(conn *proto.Agent, impl StateInterface, actions []Action) Client {
	actionMap := map[string]Action{}
	if actions != nil {
		for _, act := range actions {
			actionMap[act.Name()] = act
		}
	}
	return &client{
		conn:            conn,
		impl:            impl,
		actions:         actionMap,
		cfgIdx:          InitialConfigIdx,
		expected:        proto.StateExpected_RUNNING,
		observed:        proto.StateObserved_STARTING,
		observedMessage: "Starting",
		minCheckTimeout: CheckinMinimumTimeout,
	}
}

// Run runs the client.
func (c *client) Run(ctx context.Context) error {
	cert, err := tls.X509KeyPair(c.conn.PeerCert, c.conn.PeerKey)
	if err != nil {
		return err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(c.conn.CaCert)
	trans := credentials.NewTLS(&tls.Config{
		ServerName:   c.conn.ServerName,
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	})
	conn, err := grpc.DialContext(ctx, c.conn.Addr, grpc.WithTransportCredentials(trans))
	if err != nil {
		return err
	}
	c.client = proto.NewControlClient(conn)
	c.ctx = ctx
	c.startCheckin()
	c.startActions()
	return nil
}

// Status updates the current status of the client in the Elastic Agent.
func (c *client) Status(status proto.StateObserved_Status, message string, payload map[string]interface{}) error {
	payloadStr := ""
	if payload != nil {
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		payloadStr = string(payloadBytes)
	}
	c.obsLock.Lock()
	c.observed = status
	c.observedMessage = message
	c.observedPayload = payloadStr
	c.obsLock.Unlock()
	return nil
}

// startCheckin starts the go routines to send and receive check-ins
//
// This starts 3 go routines to manage the check-in bi-directional stream. The first
// go routine starts the stream then starts one go routine to receive messages and
// another go routine to send messages. The first go routine then blocks waiting on
// the receive and send to finish, then restarts the stream or exits if the context
// has been cancelled.
func (c *client) startCheckin() {
	c.wg.Add(1)

	go func() {
		defer c.wg.Done()
		for {
			select {
			case <-c.ctx.Done():
				// stopped
				return
			default:
			}

			checkinCtx, checkinCancel := context.WithCancel(c.ctx)
			checkinClient, err := c.client.Checkin(checkinCtx)
			if err != nil {
				c.impl.OnError(err)
				continue
			}

			var checkinWG sync.WaitGroup
			done := make(chan bool)

			// expected state check-ins
			checkinWG.Add(1)
			go func() {
				defer checkinWG.Done()
				for {
					expected, err := checkinClient.Recv()
					if err != nil {
						if err != io.EOF {
							c.impl.OnError(err)
						}
						close(done)
						return
					}

					if c.expected == proto.StateExpected_STOPPING {
						// in stopping state, do nothing with any other expected states
						continue
					}
					if expected.State == proto.StateExpected_STOPPING {
						// Elastic Agent is requesting us to stop.
						c.expected = expected.State
						c.impl.OnStop()
						continue
					}
					if expected.ConfigStateIdx != c.cfgIdx {
						// Elastic Agent is requesting us to update config.
						c.cfgLock.Lock()
						c.cfgIdx = expected.ConfigStateIdx
						c.cfg = expected.Config
						c.cfgLock.Unlock()
						c.impl.OnConfig(expected.Config)
						continue
					}
				}
			}()

			// observed state check-ins
			checkinWG.Add(1)
			go func() {
				defer checkinWG.Done()

				var lastSent time.Time
				var lastSentCfgIdx uint64
				var lastSentStatus proto.StateObserved_Status
				var lastSentMessage string
				var lastSentPayload string
				for {
					select {
					case <-done:
						return
					case <-time.After(500 * time.Millisecond):
					}

					c.cfgLock.RLock()
					cfgIdx := c.cfgIdx
					c.cfgLock.RUnlock()

					c.obsLock.RLock()
					observed := c.observed
					observedMsg := c.observedMessage
					observedPayload := c.observedPayload
					c.obsLock.RUnlock()

					sendMessage := func() error {
						err := checkinClient.Send(&proto.StateObserved{
							Token:          c.conn.Token,
							ConfigStateIdx: cfgIdx,
							Status:         observed,
							Message:        observedMsg,
							Payload: 		observedPayload,
						})
						if err != nil {
							c.impl.OnError(err)
							checkinCancel()
							return err
						}
						lastSent = time.Now()
						lastSentCfgIdx = cfgIdx
						lastSentStatus = observed
						lastSentMessage = observedMsg
						lastSentPayload = observedPayload
						return nil
					}

					// On start keep trying to send the initial check-in.
					if lastSent.IsZero() {
						if sendMessage() != nil {
							return
						}
						continue
					}

					// Send new status when it has changed.
					if lastSentCfgIdx != cfgIdx || lastSentStatus != observed || lastSentMessage != observedMsg || lastSentPayload != observedPayload {
						if sendMessage() != nil {
							return
						}
						continue
					}

					// Send when more than 30 seconds has passed without any status change.
					if time.Now().Sub(lastSent) >= c.minCheckTimeout {
						if sendMessage() != nil {
							return
						}
						continue
					}
				}
			}()

			// wait for both send and recv go routines to stop before
			// starting a new stream.
			checkinWG.Wait()
		}
	}()
}

// startActions starts the go routines to send and receive actions
//
// This starts 3 go routines to manage the actions bi-directional stream. The first
// go routine starts the stream then starts one go routine to receive messages and
// another go routine to send messages. The first go routine then blocks waiting on
// the receive and send to finish, then restarts the stream or exits if the context
// has been cancelled.
func (c *client) startActions() {
	c.wg.Add(1)

	// results are held outside of the retry loop, because on re-connect
	// we still want to send the responses that either failed or haven't been
	// sent back to the agent.
	actionResults := make(chan *proto.ActionResponse, 100)
	go func() {
		defer c.wg.Done()
		for {
			select {
			case <-c.ctx.Done():
				// stopped
				return
			default:
			}

			actionsCtx, actionsCancel := context.WithCancel(c.ctx)
			actionsClient, err := c.client.Actions(actionsCtx)
			if err != nil {
				c.impl.OnError(err)
				continue
			}

			var actionsWG sync.WaitGroup
			done := make(chan bool)

			// action requests
			actionsWG.Add(1)
			go func() {
				defer actionsWG.Done()
				for {
					action, err := actionsClient.Recv()
					if err != nil {
						if err != io.EOF {
							c.impl.OnError(err)
						}
						close(done)
						return
					}

					actionImpl, ok := c.actions[action.Name]
					if !ok {
						actionResults <- &proto.ActionResponse{
							Token:  c.conn.Token,
							Id:     action.Id,
							Status: proto.ActionResponse_FAILED,
							Result: ActionErrUndefined,
						}
						continue
					}

					var params map[string]interface{}
					err = json.Unmarshal(action.Params, &params)
					if err != nil {
						actionResults <- &proto.ActionResponse{
							Token:  c.conn.Token,
							Id:     action.Id,
							Status: proto.ActionResponse_FAILED,
							Result: ActionErrUnmarshableParams,
						}
						continue
					}

					// perform the action
					go func() {
						res, err := actionImpl.Execute(params)
						if err != nil {
							actionResults <- &proto.ActionResponse{
								Token:  c.conn.Token,
								Id:     action.Id,
								Status: proto.ActionResponse_FAILED,
								Result: ujson.MustMarshal(map[string]string{
									"error": err.Error(),
								}),
							}
							return
						}
						resBytes, err := json.Marshal(res)
						if err != nil {
							// client-side error, should have been marshal-able
							c.impl.OnError(err)
							actionResults <- &proto.ActionResponse{
								Token:  c.conn.Token,
								Id:     action.Id,
								Status: proto.ActionResponse_FAILED,
								Result: ActionErrUnmarshableResult,
							}
							return
						}
						actionResults <- &proto.ActionResponse{
							Token:  c.conn.Token,
							Id:     action.Id,
							Status: proto.ActionResponse_SUCCESS,
							Result: resBytes,
						}
					}()
				}
			}()

			// action responses
			actionsWG.Add(1)
			go func() {
				defer actionsWG.Done()

				// initial connection of stream must send the token so
				// the Elastic Agent knows this clients token.
				err := actionsClient.Send(&proto.ActionResponse{
					Token:  c.conn.Token,
					Id:     ActionResponseInitID,
					Status: proto.ActionResponse_SUCCESS,
					Result: []byte("{}"),
				})
				if err != nil {
					c.impl.OnError(err)
					actionsCancel()
					return
				}

				for {
					select {
					case <-done:
						return
					case res := <-actionResults:
						err := actionsClient.Send(res)
						if err != nil {
							// failed to send, add back to response to try again
							actionResults <- res
							c.impl.OnError(err)
							actionsCancel()
							return
						}
					}
				}
			}()

			// wait for both send and recv go routines to stop before
			// starting a new stream.
			actionsWG.Wait()
		}
	}()
}
