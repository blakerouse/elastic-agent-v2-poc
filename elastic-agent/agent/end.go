package agent

import (
	"context"
	"github.com/blakerouse/elastic-agent-sdk/pkg/control"
	"github.com/blakerouse/elastic-agent-sdk/pkg/events"
	"github.com/elastic/go-ucfg"
)

// EndComponent is added as the last component in the pipeline to signal that the events have been received.
type EndComponent struct {
	events events.Events

	done chan bool
}

// NewEndComponent creates a new end component.
func NewEndComponent() *EndComponent {
	return &EndComponent{
		done: make(chan bool),
	}
}

// Name is the name for the component.
func (e *EndComponent) Name() string {
	return "name"
}

// Version is the version for the component.
func (e *EndComponent) Version() string {
	return "alpha"
}

// Actions returns set of actions for the component.
func (e *EndComponent) Actions() []control.Action {
	return nil
}

// Run starts the component (does nothing)
func (e *EndComponent) Run(ctx context.Context, config *ucfg.Config) error {
	return nil
}

// Reload reloads the updated configuration (does nothing)
func (e *EndComponent) Reload(config *ucfg.Config) error {
	return nil
}

// Receive receives the batch of events.
func (e *EndComponent) Receive(ctx context.Context, evts events.Events) ([]uint64, error) {
	ids := make([]uint64, 0, len(evts))
	for _, evt := range evts {
		ids = append(ids, evt.ID())
	}
	if e.done != nil {
		e.done <- true
		e.done = nil
		e.events = evts
	}
	return ids, nil
}

// Done returns once the component has got a batch of events.
func (e *EndComponent) Done() events.Events {
	d := e.done
	_ = <-d
	return e.events
}
