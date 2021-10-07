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
	"encoding/json"

	"github.com/rs/zerolog/log"

	"github.com/blakerouse/elastic-agent-sdk/pkg/events"
)

type logReceiver struct {
}

// NewLogReceiver creates a receive handler that logs all received events.
func NewLogReceiver() events.Receive {
	return &logReceiver{}
}

// Receive receives the events.
func (l *logReceiver) Receive(ctx context.Context, evts events.Events) ([]uint64, error) {
	handled := make([]uint64, 0, len(evts))
	for _, evt := range evts {
		body, err := json.Marshal(evt)
		if err != nil {
			log.Error().Err(err).
				Uint64("evtId", evt.ID()).
				Msg("failed to marshal event")
		} else {
			log.Debug().
				Uint64("evtId", evt.ID()).
				RawJSON("body", body).
				Msg("received event")
		}
		handled = append(handled, evt.ID())
	}
	return handled, nil
}
