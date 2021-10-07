// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package output

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/rs/xid"

	"github.com/blakerouse/elastic-agent-sdk/pkg/events"
)

var ErrNoIndex = errors.New("event missing _index field")

type doc struct {
	id string
	internalID uint64
	index string
	data []byte
}

func newDoc(evt *events.Event) (doc, error) {
	index, ok, err := evt.GetString("_index")
	if err != nil {
		return doc{}, err
	}
	if !ok {
		return doc{}, ErrNoIndex
	}
	evt.Delete("_index")
	id, ok, err := evt.GetString("_id")
	if err != nil {
		return doc{}, err
	}
	if !ok {
		id = xid.New().String()
	} else {
		evt.Delete("_id")
	}
	bytes, err := json.Marshal(evt)
	if err != nil {
		return doc{}, err
	}
	return doc{
		id: id,
		internalID: evt.ID(),
		index: index,
		data: bytes,
	}, nil
}

func (d *doc) writeDoc(buf *bytes.Buffer) {
	buf.WriteString(`{"create":{"_id":"`)
	buf.WriteString(d.id)
	buf.WriteString(`","_index":"`)
	buf.WriteString(d.index)
	buf.WriteString("\"}}\n")
	buf.Write(d.data)
	buf.WriteRune('\n')
}
