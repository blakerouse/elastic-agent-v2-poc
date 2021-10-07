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
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"time"

	"github.com/blakerouse/elastic-agent-sdk/pkg/proto"
)

// Sender sends events to the next component in the chain.
type Sender interface {
	// Run runs the sender.
	Run(ctx context.Context) error
	// Send sends the events.
	Send(ctx context.Context, events Events) ([]uint64, error)
}

// sender sends a batch of events to a receiver
type sender struct {
	conn 	*proto.Send
	client  proto.EventsClient
}

// NewSender creates a connection to the next component to start sending.
func NewSender(conn *proto.Send) Sender {
	return &sender{
		conn: conn,
	}
}

// Run runs the sender.
func (s *sender) Run(ctx context.Context) error {
	cert, err := tls.X509KeyPair(s.conn.PeerCert, s.conn.PeerKey)
	if err != nil {
		return err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(s.conn.CaCert)
	trans := credentials.NewTLS(&tls.Config{
		ServerName:   s.conn.ServerName,
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	})
	conn, err := grpc.DialContext(ctx, s.conn.Addr, grpc.WithTransportCredentials(trans))
	if err != nil {
		return err
	}
	s.client = proto.NewEventsClient(conn)
	return nil
}

// Send sends a batch of events.
func (s *sender) Send(ctx context.Context, events Events) ([]uint64, error) {
	before := time.Now()
	evts, err := events.toProto()
	if err != nil {
		return nil, err
	}
	timeSpent := time.Now().Sub(before)

	log.Trace().
		Dur("dur", timeSpent).
		Int("evts", len(evts)).
		Msg("sending events to next node")

	res, err := s.client.Send(ctx, &proto.SendEvents{Events: evts})
	if err != nil {
		return nil, err
	}
	return res.Sent, nil
}
