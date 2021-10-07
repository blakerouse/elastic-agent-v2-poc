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

package listener

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
)

const socketFileMode = os.FileMode(0740)

func New(host string, port int) (net.Listener, error) {
	network, path, err := parse(host, port)
	if err != nil {
		return nil, err
	}

	if network == "unix" {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			if err := os.Remove(path); err != nil {
				return nil, fmt.Errorf("cannot remote existing unit socket file at location %s: %w", path, err)
			}
		}
	}

	l, err := net.Listen(network, path)
	if err != nil {
		return nil, err
	}

	// Ensure file mode
	if network == "unix" {
		if err := os.Chmod(path, socketFileMode); err != nil {
			return nil, fmt.Errorf("could not set mode %d for unix socket file at location %s: %w", socketFileMode, path, err)
		}
	}

	return l, nil
}

func Cleanup(host string) error {
	network, path, err := parse(host, 0)
	if err != nil {
		return err
	}

	if network == "unix" {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("cannot remote unix socket file at location %s: %w", path, err)
			}
		}
	}
	return nil
}

func parse(host string, port int) (string, string, error) {
	url, err := url.Parse(host)
	if err != nil {
		return "", "", err
	}

	// fallback to tcp
	if len(url.Host) == 0 && len(url.Scheme) == 0 {
		addr := host + ":" + strconv.Itoa(port)
		return "tcp", addr, nil
	}

	switch url.Scheme {
	case "http":
		return "tcp", url.Host, nil
	case "unix":
		return url.Scheme, filepath.Join(url.Host, url.Path), nil
	default:
		return "", "", fmt.Errorf("unknown scheme %s for host string %s", url.Scheme, host)
	}
}