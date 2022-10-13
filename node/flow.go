// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

// Flow is a string name identifying a flow.
type Flow string

// A Flower wraps the Flow method, to return a Flow associated with the
// implementation.
type Flower interface {
	Flow() Flow
}
