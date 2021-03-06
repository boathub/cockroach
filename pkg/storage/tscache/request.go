// Copyright 2017 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package tscache

import (
	"github.com/google/btree"

	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/cockroach/pkg/util/interval"
	"github.com/cockroachdb/cockroach/pkg/util/uuid"
)

// Request holds the timestamp cache data from a single batch request. The
// requests are stored in a btree keyed by the timestamp and are "expanded" to
// populate the read/write interval caches if a potential conflict is detected
// due to an earlier request (based on timestamp) arriving.
type Request struct {
	Span      roachpb.RSpan
	Reads     []roachpb.Span
	Writes    []roachpb.Span
	Txn       roachpb.Span
	TxnID     uuid.UUID
	Timestamp hlc.Timestamp
	// Used to distinguish requests with identical timestamps. For actual
	// requests, the uniqueID value is >0. When probing the btree for requests
	// later than a particular timestamp a value of 0 is used.
	uniqueID int64
}

// Less implements the btree.Item interface.
func (cr *Request) Less(other btree.Item) bool {
	otherReq := other.(*Request)
	if cr.Timestamp.Less(otherReq.Timestamp) {
		return true
	}
	if otherReq.Timestamp.Less(cr.Timestamp) {
		return false
	}
	// Fallback to comparison of the uniqueID as a tie-breaker. This allows
	// multiple requests with the same timestamp to exist in the requests btree.
	return cr.uniqueID < otherReq.uniqueID
}

// numSpans returns the number of spans the request will expand into.
func (cr *Request) numSpans() int {
	n := len(cr.Reads) + len(cr.Writes)
	if cr.Txn.Key != nil {
		n++
	}
	return n
}

func (cr *Request) size() uint64 {
	var n uint64
	for i := range cr.Reads {
		s := &cr.Reads[i]
		n += cacheEntrySize(interval.Comparable(s.Key), interval.Comparable(s.EndKey))
	}
	for i := range cr.Writes {
		s := &cr.Writes[i]
		n += cacheEntrySize(interval.Comparable(s.Key), interval.Comparable(s.EndKey))
	}
	if cr.Txn.Key != nil {
		n += cacheEntrySize(interval.Comparable(cr.Txn.Key), nil)
	}
	return n
}
