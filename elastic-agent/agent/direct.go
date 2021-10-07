package agent

import (
	"context"
	"github.com/blakerouse/elastic-agent-sdk/pkg/events"

	"github.com/blakerouse/elastic-agent-processor-host/host"
	"github.com/blakerouse/elastic-agent-sdk/pkg/proto"
	_ "github.com/elastic/beats/v7/libbeat/publisher/queue/spool"

	"github.com/elastic/elastic-agent/pipeline"
	"github.com/elastic/elastic-agent/server"
)

type DirectManager struct {
	srv *server.Server
}

func NewDirectManager() (*DirectManager, error) {
	m := &DirectManager{}
	srv, err := server.New("unix://agent-components.sock", m)
	if err != nil {
		return nil, err
	}
	m.srv = srv
	return m, nil
}

func (m *DirectManager) Run(ctx context.Context, sendWrap pipeline.SendWrap) (events.Events, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	input := NewQuickComponent()
	host := host.New()
	end := NewEndComponent()
	p, err := pipeline.NewDirectPipeline(&pipeline.DirectNode{
		SendWrap: sendWrap,
		Component: input,
		Config: "",
	}, &pipeline.DirectNode{
		SendWrap: sendWrap,
		Component: host,
		Config: "",
	}, &pipeline.DirectNode{
		SendWrap: sendWrap,
		Component: end,
		Config: "",
	})
	if err != nil {
		return nil, err
	}

	err = m.srv.Run(ctx)
	if err != nil {
		return nil, err
	}
	err = p.Run(ctx, m.srv)
	if err != nil {
		return nil, err
	}

	evts := end.Done()
	return evts, nil
}

func (m *DirectManager) OnStatusChange(state *server.ApplicationState, status proto.StateObserved_Status, s string, m2 map[string]interface{}) {
	// TODO: Do something!
}
