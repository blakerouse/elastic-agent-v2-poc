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
	"encoding/json"
	"fmt"
	"github.com/blakerouse/elastic-agent-sdk/pkg/component"
	"os"

	"github.com/spf13/cobra"
)


func genInfoCmd(info *component.Info) *cobra.Command {
	name := info.Name
	infoCmd := cobra.Command{
		Use:   "info",
		Short: "Info " + name,
		Run: func(cmd *cobra.Command, args []string) {
			bytes, err := json.Marshal(info)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "failed to encode component settings: %s", err)
				os.Exit(1)
			}
			cmd.OutOrStdout().Write(bytes)
		},
	}

	return &infoCmd
}
