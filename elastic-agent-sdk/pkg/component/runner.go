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

package component

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/rs/zerolog/log"

	"github.com/blakerouse/elastic-agent-sdk/pkg/control"
	"github.com/blakerouse/elastic-agent-sdk/pkg/events"
	"github.com/blakerouse/elastic-agent-sdk/pkg/proto"
)

// Runner controls and runs the component.
type Runner interface {
	Run(ctx context.Context) error
}

type runner struct {
	ctx context.Context
	canceller context.CancelFunc
	cfg *ucfg.Config
	err error

	nodeId string
	comp Component

	recvComp ReceiveComponent
	receiver events.Receiver

	sendComp SendComponent
	sender events.Sender

	agent control.Client
}

// NewRunner creates a new runner for the component.
func NewRunner(reader io.Reader, comp Component) (Runner, error) {
	r := &runner{
		comp: comp,
	}
	recvComp, ok := comp.(ReceiveComponent)
	if ok {
		r.recvComp = recvComp
	}
	sendComp, ok := comp.(SendComponent)
	if ok {
		r.sendComp = sendComp
	}
	err := r.load(reader)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (r *runner) Run(ctx context.Context) error {
	r.ctx, r.canceller = context.WithCancel(ctx)
	if err := r.agent.Run(r.ctx); err != nil && err != context.Canceled {
		return err
	}
	<-r.ctx.Done()
	if r.err != nil && r.err != context.Canceled {
		return r.err
	}
	return nil
}

func (r *runner) OnConfig(s string) {
	cfg, err := yaml.NewConfig([]byte(s), ucfg.PathSep("."))
	if err != nil {
		// bad configuration should stop
		r.err = fmt.Errorf("failed to parse config: %w", err)
		r.canceller()
		return
	}
	if r.cfg == nil {
		if r.receiver != nil {
			if err := r.receiver.Run(r.ctx); err != nil {
				r.err = err
				r.canceller()
				return
			}
		}
		if r.sender != nil {
			if err := r.sender.Run(r.ctx); err != nil {
				r.err = err
				r.canceller()
				return
			}
		}
		if err := r.comp.Run(r.ctx, cfg); err != nil {
			r.err = err
			r.canceller()
			return
		}
	} else {
		if err := r.comp.Reload(cfg); err != nil {
			r.err = err
			r.canceller()
			return
		}
	}
	r.cfg = cfg
}

func (r *runner) OnStop() {
	r.canceller()
}

func (r *runner) OnError(err error) {
	log.Error().Err(err).Msg("error communicating with Elastic Agent")
}

func (r *runner) load(reader io.Reader) error {
	node := &proto.Node{}
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}
	err = protobuf.Unmarshal(data, node)
	if err != nil {
		return err
	}

	r.nodeId = node.ID

	// global logger is updated for the whole process, as its the only thing that should
	// be running inside this process
	log.Logger = log.Logger.With().Str("node", r.nodeId).Logger()

	if r.sendComp != nil {
		if node.Send == nil {
			return errors.New("missing send connection information for component that sends")
		}

		r.sender = events.NewSender(node.Send)
		r.sendComp.SetSender(r.sender)
	}

	if r.recvComp != nil {
		if node.Receive == nil {
			return errors.New("missing receive connection information for component that receives")
		}

		r.receiver = events.NewReceiver(node.Receive, r.recvComp)
	}

	r.agent = control.New(node.Agent, r, r.comp.Actions())
	return nil
}
