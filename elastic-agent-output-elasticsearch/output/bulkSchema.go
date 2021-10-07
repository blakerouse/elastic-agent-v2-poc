// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package output

type bulkIndexerResponse struct {
	Took      int            `json:"took"`
	HasErrors bool           `json:"errors"`
	Items     []bulkStubItem `json:"items,omitempty"`
}

// commented out fields we don't use; no point decoding.
type bulkStubItem struct {
	//Index  *BulkIndexerResponseItem `json:"index"`
	//Delete *BulkIndexerResponseItem `json:"delete"`
	Create *bulkIndexerResponseItem `json:"create"`
	//Update *BulkIndexerResponseItem `json:"update"`
}

// commented out fields we don't use; no point decoding.
type bulkIndexerResponseItem struct {
	//	Index      string `json:"_index"`
	DocumentID string `json:"_id"`
	//	Version    int64  `json:"_version"`
	//	Result     string `json:"result"`
	Status int `json:"status"`
	//	SeqNo      int64  `json:"_seq_no"`
	//	PrimTerm   int64  `json:"_primary_term"`

	//	Shards struct {
	//		Total      int `json:"total"`
	//		Successful int `json:"successful"`
	//		Failed     int `json:"failed"`
	//	} `json:"_shards"`

	//   struct {
	//  	Type   string `json:"type"`
	//  	Reason string `json:"reason"`
	//  	Cause  struct {
	//  		Type   string `json:"type"`
	//  		Reason string `json:"reason"`
	//  	} `json:"caused_by"`
	//   } `json:"error,omitempty"`
}