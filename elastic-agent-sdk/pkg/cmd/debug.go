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

package cmd

import (
	"context"
	"errors"
	"fmt"
	"github.com/blakerouse/elastic-agent-sdk/pkg/utils/authority"
	"github.com/rs/xid"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	protobuf "github.com/golang/protobuf/proto"
	"github.com/spf13/cobra"

	"github.com/blakerouse/elastic-agent-sdk/pkg/component"
	"github.com/blakerouse/elastic-agent-sdk/pkg/debug"
	"github.com/blakerouse/elastic-agent-sdk/pkg/events"
	"github.com/blakerouse/elastic-agent-sdk/pkg/proto"
	"github.com/blakerouse/elastic-agent-sdk/pkg/utils/signal"
)

func genDebugCmd(comp component.Component) *cobra.Command {
	name := comp.Name()
	debugCmd := cobra.Command{
		Use:   "debug",
		Short: "debug " + name,
		RunE: func(cmd *cobra.Command, args []string) error {
			debuggable, ok := comp.(debug.DebuggableComponent)
			if !ok {
				return errors.New("component must implement debug.DebuggableComponent to be debuggable")
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx = signal.HandleInterrupt(ctx)

			dir, err := ioutil.TempDir(os.TempDir(), "debug")
			if err != nil {
				return err
			}
			dir, err = filepath.Abs(dir)
			if err != nil {
				return err
			}
			defer os.RemoveAll(dir)

			// start the debug controller (acting as the agent)
			socket := fmt.Sprintf("unix://%s", filepath.Join(dir, "control.sock"))
			ctrl, err := debug.New(socket, debuggable.DebugConfig())
			if err != nil {
				return err
			}
			err = ctrl.Run(ctx)
			if err != nil {
				return err
			}
			node := &proto.Node{
				ID: xid.New().String(),
				Agent: ctrl.GetAgentConn(),
			}

			// start a receiver to catch the events sent by the component
			var receiver events.Receiver
			_, sender := comp.(component.SendComponent)
			if sender {
				ca, err := authority.NewCA()
				if err != nil {
					return err
				}
				pair, err := ca.GeneratePair()
				if err != nil {
					return err
				}
				logReceiver := debug.NewLogReceiver()
				eventsSocket := fmt.Sprintf("unix://%s", filepath.Join(dir, "events.sock"))
				node.Send = &proto.Send{
					Addr:       eventsSocket,
					ServerName: "localhost",
					CaCert: ca.Crt(),
					PeerCert: pair.Crt,
					PeerKey: pair.Key,
				}
				receiver = events.NewReceiver(&proto.Receive{
					Addr:     eventsSocket,
					CaCert: ca.Crt(),
					PeerCert: pair.Crt,
					PeerKey: pair.Key,
				}, logReceiver)
				err = receiver.Run(ctx)
				if err != nil {
					return err
				}
			}

			infoBytes, err := protobuf.Marshal(node)
			if err != nil {
				return fmt.Errorf("failed to marshal connection information: %w", err)
			}
			errChan := make(chan error)
			reader, writer := io.Pipe()
			go func() {
				_, err := writer.Write(infoBytes)
				_ = writer.Close()
				errChan <- err
			}()

			runner, err := component.NewRunner(reader, comp)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "failed to create runner: %s", err)
				os.Exit(1)
			}

			err = runner.Run(ctx)
			if err != nil {
				return err
			}
			err = <-errChan
			if err != nil {
				return err
			}
			return nil
		},
	}

	return &debugCmd
}
