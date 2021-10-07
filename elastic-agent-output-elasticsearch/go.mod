module github.com/blakerouse/elastic-agent-output-elasticsearch

go 1.16

require (
	github.com/blakerouse/elastic-agent-sdk v0.0.0-20210928143458-82a1a1f07d64
	github.com/elastic/go-elasticsearch/v8 v8.0.0-20210916085751-c2fb55d91ba4
	github.com/elastic/go-ucfg v0.8.3
	github.com/hashicorp/go-multierror v1.1.1
	github.com/magefile/mage v1.11.0
	github.com/rs/xid v1.3.0
	github.com/rs/zerolog v1.25.0
	go.elastic.co/apm/module/apmelasticsearch v1.14.0
)

replace github.com/blakerouse/elastic-agent-sdk => ../elastic-agent-sdk
