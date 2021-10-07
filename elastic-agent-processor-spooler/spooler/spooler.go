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

package spooler

import (
	"context"
	"time"

	"github.com/blakerouse/elastic-agent-sdk/pkg/component"
	"github.com/blakerouse/elastic-agent-sdk/pkg/control"
	"github.com/blakerouse/elastic-agent-sdk/pkg/events"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/publisher"
	"github.com/elastic/beats/v7/libbeat/publisher/queue"
	"github.com/elastic/go-ucfg"
	"github.com/rs/zerolog/log"
)

type processor struct {
	factory queue.Factory
	queue queue.Queue
	producer queue.Producer

	sender component.Sender
}

// New creates a the spooler component.
func New() component.ReceiveSendComponent {
	spoolFactory := queue.FindFactory("spool")
	if spoolFactory == nil {
		panic("unable to find spool factory")
	}
	return &processor{factory: spoolFactory}
}

// Name returns the name of the component.
func (p *processor) Name() string {
	return "processor-spooler"
}

// Version returns the version of the component.
func (p *processor) Version() string {
	return "alpha"
}

// Actions returns zero actions as a pipeline component does not have actions.
func (p *processor) Actions() []control.Action {
	return nil
}

// Receive is called with a batch of events. The events are passed through the PipelineFunc and
// forwarded on to the sender.
func (p *processor) Receive(ctx context.Context, evts events.Events) ([]uint64, error) {
	if p.sender == nil {
		panic("Run must be called before Receive")
	}

	before := time.Now()
	processed := make([]uint64, 0, len(evts))
	for _, e := range evts {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		pe, err := toPublisherEvent(e)
		if err != nil {
			log.Error().Err(err).Msg("failed to convert to publisher event")
			continue
		}
		if p.producer.Publish(pe) {
			processed = append(processed, e.ID())
		}
	}
	timeSpent := time.Now().Sub(before)
	log.Trace().Dur("dur", timeSpent).Int("evts", len(processed)).Msg("added to spool")
	return processed, nil
}

// Run runs for the sender.
func (p *processor) Run(ctx context.Context, cfg *ucfg.Config) (err error) {
	c, err := common.NewConfigFrom(cfg)
	if err != nil {
		return err
	}
	q, err := p.factory(&noopAckListener{}, logp.NewLogger("spool"), c, 0)
	if err != nil {
		return err
	}
	p.queue = q
	p.producer = q.Producer(queue.ProducerConfig{
		ACK:          nil,
		OnDrop:       nil,
		DropOnCancel: false,
	})

	go func(consumer queue.Consumer) {
		defer consumer.Close()
		for {
			batch, err := consumer.Get(100)
			if err == context.Canceled {
				return
			}
			if err != nil {
				log.Error().Err(err).Msg("failed to get batch of events from spooler")
				continue
			}

			evts := batch.Events()
			send := make(events.Events, 0, len(evts))
			for _, e := range evts {
				send = append(send, toEvent(e.Content))
			}
			ids, err := p.sender.Send(ctx, send)
			if err == context.Canceled {
				return
			}
			if err != nil {
				log.Error().Err(err).Msg("failed to send events to next component")
				continue
			}
			if len(ids) == 0 {
				log.Error().Msg("next component failed to send events")
				continue
			}
			batch.ACK()
		}

	}(q.Consumer())

	go func() {
		<-ctx.Done()
		if err := p.queue.Close(); err != nil && err != context.Canceled {
			log.Error().Err(err).Msg("closing spooler failed")
		}
	}()

	return nil
}

// Reload is called when the configuration changes.
func (p *processor) Reload(cfg *ucfg.Config) (err error) {
	// TODO: Handle reload.
	return nil
}

// SetSender sets the sender.
func (p *processor) SetSender(sender component.Sender) {
	p.sender = sender
}

type noopAckListener struct {
}

func (n noopAckListener) OnACK(_ int) {
}

func toPublisherEvent(evt *events.Event) (publisher.Event, error) {
	e, err := toBeatEvent(evt)
	if err != nil {
		return publisher.Event{}, err
	}
	return publisher.Event{
		Content: e,
		Flags: publisher.GuaranteedSend,
	}, nil
}

func toBeatEvent(evt *events.Event) (beat.Event, error) {
	var e beat.Event
	contents, err := evt.Contents()
	if err != nil {
		return e, err
	}
	timestamp, ok := contents["@timestamp"]
	if ok {
		time, ok := timestamp.(time.Time)
		if ok {
			delete(contents, "@timestamp")
			e.Timestamp = time
		}
	}
	e.Fields = contents
	return e, nil
}

func toEvent(e beat.Event) *events.Event {
	evt := events.New()
	evt.Put("@timestamp", e.Timestamp)
	for key, val := range e.Fields {
		evt.Put(key, val)
	}
	return evt
}
