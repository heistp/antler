// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import "context"

// Index is a reporter that creates an index.html file for a Group.
type Index struct {
}

// report implements reporter
func (*Index) report(ctx context.Context, rw rwer, in <-chan any,
	out chan<- any) (err error) {
	// TODO implement Index reporter
	return
}
