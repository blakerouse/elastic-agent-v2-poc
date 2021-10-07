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
	"os"

	"github.com/spf13/cobra"

	"github.com/blakerouse/elastic-agent-sdk/pkg/component"
	"github.com/blakerouse/elastic-agent-sdk/pkg/utils/signal"
)

func genRunCmd(comp component.Component) *cobra.Command {
	name := comp.Name()
	runCmd := cobra.Command{
		Use:   "run",
		Short: "Run " + name,
		Run: func(cmd *cobra.Command, args []string) {
			runner, err := component.NewRunner(cmd.InOrStdin(), comp)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "failed to create runner: %s", err)
				os.Exit(1)
			}
			err = runner.Run(signal.HandleInterrupt(context.Background()))
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "runner failed: %s", err)
				os.Exit(1)
			}
		},
	}

	return &runCmd
}
