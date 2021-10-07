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
	"github.com/blakerouse/elastic-agent-sdk/pkg/events"
	"github.com/elastic/go-ucfg"

	"github.com/blakerouse/elastic-agent-sdk/pkg/control"
)

// Component is the based interface each component must implement.
type Component interface {
	// Name is the name of the component.
	Name() string

	// Version is the version of the component.
	Version() string

	// Actions that the component implements.
	Actions() []control.Action

	// Run runs the component.
	Run(context.Context, *ucfg.Config) error

	// Reload called to when the config has changed.
	Reload(*ucfg.Config) error
}

// ReceiveComponent is a component that only receives events. A receive only component always ends the chain
// of components and is almost always the final output for the agent.
type ReceiveComponent interface {
	Component

	// Receive is called when a set of events is received.
	// Slice of events ID's should be returned for those that where handled
	// correctly. ID's not returned are assumed to not been handled correctly.
	Receive(context.Context, events.Events) ([]uint64, error)
}

// Sender is passed to the run command of the SendComponent to send events.
type Sender interface {
	// Send is called by the SendComponent to send events.
	Send(context.Context, events.Events) ([]uint64, error)
}

// SendComponent is a component that creates an initial event to send through the chain of components.
type SendComponent interface {
	Component

	// SetSender set the sender for the component to use.
	SetSender(sender Sender)
}

// ReceiveSendComponent is placed in the middle of the chain of components. It takes a set of events
// and then forwards them on, normally adding some bit of information to each event.
type ReceiveSendComponent interface {
	ReceiveComponent
	SendComponent
}
