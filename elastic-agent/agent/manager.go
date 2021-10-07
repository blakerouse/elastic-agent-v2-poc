package agent

import (
	"context"
	"errors"
	"github.com/blakerouse/elastic-agent-sdk/pkg/proto"
	"github.com/elastic/elastic-agent/pipeline"
	"github.com/elastic/elastic-agent/server"
)

var esCfg = `
hosts:
 - localhost:9200
username: elastic
password: changeme
`

type Manager struct {
	components []*pipeline.Component
	srv *server.Server
}

func NewManager(componentsPath string) (*Manager, error) {
	comps, err := pipeline.LoadComponents(componentsPath)
	if err != nil {
		return nil, err
	}
	m := &Manager{
		components: comps,
	}
	srv, err := server.New("unix://agent-components.sock", m)
	if err != nil {
		return nil, err
	}
	m.srv = srv
	return m, nil
}

func (m *Manager) Run(ctx context.Context) error {
	fake := m.getComponent("fake")
	if fake == nil {
		return errors.New("failed to find component: fake")
	}
	host := m.getComponent("processor-host")
	if host == nil {
		return errors.New("failed to find component: processor-host")
	}
	spooler := m.getComponent("processor-spooler")
	if spooler == nil {
		return errors.New("failed to find component: processor-spooler")
	}
	es := m.getComponent("elasticsearch")
	if es == nil {
		return errors.New("failed to find component: elasticsearch")
	}
	p, err := pipeline.NewPipeline(&pipeline.Node{
		Component: fake,
		Config: "",
	}, &pipeline.Node{
		Component: host,
		Config: "",
	}, &pipeline.Node{
		Component: spooler,
		Config: "",
	}, &pipeline.Node{
		Component: es,
		Config: esCfg,
	})
	if err != nil {
		return err
	}

	err = m.srv.Run(ctx)
	if err != nil {
		return err
	}
	err = p.Run(ctx, m.srv)
	if err != nil {
		return err
	}

	<-ctx.Done()
	return nil
}

func (m *Manager) getComponent(name string) *pipeline.Component {
	for _, comp := range m.components {
		if comp.Info.Name == name {
			return comp
		}
	}
	return nil
}

func (m *Manager) OnStatusChange(state *server.ApplicationState, status proto.StateObserved_Status, s string, m2 map[string]interface{}) {
	// TODO: Do something!
}
