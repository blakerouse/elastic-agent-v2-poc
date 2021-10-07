// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

// +build mage

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

const (
	goLintRepo     = "golang.org/x/lint/golint"
	goLicenserRepo = "github.com/elastic/go-licenser"
)

// Aliases for commands required by master makefile
var Aliases = map[string]interface{}{
	"fmt":   Format.All,
	"check": Check.All,
	"build": Build.All,
}

// Prepare tasks related to bootstrap the environment or get information about the environment.
type Prepare mg.Namespace

// Format automatically format the code.
type Format mg.Namespace

// Check namespace contains tasks related check the actual code quality.
type Check mg.Namespace

// Build namespace contains tasks related to building.
type Build mg.Namespace

// InstallGoLicenser install go-licenser to check license of the files.
func (Prepare) InstallGoLicenser() error {
	return GoGet(goLicenserRepo)
}

// InstallGoLint for the code.
func (Prepare) InstallGoLint() error {
	return GoGet(goLintRepo)
}

// Update formats all the code.
func Update() {
	mg.SerialDeps(Format.All)
}

// All format automatically all the codes.
func (Format) All() {
	mg.SerialDeps(Format.License)
}

// License applies the right license header.
func (Format) License() error {
	mg.Deps(Prepare.InstallGoLicenser)
	return sh.RunV("go-licenser", "-license", "ASL2")
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
	return sh.RunV("go-licenser", "-d", "-license", "ASL2")
}

// Agent builds the agent code
func (Build) Agent() error {
	if err := os.MkdirAll("build", 0755); err != nil {
		return err
	}

	return sh.RunV("go", "build", "-o", "build/elastic-agent", "main.go")
}

// Agent builds all the components.
func (Build) Components() error {
	if err := os.MkdirAll("build/components", 0755); err != nil {
		return err
	}

	var e error
	components, err := ioutil.ReadDir("..")
	if err != nil {
		return err
	}
	for _, comp := range components {
		name := comp.Name()
		if strings.HasPrefix(name, "elastic-agent-") && name != "elastic-agent-sdk" {
			fmt.Printf("Building component %s...\n", name)
			err := sh.RunV("mage", "-d", filepath.Join("..", name), "build")
			if err != nil {
				e = multierror.Append(e, err)
				continue
			}
			compName := strings.TrimPrefix(name, "elastic-agent-")
			err = sh.RunV("cp", "-f", filepath.Join("..", name, "build", compName), "build/components/")
			if err != nil {
				e = multierror.Append(e, err)
			}
		}
	}
	if e != nil {
		return e
	}

	return nil
}

// All run all the build steps
func (Build) All() {
	mg.SerialDeps(Build.Agent, Build.Components)
}

// GoGet fetch a remote dependencies.
func GoGet(link string) error {
	_, err := sh.Exec(map[string]string{"GO111MODULE": "off"}, os.Stdout, os.Stderr, "go", "get", link)
	return err
}
