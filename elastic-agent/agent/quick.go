package agent

import (
	"context"
	"github.com/blakerouse/elastic-agent-sdk/pkg/component"
	"github.com/blakerouse/elastic-agent-sdk/pkg/control"
	"github.com/blakerouse/elastic-agent-sdk/pkg/events"
	"github.com/elastic/go-ucfg"
	"github.com/rs/xid"
	"time"
)

// QuickComponent sends a batch of 100 events, thats it.
type QuickComponent struct {
	sender component.Sender
}

// NewQuickComponent creates a new end component.
func NewQuickComponent() *QuickComponent {
	return &QuickComponent{}
}

// Name is the name for the component.
func (e *QuickComponent) Name() string {
	return "quick"
}

// Version is the version for the component.
func (e *QuickComponent) Version() string {
	return "alpha"
}

// Actions returns set of actions for the component.
func (e *QuickComponent) Actions() []control.Action {
	return nil
}

// Run sends the 100 events, then returns.
func (e *QuickComponent) Run(ctx context.Context, config *ucfg.Config) error {
	evts := make(events.Events, 0, 100)
	for i := 0; i < 100; i++ {
		e := events.New()
		e.Put("random", xid.New().String())
		e.Put("@timestamp", time.Now())
		evts = append(evts, e)
	}
	_, err := e.sender.Send(ctx, evts)
	return err
}

// Reload reloads the updated configuration (does nothing)
func (e *QuickComponent) Reload(config *ucfg.Config) error {
	return nil
}

// SetSender sets the sender.
func (e *QuickComponent) SetSender(sender component.Sender) {
	e.sender = sender
}
