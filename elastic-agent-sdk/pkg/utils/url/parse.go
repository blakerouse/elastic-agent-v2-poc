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

package url

import (
	"net/url"
	"strings"
)

type ParseHint func(raw string) string

// Parse tries to parse a URL and return the parsed result.
func Parse(raw string, hints ...ParseHint) (*url.URL, error) {
	if raw == "" {
		return nil, nil
	}

	if len(hints) == 0 {
		hints = append(hints, WithDefaultScheme("http"))
	}

	if strings.Index(raw, "://") == -1 {
		for _, hint := range hints {
			raw = hint(raw)
		}
	}

	return url.Parse(raw)
}

func WithDefaultScheme(scheme string) ParseHint {
	return func(raw string) string {
		if !strings.Contains(raw, "://") {
			return scheme + "://" + raw
		}
		return raw
	}
}
