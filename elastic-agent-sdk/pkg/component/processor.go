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
	"fmt"
	"github.com/rs/zerolog/log"

	"github.com/blakerouse/elastic-agent-sdk/pkg/control"
	"github.com/blakerouse/elastic-agent-sdk/pkg/events"
	"github.com/elastic/go-ucfg"
)

// ConfigFunc is a function that is called to process the configuration
type ConfigFunc func(*ucfg.Config) (interface{}, error)

// ProcessorFunc is a function that processes a single event
type ProcessorFunc func(context.Context, *events.Event, interface{}) error

type processor struct {
	name string
	version string
	cf ConfigFunc
	pf ProcessorFunc
	cfgData interface{}
	sender Sender
}

// NewProcessor creates a new processor component that processes events.
func NewProcessor(name, version string, cf ConfigFunc, pf ProcessorFunc) ReceiveSendComponent {
	return &processor{
		name: fmt.Sprintf("processor-%s", name),
		version: version,
		cf: cf,
		pf: pf,
	}
}

// Name returns the name of the component.
func (p *processor) Name() string {
	return p.name
}

// Version returns the version of the component.
func (p *processor) Version() string {
	return p.version
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

	processed := make(events.Events, 0, len(evts))
	for _, e := range evts {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		err := p.pf(ctx, e, p.cfgData)
		if err != nil {
			if err == context.Canceled {
				return nil, err
			}
			log.Error().Err(err).Uint64("id", e.ID()).Msg("failed to process event")
			continue
		}
		processed = append(processed, e)
	}
	return p.sender.Send(ctx, processed)
}

// Run runs for the sender.
func (p *processor) Run(_ context.Context, cfg *ucfg.Config) (err error) {
	return p.Reload(cfg)
}

// Reload is called when the configuration changes.
func (p *processor) Reload(cfg *ucfg.Config) (err error) {
	p.cfgData, err = p.cf(cfg)
	if err != nil {
		return err
	}
	return nil
}

// SetSender sets the sender.
func (p *processor) SetSender(sender Sender) {
	p.sender = sender
}
