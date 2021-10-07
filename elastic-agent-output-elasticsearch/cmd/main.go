// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"os"

	"github.com/blakerouse/elastic-agent-sdk/pkg/cmd"

	"github.com/blakerouse/elastic-agent-output-elasticsearch/output"
)

func main() {
	if err := cmd.GenRootCmd(output.New()).Execute(); err != nil {
		os.Exit(1)
	}
}
