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
	"encoding/json"
	"errors"
	"sync/atomic"

	"github.com/blakerouse/elastic-agent-sdk/pkg/proto"
)

var gID uint64 = 0

// Event is an event sent through a pipeline of components.
type Event struct {
	id uint64
	decoded map[string]interface{}
	parsed map[string]json.RawMessage
	data []byte
}

func FromEvent(evt *proto.Event) *Event {
	return &Event{
		id: evt.Id,
		data: evt.Data,
	}
}

// New returns a new empty event.
func New() *Event {
	next := atomic.AddUint64(&gID, 1)
	return &Event{
		id: next,
	}
}

// ID is the id of the event through the pipeline.
func (e *Event) ID() uint64 {
	return e.id
}

// Get gets the value from the event.
//
// Loading values from the event is delayed to only load what needed by the component at the top-level. The less
// keys you access the more performant the component will be.
func (e *Event) Get(key string) (interface{}, bool, error) {
	// try to fetch from decoded first
	if e.decoded != nil {
		val, ok := e.decoded[key]
		if ok {
			return val, ok, nil
		}
	}

	// not in decoded; must be parsed
	if e.parsed == nil {
		if err := e.parse(); err != nil {
			return nil, false, err
		}
	}
	var val interface{}
	raw, ok := e.parsed[key]
	if !ok {
		return nil, false, nil
	}
	if err := json.Unmarshal(raw, &val); err != nil {
		return nil, true, err
	}

	// store in decoded for quick get
	if e.decoded == nil {
		e.decoded = make(map[string]interface{})
	}
	e.decoded[key] = val
	return val, true, nil
}

// GetString gets the string value from the event.
func (e *Event) GetString(key string) (string, bool, error) {
	val, ok, err := e.Get(key)
	if err != nil {
		return "", ok, err
	}
	if !ok {
		return "", ok, err
	}
	str, ok := val.(string)
	if !ok {
		return "", ok, errors.New("not string")
	}
	return str, ok, err
}

// Contents return the all the contents of the event decoded.
func (e *Event) Contents() (map[string]interface{}, error) {
	var contents map[string]interface{}
	if e.data != nil {
		if err := json.Unmarshal(e.data, &contents); err != nil {
			return nil, err
		}
	} else {
		contents = make(map[string]interface{})
	}
	if e.decoded != nil {
		for key, val := range e.decoded {
			if val == nil {
				delete(contents, key)
				continue
			}
			contents[key] = val
		}
	}
	return contents, nil
}

// Put adds a value to the event at the top-level key.
func (e *Event) Put(key string, val interface{}) {
	if e.decoded == nil {
		e.decoded = make(map[string]interface{})
	}
	e.decoded[key] = val
}

// PutEncoded adds a value to the event at the top-level key with the data already JSON
// encoded. This prevents the requirement of re-encoding the entire value as its already
// encoded.
func (e *Event) PutEncoded(key string, val json.RawMessage) error {
	if e.decoded == nil {
		e.decoded = make(map[string]interface{})
	}
	if e.parsed == nil {
		if err := e.parse(); err != nil {
			return err
		}
	}
	e.parsed[key] = val
	return nil
}

// Delete deletes a value from the event.
func (e *Event) Delete(key string) {
	if e.decoded == nil {
		e.decoded = make(map[string]interface{})
	}
	e.decoded[key] = nil
}

// MarshalJSON marshals the event into JSON.
func (e *Event) MarshalJSON() ([]byte, error) {
	data := e.data
	if e.decoded != nil {
		if e.parsed == nil {
			if err := e.parse(); err != nil {
				return nil, err
			}
		}
		parsed := e.parsed
		for key, val := range e.decoded {
			if val == nil {
				delete(parsed, key)
				continue
			}
			raw, err := json.Marshal(val)
			if err != nil {
				return nil, err
			}
			parsed[key] = raw
		}
		raw, err := json.Marshal(parsed)
		if err != nil {
			return nil, err
		}
		data = raw
	}
	return data, nil
}

func (e *Event) parse() error {
	if e.data == nil {
		e.parsed = make(map[string]json.RawMessage)
		return nil
	}
	err := json.Unmarshal(e.data, &e.parsed)
	if err != nil {
		return err
	}
	return nil
}

func (e *Event) toProto() (*proto.Event, error) {
	data, err := e.MarshalJSON()
	if err != nil {
		return nil, err
	}
	return &proto.Event{
		Id: e.id,
		Data: data,
	}, nil
}

// Events is a slice of event.
type Events []*Event

func (e Events) toProto() ([]*proto.Event, error) {
	evts := make([]*proto.Event, 0, len(e))
	for _, evt := range e {
		ev, err := evt.toProto()
		if err != nil {
			return nil, err
		}
		evts = append(evts, ev)
	}
	return evts, nil
}