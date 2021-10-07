// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package output

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"

	"github.com/blakerouse/elastic-agent-sdk/pkg/utils/transport/tlscommon"
	uurl "github.com/blakerouse/elastic-agent-sdk/pkg/utils/url"
)

// The timeout would be driven by the server for long poll.
// Giving it some sane long value.
const httpTransportLongPollTimeout = 10 * time.Minute

var hasScheme = regexp.MustCompile(`^([a-z][a-z0-9+\-.]*)://`)

// Config is the configuration for elasticsearch.
type Config struct {
	Protocol       string            `config:"protocol"`
	Hosts          []string          `config:"hosts"`
	Path           string            `config:"path"`
	Headers        map[string]string `config:"headers"`
	Username       string            `config:"username"`
	Password       string            `config:"password"`
	APIKey         string            `config:"api_key"`
	ServiceToken   string            `config:"service_token"`
	ProxyURL       string            `config:"proxy_url"`
	ProxyDisable   bool              `config:"proxy_disable"`
	ProxyHeaders   map[string]string `config:"proxy_headers"`
	TLS            *tlscommon.Config `config:"ssl"`
	MaxRetries     int               `config:"max_retries"`
	MaxConnPerHost int               `config:"max_conn_per_host"`
	Timeout        time.Duration     `config:"timeout"`
}

// InitDefaults initializes the defaults for the configuration.
func (c *Config) InitDefaults() {
	c.Protocol = "http"
	c.Hosts = []string{"localhost:9200"}
	c.Timeout = 90 * time.Second
	c.MaxRetries = 3
	c.MaxConnPerHost = 128
}

// Validate ensures that the configuration is valid.
func (c *Config) Validate() error {
	if c.APIKey != "" {
		return fmt.Errorf("cannot connect to elasticsearch with api_key; must use username/password")
	}
	if c.ProxyURL != "" && !c.ProxyDisable {
		if _, err := uurl.Parse(c.ProxyURL); err != nil {
			return err
		}
	}
	if c.TLS != nil && c.TLS.IsEnabled() {
		_, err := tlscommon.LoadTLSConfig(c.TLS)
		if err != nil {
			return err
		}
	}
	return nil
}

// ToESConfig converts the configuration object into the config for the elasticsearch client.
func (c *Config) ToESConfig() (elasticsearch.Config, error) {
	// build the addresses
	addrs := make([]string, len(c.Hosts))
	for i, host := range c.Hosts {
		addr, err := makeURL(c.Protocol, c.Path, host, 9200)
		if err != nil {
			return elasticsearch.Config{}, err
		}
		addrs[i] = addr
	}

	// build the transport from the config
	httpTransport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		DisableKeepAlives:     false,
		DisableCompression:    false,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   32,
		MaxConnsPerHost:       c.MaxConnPerHost,
		IdleConnTimeout:       60 * time.Second,
		ResponseHeaderTimeout: c.Timeout,
		ExpectContinueTimeout: 1 * time.Second,
	}

	if c.TLS != nil && c.TLS.IsEnabled() {
		tls, err := tlscommon.LoadTLSConfig(c.TLS)
		if err != nil {
			return elasticsearch.Config{}, err
		}
		httpTransport.TLSClientConfig = tls.ToConfig()
	}

	if !c.ProxyDisable {
		if c.ProxyURL != "" {
			proxyUrl, err := uurl.Parse(c.ProxyURL)
			if err != nil {
				return elasticsearch.Config{}, err
			}
			httpTransport.Proxy = http.ProxyURL(proxyUrl)
		} else {
			httpTransport.Proxy = http.ProxyFromEnvironment
		}

		var proxyHeaders http.Header
		if len(c.ProxyHeaders) > 0 {
			proxyHeaders = make(http.Header, len(c.ProxyHeaders))
			for k, v := range c.ProxyHeaders {
				proxyHeaders.Add(k, v)
			}
		}
		httpTransport.ProxyConnectHeader = proxyHeaders
	}

	h := http.Header{}
	for key, val := range c.Headers {
		h.Set(key, val)
	}

	return elasticsearch.Config{
		Addresses:    addrs,
		Username:     c.Username,
		Password:     c.Password,
		ServiceToken: c.ServiceToken,
		Header:       h,
		Transport:    httpTransport,
		MaxRetries:   c.MaxRetries,
	}, nil
}

func makeURL(defaultScheme string, defaultPath string, rawURL string, defaultPort int) (string, error) {
	if defaultScheme == "" {
		defaultScheme = "http"
	}
	if !hasScheme.MatchString(rawURL) {
		rawURL = fmt.Sprintf("%v://%v", defaultScheme, rawURL)
	}
	addr, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	scheme := addr.Scheme
	host := addr.Host
	port := strconv.Itoa(defaultPort)

	if host == "" {
		host = "localhost"
	} else {
		// split host and optional port
		if splitHost, splitPort, err := net.SplitHostPort(host); err == nil {
			host = splitHost
			port = splitPort
		}

		// Check if ipv6
		if strings.Count(host, ":") > 1 && strings.Count(host, "]") == 0 {
			host = "[" + host + "]"
		}
	}

	// Assign default path if not set
	if addr.Path == "" {
		addr.Path = defaultPath
	}

	// reconstruct url
	addr.Scheme = scheme
	addr.Host = host + ":" + port
	return addr.String(), nil
}
