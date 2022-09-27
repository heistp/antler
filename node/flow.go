// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

// Series is a string name for a series. By convention, it matches
// [a-z][a-z\.]*.
type Series string

// A Serieser wraps the Series method, to return a Series associated with the
// implementation.
type Serieser interface {
	Series() Series
}
