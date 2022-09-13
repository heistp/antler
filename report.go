// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

// A reporter can process data items from the node and take some action, such as
// saving results, generating plots, or emitting progress during the test.
//
// Reporters may return a nil channel if they choose not to handle the given ID.
// If a non-nil channel is returned, they must read from this channel until it
// is closed, and must write either an error or nil to the error channel to
// indicate completion.
type reporter interface {
	report(ID, chan error) chan interface{}
}
