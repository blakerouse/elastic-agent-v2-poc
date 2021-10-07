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

package events

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"github.com/blakerouse/elastic-agent-sdk/pkg/utils/listener"
	"github.com/rs/zerolog/log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/blakerouse/elastic-agent-sdk/pkg/proto"
)

// Receive is interface for receiving events into the component.
type Receive interface {
	// Receive is called when a set of events is received.
	// Slice of events ID's should be returned for those that
	// where handled correctly.
	Receive(context.Context, Events) ([]uint64, error)
}

// Receiver is GRPC endpoint for a component to receive events.
type Receiver interface {
	// Run runs the receiver.
	Run(ctx context.Context) error
}

// receiver is GRPC endpoint for a component to receive events.
type receiver struct {
	conn *proto.Receive
	impl Receive

	listener     net.Listener
	server       *grpc.Server
}

type handleReceiver struct {
	proto.UnimplementedEventsServer

	impl Receive
}

// NewReceiver creates a new GRPC server for clients to connect to.
func NewReceiver(conn *proto.Receive, impl Receive) Receiver {
	return &receiver{
		conn: conn,
		impl: impl,
	}
}

// Run runs the GRPC endpoint and accepts new connections.
func (r *receiver) Run(ctx context.Context) error {
	if r.server != nil {
		// already running
		return nil
	}

	lis, err := listener.New(r.conn.Addr, 0)
	if err != nil {
		return err
	}
	r.listener = lis
	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(r.conn.CaCert); !ok {
		return errors.New("failed to append root CA")
	}
	cert, err := tls.X509KeyPair(r.conn.PeerCert, r.conn.PeerKey)
	if err != nil {
		return err
	}
	creds := credentials.NewTLS(&tls.Config{
		ClientAuth:     tls.RequireAndVerifyClientCert,
		ClientCAs:      certPool,
		Certificates:   []tls.Certificate{cert},
	})
	r.server = grpc.NewServer(grpc.Creds(creds))
	proto.RegisterEventsServer(r.server, &handleReceiver{
		impl: r.impl,
	})

	// start serving GRPC connections
	go func() {
		for {
			err := r.server.Serve(lis)
			if err == nil || err == context.Canceled {
				listener.Cleanup(r.conn.Addr)
				return
			}
			<-time.After(100 * time.Millisecond)
		}
	}()

	// handle cancel of the context
	go func() {
		<-ctx.Done()
		r.server.Stop()
		r.server = nil
		r.listener = nil
	}()

	return nil
}

// Send is when the receiver gets the actual events.
func (s *handleReceiver) Send(ctx context.Context, events *proto.SendEvents) (*proto.SendEventsResult, error) {
	before := time.Now()
	ids := make([]uint64, 0, len(events.Events))
	evts := make(Events, 0, len(events.Events))
	for _, evt := range events.Events {
		ids = append(ids, evt.Id)
		evts = append(evts, FromEvent(evt))
	}
	timeSpent := time.Now().Sub(before)

	log.Trace().
		Dur("dur", timeSpent).
		Int("evts", len(evts)).
		Msg("received events from previous node")

	implSent, err := s.impl.Receive(ctx, evts)
	if err != nil {
		return nil, err
	}
	sent := make([]uint64, 0, len(events.Events))
	for _, id := range ids {
		if contains(implSent, id) {
			sent = append(sent, id)
		}
	}
	return &proto.SendEventsResult{
		Sent:   sent,
	}, nil
}

func contains(array []uint64, value uint64) bool {
	for _, v := range array {
		if v == value {
			return true
		}
	}
	return false
}
