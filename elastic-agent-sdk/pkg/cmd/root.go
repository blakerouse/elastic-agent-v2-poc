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
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/blakerouse/elastic-agent-sdk/pkg/component"
)

// RootCmd handles all components command line interface
type RootCmd struct {
	cobra.Command
	RunCmd        *cobra.Command
	InfoCmd       *cobra.Command
	DebugCmd      *cobra.Command
}

// GenRootCmd returns the root command to use for your component.
func GenRootCmd(comp component.Component) *RootCmd {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Logger.With().Str("component", comp.Name()).Logger()

	_, receiver := comp.(component.ReceiveComponent)
	_, sender := comp.(component.SendComponent)
	if !receiver && !sender {
		panic("must implement at least one of: ReceiveComponent or SendComponent")
	}

	var actions []string
	for _, action := range comp.Actions() {
		actions = append(actions, action.Name())
	}
	info := &component.Info{
		Name:     comp.Name(),
		Version:  comp.Version(),
		Receiver: receiver,
		Sender:   sender,
		Actions:  actions,
	}

	rootCmd := &RootCmd{}
	rootCmd.Use = info.Name

	rootCmd.RunCmd = genRunCmd(comp)
	rootCmd.InfoCmd = genInfoCmd(info)
	rootCmd.DebugCmd = genDebugCmd(comp)

	// debug command is only added if implemented debuggable interface

	// root command is an alias for run
	rootCmd.Run = rootCmd.RunCmd.Run

	// Register subcommands
	rootCmd.AddCommand(rootCmd.RunCmd)
	rootCmd.AddCommand(rootCmd.InfoCmd)
	rootCmd.AddCommand(rootCmd.DebugCmd)

	return rootCmd
}
