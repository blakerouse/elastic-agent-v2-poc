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

package debug

import "github.com/blakerouse/elastic-agent-sdk/pkg/component"

// DebuggableComponent is a component that can be used by the debug sub-command.
type DebuggableComponent interface {
	component.Component

	// DebugConfig returns the default configuration to use for debugging the component.
	//
	// This can be overridden by command-line arguments to the debug sub-command. When one
	// is not provided to the sub-command then this configuration is used.
	DebugConfig() string
}