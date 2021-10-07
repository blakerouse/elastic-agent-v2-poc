module github.com/blakerouse/elastic-agent-processor-host

go 1.16

require (
	github.com/blakerouse/elastic-agent-sdk v0.0.0-20210928143458-82a1a1f07d64
	github.com/elastic/beats/v7 v7.15.0
	github.com/elastic/go-ucfg v0.8.3
	github.com/hashicorp/go-multierror v1.1.1
	github.com/magefile/mage v1.11.0
	github.com/rs/zerolog v1.25.0
)

replace (
	github.com/Shopify/sarama => github.com/elastic/sarama v1.19.1-0.20210823122811-11c3ef800752
	github.com/blakerouse/elastic-agent-sdk => ../elastic-agent-sdk
)
