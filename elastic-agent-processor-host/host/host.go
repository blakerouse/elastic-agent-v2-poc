package host

import (
	"context"
	"encoding/json"

	"github.com/blakerouse/elastic-agent-sdk/pkg/component"
	"github.com/blakerouse/elastic-agent-sdk/pkg/events"
	"github.com/elastic/go-sysinfo"
	"github.com/elastic/go-ucfg"
	"github.com/rs/zerolog/log"
)

func New() component.ReceiveSendComponent {
	var data json.RawMessage
	cfgFunc := func(_ *ucfg.Config) (interface{}, error) {
		// load host information on config change
		info, err := sysinfo.Host()
		if err != nil {
			log.Error().Err(err).Msg("failed to load host information")
			return nil, err
		}
		host := info.Info()
		raw, err := json.Marshal(host)
		if err != nil {
			return nil, err
		}
		data = raw
		return nil, nil
	}
	evtFunc := func(ctx context.Context, evt *events.Event, _ interface{}) error {
		return evt.PutEncoded("host", data)
	}

	return component.NewProcessor("host", "alpha", cfgFunc, evtFunc)
}