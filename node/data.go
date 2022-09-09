// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"time"
)

// Series is a string name for a series. By convention, it matches
// [a-z][a-z\.]*.
type Series string

// DataPoint is a single time series data point.
type DataPoint struct {
	Series Series      // series the DataPoint belongs to
	Time   time.Time   // node time that the DataPoint was created
	Value  interface{} // the DataPoint value
}

// newDataPoint returns a new DataPoint.
func newDataPoint(series Series, time time.Time, value interface{}) DataPoint {
	return DataPoint{series, time, value}
}

// flags implements message
func (d DataPoint) flags() flag {
	return flagForward
}

// handle implements event
func (d DataPoint) handle(node *node) {
	node.parent.Send(d)
}
