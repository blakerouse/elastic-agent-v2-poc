module github.com/elastic/elastic-agent

go 1.16

require (
	github.com/blakerouse/elastic-agent-input-fake v0.0.0-20210928143458-82a1a1f07d64
	github.com/blakerouse/elastic-agent-output-elasticsearch v0.0.0-20210928143458-82a1a1f07d64
	github.com/blakerouse/elastic-agent-processor-host v0.0.0-20210928143458-82a1a1f07d64
	github.com/blakerouse/elastic-agent-processor-spooler v0.0.0-20210928143458-82a1a1f07d64
	github.com/blakerouse/elastic-agent-sdk v0.0.0-20210928143458-82a1a1f07d64
	github.com/elastic/beats/v7 v7.15.0
	github.com/elastic/go-ucfg v0.8.3
	github.com/golang/protobuf v1.5.2
	github.com/hashicorp/go-multierror v1.1.1
	github.com/magefile/mage v1.11.0
	github.com/rs/xid v1.3.0
	github.com/rs/zerolog v1.25.0
	github.com/spf13/cobra v1.2.1
	google.golang.org/grpc v1.41.0
)

replace (
	github.com/Shopify/sarama => github.com/elastic/sarama v1.19.1-0.20210823122811-11c3ef800752
	github.com/blakerouse/elastic-agent-input-fake => ../elastic-agent-input-fake
	github.com/blakerouse/elastic-agent-output-elasticsearch => ../elastic-agent-output-elasticsearch
	github.com/blakerouse/elastic-agent-processor-host => ../elastic-agent-processor-host
	github.com/blakerouse/elastic-agent-processor-spooler => ../elastic-agent-processor-spooler
	github.com/blakerouse/elastic-agent-sdk => ../elastic-agent-sdk
)
