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

// +build mage

package main

import (
	"os"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

const (
	goLintRepo     = "golang.org/x/lint/golint"
	goLicenserRepo = "github.com/elastic/go-licenser"
	goProtoc       = "google.golang.org/protobuf/cmd/protoc-gen-go"
	goGRPC         = "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
)

// Aliases for commands required by master makefile
var Aliases = map[string]interface{}{
	"fmt":   Format.All,
	"check": Check.All,
}

// Prepare tasks related to bootstrap the environment or get information about the environment.
type Prepare mg.Namespace

// Format automatically format the code.
type Format mg.Namespace

// Check namespace contains tasks related check the actual code quality.
type Check mg.Namespace

// InstallGoLicenser install go-licenser to check license of the files.
func (Prepare) InstallGoLicenser() error {
	return GoGet(goLicenserRepo)
}

// InstallGoLint for the code.
func (Prepare) InstallGoLint() error {
	return GoGet(goLintRepo)
}

// InstallGRPCGen for the code.
func (Prepare) InstallGRPCGen() error {
	return combineErr(
		GoGet(goProtoc),
		GoGet(goGRPC),
	)
}

// Update generates client/server code based on proto definition.
func Update() error {
	mg.Deps(Prepare.InstallGRPCGen)
	defer mg.SerialDeps(Format.All)
	return combineErr(
		sh.RunV(
			"protoc",
			"--go_out=pkg", "--go_opt=paths=source_relative",
			"--go-grpc_out=pkg", "--go-grpc_opt=paths=source_relative",
			"proto/control.proto"),
		sh.RunV(
			"protoc",
			"--go_out=pkg", "--go_opt=paths=source_relative",
			"--go-grpc_out=pkg", "--go-grpc_opt=paths=source_relative",
			"proto/event.proto"),
		sh.RunV(
			"protoc",
			"--go_out=pkg", "--go_opt=paths=source_relative",
			"proto/node.proto"),
	)
}

// All format automatically all the codes.
func (Format) All() {
	mg.SerialDeps(Format.License)
}

// License applies the right license header.
func (Format) License() error {
	mg.Deps(Prepare.InstallGoLicenser)
	return combineErr(
		sh.RunV("go-licenser", "-license", "ASL2"),
		sh.RunV("go-licenser", "-license", "ASL2", "-ext", ".proto"),
	)
}

// All run all the code checks.
func (Check) All() {
	mg.SerialDeps(Check.License, Check.GoLint)
}

// GoLint run the code through the linter.
func (Check) GoLint() error {
	mg.Deps(Prepare.InstallGoLint)
	packagesString, err := sh.Output("go", "list", "./...")
	if err != nil {
		return err
	}

	packages := strings.Split(packagesString, "\n")
	for _, pkg := range packages {
		if strings.Contains(pkg, "/vendor/") {
			continue
		}

		if e := sh.RunV("golint", "-set_exit_status", pkg); e != nil {
			err = multierror.Append(err, e)
		}
	}

	return err
}

// License makes sure that all the Golang files have the appropriate license header.
func (Check) License() error {
	mg.Deps(Prepare.InstallGoLicenser)
	// exclude copied files until we come up with a better option
	return combineErr(
		sh.RunV("go-licenser", "-d", "-license", "ASL2"),
	)
}

// GoGet fetch a remote dependencies.
func GoGet(link string) error {
	_, err := sh.Exec(map[string]string{"GO111MODULE": "off"}, os.Stdout, os.Stderr, "go", "get", link)
	return err
}

func combineErr(errors ...error) error {
	var e error
	for _, err := range errors {
		if err == nil {
			continue
		}
		e = multierror.Append(e, err)
	}
	return e
}
