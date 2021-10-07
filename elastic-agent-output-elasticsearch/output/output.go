// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package output

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/rs/zerolog/log"
	"sync"
	"time"

	"github.com/blakerouse/elastic-agent-sdk/pkg/control"
	"github.com/blakerouse/elastic-agent-sdk/pkg/events"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/elastic/go-ucfg"
	"go.elastic.co/apm/module/apmelasticsearch"
)

// BulkBufSlop is the extra space added to a buffer for the body of each event.
const BulkBufSlop = 64

type Output struct {
	ctx context.Context
	es *elasticsearch.Client

	bufPool sync.Pool
}

func New() *Output {
	return &Output{
		bufPool: sync.Pool{New: func() interface{} {
			return &bytes.Buffer{}
		}},
	}
}

func (o *Output) Name() string {
	return "elasticsearch"
}

func (o *Output) Version() string {
	return "alpha"
}

func (o *Output) Actions() []control.Action {
	return nil
}

func (o *Output) Run(ctx context.Context, cfg *ucfg.Config) error {
	o.ctx = ctx
	if err := o.Reload(cfg); err != nil {
		return err
	}
	return nil
}

func (o *Output) Reload(cfg *ucfg.Config) error {
	var parsed Config
	if err := cfg.Unpack(&parsed, ucfg.PathSep(".")); err != nil {
		return err
	}
	escfg, err := parsed.ToESConfig()
	if err != nil {
		return err
	}
	escfg.Transport = apmelasticsearch.WrapRoundTripper(escfg.Transport)
	es, err := elasticsearch.NewClient(escfg)
	if err != nil {
		return err
	}
	o.es = es
	return nil
}

func (o *Output) Receive(ctx context.Context, events events.Events) ([]uint64, error) {
	start := time.Now()

	bufSz := 0
	evts := make([]doc, 0, len(events))
	for _, e := range events {
		doc, err := newDoc(e)
		if err != nil {
			return nil, err
		}
		evts = append(evts, doc)
		bufSz += len(doc.data) + BulkBufSlop
	}

	buf := o.newPool(bufSz)
	defer o.putPool(buf)
	for _, e := range evts {
		e.writeDoc(buf)
	}
	req := esapi.BulkRequest{
		Body: bytes.NewReader(buf.Bytes()),
	}
	res, err := req.Do(ctx, o.es)

	if err != nil {
		// error is logged, but not returned (previous component needs to resend)
		log.Error().Err(err).Msg("failed to bulk create")
		return []uint64{}, nil
	}
	if res.Body != nil {
		defer res.Body.Close()
	}
	if res.IsError() {
		// error is logged, but not returned (previous component needs to resend)
		log.Error().Str("err", res.String()).Msg("failed to bulk create")
		return []uint64{}, nil
	}

	buf.Reset()
	bodySz, err := buf.ReadFrom(res.Body)
	if err != nil {
		// error is logged, but not returned (previous component needs to resend)
		log.Error().
			Err(err).
			Msg("response error")
		return []uint64{}, nil
	}

	var blk bulkIndexerResponse
	blk.Items = make([]bulkStubItem, 0, len(evts))

	if err := json.Unmarshal(buf.Bytes(), &blk); err != nil {
		// error is logged, but not returned (previous component needs to resend)
		log.Error().
			Err(err).
			Msg("unmarshal error")
		return []uint64{}, nil
	}

	log.Trace().
		Err(err).
		Int("took", blk.Took).
		Dur("rtt", time.Since(start)).
		Bool("hasErrors", blk.HasErrors).
		Int("cnt", len(blk.Items)).
		Int("bufSz", bufSz).
		Int64("bodySz", bodySz).
		Msg("bulk create")

	success := make([]uint64, 0, len(blk.Items))
	for _, item := range blk.Items {
		doc, ok := getDoc(evts, item.Create.DocumentID)
		if !ok {
			continue
		}
		if item.Create.Status == 200 || item.Create.Status == 201 {
			success = append(success, doc.internalID)
		}
	}
	return success, nil
}

func (o *Output) newPool(size int) *bytes.Buffer {
	buf := o.bufPool.Get().(*bytes.Buffer)
	buf.Grow(size)
	return buf
}

func (o *Output) putPool(b *bytes.Buffer) {
	b.Reset()
	o.bufPool.Put(b)
}

func getDoc(docs []doc, id string) (doc, bool) {
	for _, doc := range docs {
		if doc.id == id {
			return doc, true
		}
	}
	return doc{}, false
}
