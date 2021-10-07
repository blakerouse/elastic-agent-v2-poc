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

type InsideManager struct {
	srv *server.Server
}

func NewInsideManager() (*InsideManager, error) {
	m := &InsideManager{}
	srv, err := server.New("unix://agent-components.sock", m)
	if err != nil {
		return nil, err
	}
	m.srv = srv
	return m, nil
}

func (m *InsideManager) Run(ctx context.Context) (events.Events, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	input := NewQuickComponent()
	host := host.New()
	end := NewEndComponent()
	p, err := pipeline.NewInsidePipeline(&pipeline.InsideNode{
		Component: input,
		Config: "",
	}, &pipeline.InsideNode{
		Component: host,
		Config: "",
	}, &pipeline.InsideNode{
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

func (m *InsideManager) OnStatusChange(state *server.ApplicationState, status proto.StateObserved_Status, s string, m2 map[string]interface{}) {
	// TODO: Do something!
}
