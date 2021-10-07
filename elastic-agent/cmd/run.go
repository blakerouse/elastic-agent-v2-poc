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
	"fmt"
	"github.com/blakerouse/elastic-agent-sdk/pkg/utils/signal"
	"github.com/elastic/elastic-agent/agent"
	"github.com/spf13/cobra"
	"os"
)

func genRunCmd() *cobra.Command {
	runCmd := cobra.Command{
		Use:   "run",
		Short: "Run Elastic Agent",
		Run: func(cmd *cobra.Command, args []string) {
			mgr, err := agent.NewManager("components")
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "%s\n", err)
				os.Exit(1)
			}
			ctx := signal.HandleInterrupt(context.Background())
			err = mgr.Run(ctx)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "%s\n", err)
				os.Exit(1)
			}
		},
	}

	return &runCmd
}
