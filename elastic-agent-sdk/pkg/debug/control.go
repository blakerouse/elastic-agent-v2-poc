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

package debug

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/blakerouse/elastic-agent-sdk/pkg/utils/authority"
	"google.golang.org/grpc/credentials"
	"sync"

	"github.com/rs/xid"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/blakerouse/elastic-agent-sdk/pkg/proto"
	"github.com/blakerouse/elastic-agent-sdk/pkg/utils/listener"
)

type performAction struct {
	Name     string
	Params   []byte
	Callback func(map[string]interface{}, error)
}

type actionResultChan struct {
	Result map[string]interface{}
	Err    error
}

type SimpleController struct {
	proto.UnimplementedControlServer

	ca 			*authority.CertificateAuthority
	pair        *authority.Pair

	addr        string
	token       string
	config      string

	server      *grpc.Server
	actions     chan *performAction
	sentActions map[string]*performAction
}

func New(addr string, config string) (*SimpleController, error) {
	ca, err := authority.NewCA()
	if err != nil {
		return nil, err
	}
	pair, err := ca.GeneratePair()
	if err != nil {
		return nil, err
	}
	return &SimpleController{
		ca: 	ca,
		pair: 	pair,
		addr:   addr,
		token: 	xid.New().String(),
		config: config,
		actions: make(chan *performAction, 100),
	}, nil
}

func (s *SimpleController) Run(ctx context.Context) error {
	lis, err := listener.New(s.addr, 0)
	if err != nil {
		return err
	}
	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(s.ca.Crt()); !ok {
		return errors.New("failed to append root CA")
	}
	creds := credentials.NewTLS(&tls.Config{
		ClientAuth:     tls.RequireAndVerifyClientCert,
		ClientCAs:      certPool,
		Certificates: 	[]tls.Certificate{*s.pair.Certificate},
	})
	srv := grpc.NewServer(grpc.Creds(creds))
	s.server = srv
	proto.RegisterControlServer(s.server, s)
	go func() {
		srv.Serve(lis)
	}()
	go func() {
		<-ctx.Done()
		srv.Stop()
		s.server = nil
	}()
	return nil
}

func (s *SimpleController) Checkin(server proto.Control_CheckinServer) error {
	for {
		checkin, err := server.Recv()
		if err != nil {
			return err
		}
		if s.token != checkin.Token {
			log.Error().
				Str("expectedTxn", s.token).
				Str("token", checkin.Token).
				Msg("checkin had invalid token")
			return errors.New("invalid checkin token")
		}
		log.Debug().
			Str("token", checkin.Token).
			Uint64("cfgIdx", checkin.ConfigStateIdx).
			Int("status", int(checkin.Status)).
			Str("msg", checkin.Message).
			Str("payload", checkin.Payload).
			Msg("controller received checkin status")
		idx := checkin.ConfigStateIdx
		cfg := ""
		if idx == 0 {
			idx = 1
			cfg = s.config
		}
		err = server.Send(&proto.StateExpected{
			State:          proto.StateExpected_RUNNING,
			ConfigStateIdx: idx,
			Config:         cfg,
		})
		if err != nil {
			return err
		}
	}
}

func (s *SimpleController) Actions(server proto.Control_ActionsServer) error {
	var m sync.Mutex
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				return
			case act := <-s.actions:
				id := xid.New().String()
				m.Lock()
				s.sentActions[id] = act
				m.Unlock()
				err := server.Send(&proto.ActionRequest{
					Id:     id,
					Name:   act.Name,
					Params: act.Params,
				})
				if err != nil {
					panic(err)
				}
			}
		}
	}()
	defer close(done)

	for {
		response, err := server.Recv()
		if err != nil {
			return err
		}
		m.Lock()
		action, ok := s.sentActions[response.Id]
		if !ok {
			// nothing to do, unknown action
			if response.Id != "init" {
				log.Error().Str("action", response.Id).Msg("received unknown action response")
			}
			m.Unlock()
			continue
		}
		delete(s.sentActions, response.Id)
		m.Unlock()
		var result map[string]interface{}
		err = json.Unmarshal(response.Result, &result)
		if err != nil {
			return err
		}
		if response.Status == proto.ActionResponse_FAILED {
			error, ok := result["error"]
			if ok {
				err = fmt.Errorf("%s", error)
			} else {
				err = fmt.Errorf("unknown error")
			}
			log.Error().Str("action", response.Id).Err(err).Msg("action errored")
			action.Callback(nil, err)
		} else {
			log.Debug().Str("action", response.Id).Interface("res", result).Msg("action result")
			action.Callback(result, nil)
		}
	}
}

func (s *SimpleController) PerformAction(name string, params map[string]interface{}) (map[string]interface{}, error) {
	paramBytes, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	resChan := make(chan actionResultChan)
	s.actions <- &performAction{
		Name:   name,
		Params: paramBytes,
		Callback: func(m map[string]interface{}, err error) {
			resChan <- actionResultChan{
				Result: m,
				Err:    err,
			}
		},
	}
	res := <-resChan
	return res.Result, res.Err
}

func (s *SimpleController) GetAgentConn() *proto.Agent {
	return &proto.Agent{
		Addr:       s.addr,
		ServerName: "localhost",
		Token:      s.token,
		CaCert:     s.ca.Crt(),
		PeerCert:   s.pair.Crt,
		PeerKey:    s.pair.Key,
	}
}
