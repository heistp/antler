// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"strings"
	"time"
)

// Series is a string name for a series. By convention, it matches
// [a-z][a-z\-\.]*. The substring up to the first dot is always the node ID.
type Series string

// DataPoint is a single time series data point.
type DataPoint struct {
	Series Series      // series the DataPoint belongs to
	Time   Time        // node time that DataPoint was created
	Value  interface{} // the DataPoint value
}

// newDataPoint returns a new DataPoint.
func newDataPoint(series Series, time time.Time, value interface{}) DataPoint {
	return DataPoint{series, Time{time}, value}
}

// DataPoint implements DataPointer
func (d DataPoint) DataPoint() DataPoint {
	return d
}

// flags implements message
func (d DataPoint) flags() flag {
	return flagForward
}

// handle implements event
func (d DataPoint) handle(node *node) {
	node.parent.Send(d)
}

// Time is an alias for time.Time that provides domain appropriate JSON
// marshaling.
type Time struct {
	time.Time
}

// dataTimeFormat is the time format used for Time's.
var dataTimeFormat = "2006-01-02 15:04:05.000000.000"

// UnmarshalJSON implements json.Unmarshaler.
func (t *Time) UnmarshalJSON(b []byte) (err error) {
	var s string
	if s = strings.Trim(string(b), `"`); s == "" || s == "null" {
		return
	}
	var p time.Time
	if p, err = time.Parse(dataTimeFormat, s); err != nil {
		return
	}
	*t = Time{p}
	return
}

// MarshalJSON implements json.Marshaler.
func (t *Time) MarshalJSON() ([]byte, error) {
	var b strings.Builder
	b.Grow(len(dataTimeFormat) + 2)
	b.WriteByte('"')
	b.WriteString((*t).Format(dataTimeFormat))
	b.WriteByte('"')
	return []byte(b.String()), nil
}

// DataPointer wraps the DataPoint method, which returns a DataPoint.
type DataPointer interface {
	DataPoint() DataPoint
	message
}
