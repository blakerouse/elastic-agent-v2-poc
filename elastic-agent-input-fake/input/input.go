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

package input

import (
	"context"
	"github.com/rs/zerolog/log"
	"sync"
	"time"

	"github.com/blakerouse/elastic-agent-sdk/pkg/component"
	"github.com/blakerouse/elastic-agent-sdk/pkg/control"
	"github.com/blakerouse/elastic-agent-sdk/pkg/events"
	"github.com/blakerouse/elastic-agent-sdk/pkg/utils/sleep"
	"github.com/elastic/go-ucfg"
	"github.com/rs/xid"
)

const EventEvery = 10 * time.Millisecond
const BatchSize = 100
const BatchTime = 30 * time.Second

type Input struct {
	sender component.Sender
	queuePool sync.Pool
}

func New() *Input {
	return &Input{
		queuePool: sync.Pool{New: func() interface{} {
			return make(events.Events, 0, BatchSize)
		}},
	}
}

func (i *Input) Name() string {
	return "fake"
}

func (i *Input) Version() string {
	return "alpha"
}

func (i *Input) Actions() []control.Action {
	return nil
}

func (i *Input) Run(ctx context.Context, cfg *ucfg.Config) error {
	send := make(chan events.Events)
	go func() {
		batch := i.queuePool.Get().(events.Events)
		lastSent := time.Now()

		for {
			select {
			case <-ctx.Done():
				if len(batch) >= 0 {
					send <- batch
				}
				close(send)
				return
			default:
			}

			now := time.Now()
			id := xid.New().String()
			evt := events.New()
			evt.Put("_id", id)
			evt.Put("_index", "random")
			evt.Put("random", id)
			evt.Put("@timestamp", now)
			batch = append(batch, evt)

			if len(batch) >= BatchSize || now.Sub(lastSent) > BatchTime {
				lastSent = now
				send <- batch
				batch = i.queuePool.Get().(events.Events)
			}

			sleep.WithContext(ctx, EventEvery)
		}
	}()

	go func() {
		for {
			// always sends a batch even on context cancelled
			var batch events.Events
			var ok bool
			select {
			case batch, ok = <-send:
			}

			if !ok {
				// channel closed
				return
			}

			// ignores failed event ids
			_, err := i.sender.Send(ctx, batch)
			batch = batch[:0]
			i.queuePool.Put(batch)
			if err != nil {
				log.Error().Err(err).Msg("failed to send batch")
			}
		}
	}()

	return nil
}

func (i *Input) Reload(cfg *ucfg.Config) error {
	// has not config
	return nil
}

func (i *Input) SetSender(sender component.Sender) {
	i.sender = sender
}
