package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/blakerouse/elastic-agent-sdk/pkg/proto"
	"github.com/blakerouse/elastic-agent-sdk/pkg/utils/authority"
	"github.com/blakerouse/elastic-agent-sdk/pkg/utils/sleep"
	"github.com/elastic/elastic-agent/server"
	protobuf "github.com/golang/protobuf/proto"
	"github.com/rs/xid"
	"github.com/rs/zerolog/log"
)

var (
	ErrInvalidPipeline = errors.New("invalid pipeline")
)

// Socket is an exposed socket by the node in the pipeline that the
// previous node in the pipeline can connect.
type Socket struct {
	ID string
	CA *authority.CertificateAuthority
	Pair *authority.Pair
}

// GetAddr returns the address of the socket.
func (s Socket) GetAddr() string {
	return fmt.Sprintf("unix://agent-pnode-%s.sock", s.ID)
}

// Process is the running process for the node in the pipeline
type Process struct {
	os.Process
	Stdin io.WriteCloser
}

// Node is single instance of a component in the pipeline.
type Node struct {
	Component *Component
	Config string

	id string
	socket Socket
	state *server.ApplicationState
	process *Process

	prev []*Node
	next *Node
}

func (n *Node) ID() string {
	return n.id
}

func (n *Node) start(ctx context.Context, srv *server.Server) error {
	if n.next != nil {
		if err := n.next.start(ctx, srv); err != nil {
			return err
		}
	}
	if n.process != nil {
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

func (n *Node) exec(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, n.Component.Executable, "run")
	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	err = cmd.Start()
	if err != nil {
		return err
	}
	err = n.writeConnInfo(stdin)
	if err != nil {
		return err
	}
	n.process = &Process{
		Process: *cmd.Process,
		Stdin: stdin,
	}

	res := make(chan *os.ProcessState)
	go func() {
		proc, _ := n.process.Wait()
		res <- proc
	}()

	go func() {
		proc := <-res
		select {
		case <-ctx.Done():
			return
		default:
		}

		log.Error().
			Int("pid", proc.Pid()).
			Int("exitcode", proc.ExitCode()).
			Str("component", n.Component.Info.Name).
			Msg("exited unexpectedly")

		sleep.WithContext(ctx, 1 * time.Second)
		for {
			err := n.exec(ctx)
			if err != nil && err != context.Canceled {
				log.Error().Err(err).Str("component", n.Component.Info.Name).Msg("failed to restart")
				sleep.WithContext(ctx, 1 * time.Second)
			} else {
				break
			}
		}
	}()

	return nil
}

func (n *Node) writeConnInfo(w io.Writer) error {
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

// Pipeline is a complete pipeline of nodes (aka. component instances)
// that are composed together passing events to each other through GRPC.
type Pipeline struct {
	root *Node
}

// NewPipeline creates a new pipeline from the defined components.
func NewPipeline(nodes ...*Node) (*Pipeline, error) {
	if len(nodes) < 2 {
		return nil, ErrInvalidPipeline
	}
	firstNode := nodes[0]
	if firstNode.Component.Info.Receiver {
		// first component cannot be a receiver
		return nil, ErrInvalidPipeline
	}
	lastNode := nodes[len(nodes) - 1]
	if lastNode.Component.Info.Sender {
		// last component cannot be a sender
		return nil, ErrInvalidPipeline
	}
	for i := 1; i < len(nodes) - 1; i++ {
		node := nodes[i]
		if !node.Component.Info.Receiver || !node.Component.Info.Sender {
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
	pipeline := &Pipeline{
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
		node.prev = []*Node{prevNode}
		prevNode.next = node
		prevNode = node
	}
	return pipeline, nil
}

func (p *Pipeline) Run(ctx context.Context, srv *server.Server) error {
	return p.root.start(ctx, srv)
}
