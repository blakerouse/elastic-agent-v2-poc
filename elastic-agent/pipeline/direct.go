package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/blakerouse/elastic-agent-sdk/pkg/component"
	"github.com/blakerouse/elastic-agent-sdk/pkg/control"
	"github.com/blakerouse/elastic-agent-sdk/pkg/events"
	"github.com/blakerouse/elastic-agent-sdk/pkg/proto"
	"github.com/blakerouse/elastic-agent-sdk/pkg/utils/authority"
	"github.com/blakerouse/elastic-agent-sdk/pkg/utils/sleep"
	"github.com/elastic/elastic-agent/server"
	"github.com/elastic/go-ucfg"
	"github.com/elastic/go-ucfg/yaml"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/rs/xid"
	"github.com/rs/zerolog/log"
)

// DirectNode is a single node in the pipeline where the component is connected directly to the other components
// removing the GRPC overhead.
type DirectNode struct {
	SendWrap SendWrap
	Component component.Component
	Config string

	id string
	socket Socket
	state *server.ApplicationState
	running bool

	prev []*DirectNode
	next *DirectNode
}

func (n *DirectNode) ID() string {
	return n.id
}

func (n *DirectNode) start(ctx context.Context, srv *server.Server) error {
	if n.next != nil {
		if err := n.next.start(ctx, srv); err != nil {
			return err
		}
	}
	if n.running {
		// already started
		return nil
	}
	state, err := srv.Register(n, n.Config)
	if err != nil {
		return err
	}
	n.state = state
	err = n.exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (n *DirectNode) exec(ctx context.Context) error {
	reader, writer := io.Pipe()
	go func() {
		n.writeConnInfo(writer)
	}()

	var nextComponent component.ReceiveComponent
	if n.next != nil {
		nextComponent = n.next.Component.(component.ReceiveComponent)
	}
	runner, err := NewDirectRunner(reader, n.Component, nextComponent, n.SendWrap)
	if err != nil {
		return err
	}
	go func() {
		for {
			err := runner.Run(ctx)
			if err == nil || err == context.Canceled {
				return
			}

			log.Error().Err(err).Str("node", n.id).Str("component", n.Component.Name()).Msg("runner failed")
			sleep.WithContext(ctx, 500 * time.Millisecond)
		}
	}()
	return nil
}

func (n *DirectNode) writeConnInfo(w io.Writer) error {
	node := &proto.Node{
		ID: n.id,
		Agent: n.state.GetAgentConn(),
	}
	if n.socket.ID != "" {
		node.Receive = &proto.Receive{
			Addr:     n.socket.GetAddr(),
			CaCert:   n.socket.CA.Crt(),
			PeerCert: n.socket.Pair.Crt,
			PeerKey:  n.socket.Pair.Key,
		}
	}
	if n.next != nil {
		node.Send = &proto.Send{
			Addr:       n.next.socket.GetAddr(),
			ServerName: n.next.socket.ID,
			CaCert:     n.next.socket.CA.Crt(),
			PeerCert:   n.next.socket.Pair.Crt,
			PeerKey:    n.next.socket.Pair.Key,
		}
	}
	infoBytes, err := protobuf.Marshal(node)
	if err != nil {
		return fmt.Errorf("failed to marshal connection information: %w", err)
	}
	_, err = w.Write(infoBytes)
	if err != nil {
		return fmt.Errorf("failed to write connection information: %w", err)
	}
	closer, ok := w.(io.Closer)
	if ok {
		_ = closer.Close()
	}
	return nil
}

// DirectPipeline is a complete pipeline of nodes (aka. component instances)
// that are composed together passing events to each directly (not with GRPC), but they are running
// all with in the same process.
type DirectPipeline struct {
	root *DirectNode
}

// NewDirectPipeline creates a new pipeline from the defined components.
func NewDirectPipeline(nodes ...*DirectNode) (*DirectPipeline, error) {
	if len(nodes) < 2 {
		return nil, ErrInvalidPipeline
	}
	firstNode := nodes[0]
	_, receiver := firstNode.Component.(component.ReceiveComponent)
	if receiver {
		// first component cannot be a receiver
		return nil, ErrInvalidPipeline
	}
	lastNode := nodes[len(nodes) - 1]
	_, sender := lastNode.Component.(component.SendComponent)
	if sender {
		// last component cannot be a sender
		return nil, ErrInvalidPipeline
	}
	for i := 1; i < len(nodes) - 1; i++ {
		node := nodes[i]
		_, ok := node.Component.(component.ReceiveSendComponent)
		if !ok {
			// middle components must be both
			return nil, ErrInvalidPipeline
		}
	}

	ca, err := authority.NewCA()
	if err != nil {
		return nil, err
	}
	prevNode := firstNode
	prevNode.id = xid.New().String()
	pipeline := &DirectPipeline{
		root: prevNode,
	}
	for i := 1; i < len(nodes); i++ {
		node := nodes[i]
		node.id = xid.New().String()
		pair, err := ca.GeneratePairWithName(node.id)
		if err != nil {
			return nil, err
		}
		node.socket.ID = node.id
		node.socket.CA = ca
		node.socket.Pair = pair
		node.prev = []*DirectNode{prevNode}
		prevNode.next = node
		prevNode = node
	}
	return pipeline, nil
}

func (p *DirectPipeline) Run(ctx context.Context, srv *server.Server) error {
	return p.root.start(ctx, srv)
}

type runner struct {
	ctx context.Context
	canceller context.CancelFunc
	cfg *ucfg.Config
	err error

	nodeId string
	comp component.Component
	next component.ReceiveComponent

	sendWrap SendWrap
	sendComp component.SendComponent

	agent control.Client
}

// NewDirectRunner creates a new runner for the component that runs in direct mode.
func NewDirectRunner(reader io.Reader, comp component.Component, next component.ReceiveComponent, sender SendWrap) (component.Runner, error) {
	r := &runner{
		comp: comp,
		next: next,
		sendWrap: sender,
	}
	sendComp, ok := comp.(component.SendComponent)
	if ok {
		r.sendComp = sendComp
		if next == nil {
			return nil, errors.New("next required when component is a sending component")
		}
	}
	err := r.load(reader)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (r *runner) Run(ctx context.Context) error {
	r.ctx, r.canceller = context.WithCancel(ctx)
	if err := r.agent.Run(r.ctx); err != nil && err != context.Canceled {
		return err
	}
	<-r.ctx.Done()
	if r.err != nil && r.err != context.Canceled {
		return r.err
	}
	return nil
}

func (r *runner) OnConfig(s string) {
	cfg, err := yaml.NewConfig([]byte(s), ucfg.PathSep("."))
	if err != nil {
		// bad configuration should stop
		r.err = fmt.Errorf("failed to parse config: %w", err)
		r.canceller()
		return
	}
	if r.cfg == nil {
		// initial configuration, actually start the component
		if err := r.comp.Run(r.ctx, cfg); err != nil {
			r.err = err
			r.canceller()
			return
		}
	} else {
		if err := r.comp.Reload(cfg); err != nil {
			r.err = err
			r.canceller()
			return
		}
	}
	r.cfg = cfg
}

func (r *runner) OnStop() {
	r.canceller()
}

func (r *runner) OnError(err error) {
	log.Error().Err(err).Msg("error communicating with Elastic Agent")
}

func (r *runner) load(reader io.Reader) error {
	node := &proto.Node{}
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}
	err = protobuf.Unmarshal(data, node)
	if err != nil {
		return err
	}

	r.nodeId = node.ID

	// global logger is updated for the whole process, as its the only thing that should
	// be running inside this process
	log.Logger = log.Logger.With().Str("node", r.nodeId).Logger()

	if r.sendComp != nil {
		r.sendComp.SetSender(&sendWrapper{r.next, r.sendWrap})
	}

	r.agent = control.New(node.Agent, r, r.comp.Actions())
	return nil
}

type sendWrapper struct {
	comp component.ReceiveComponent
	wrapper SendWrap
}

func (s sendWrapper) Send(ctx context.Context, evts events.Events) ([]uint64, error) {
	return s.wrapper.Send(ctx, evts, s.comp)
}

type SendWrap interface {
	Send(context.Context, events.Events, component.ReceiveComponent) ([]uint64, error)
}

type DirectSend struct {
}

func (d DirectSend) Send(ctx context.Context, evts events.Events, comp component.ReceiveComponent) ([]uint64, error) {
	return comp.Receive(ctx, evts)
}

type EncodeSend struct {
}

func (d EncodeSend) Send(ctx context.Context, evts events.Events, comp component.ReceiveComponent) ([]uint64, error) {
	encoded := make(events.Events, 0, len(evts))
	for _, e := range evts {
		data, err := json.Marshal(e)
		if err != nil {
			return nil, err
		}
		encoded = append(encoded, events.FromEvent(&proto.Event{Id: e.ID(), Data: data}))
	}
	return comp.Receive(ctx, evts)
}

