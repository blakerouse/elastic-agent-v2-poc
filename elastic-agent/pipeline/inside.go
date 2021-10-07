package pipeline

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/blakerouse/elastic-agent-sdk/pkg/component"
	"github.com/blakerouse/elastic-agent-sdk/pkg/proto"
	"github.com/blakerouse/elastic-agent-sdk/pkg/utils/authority"
	"github.com/blakerouse/elastic-agent-sdk/pkg/utils/sleep"
	"github.com/elastic/elastic-agent/server"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/rs/xid"
	"github.com/rs/zerolog/log"
)

// InsideNode is a single node in the pipeline where the component runs all in the same process.
type InsideNode struct {
	Component component.Component
	Config string

	id string
	socket Socket
	state *server.ApplicationState
	running bool

	prev []*InsideNode
	next *InsideNode
}

func (n *InsideNode) ID() string {
	return n.id
}

func (n *InsideNode) start(ctx context.Context, srv *server.Server) error {
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

func (n *InsideNode) exec(ctx context.Context) error {
	reader, writer := io.Pipe()
	go func() {
		n.writeConnInfo(writer)
	}()

	runner, err := component.NewRunner(reader, n.Component)
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

func (n *InsideNode) writeConnInfo(w io.Writer) error {
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

// InsidePipeline is a complete pipeline of nodes (aka. component instances)
// that are composed together passing events to each other through GRPC, but they are running
// all with in the same process.
type InsidePipeline struct {
	root *InsideNode
}

// NewInsidePipeline creates a new pipeline from the defined components.
func NewInsidePipeline(nodes ...*InsideNode) (*InsidePipeline, error) {
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
	pipeline := &InsidePipeline{
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
		node.prev = []*InsideNode{prevNode}
		prevNode.next = node
		prevNode = node
	}
	return pipeline, nil
}

func (p *InsidePipeline) Run(ctx context.Context, srv *server.Server) error {
	return p.root.start(ctx, srv)
}
