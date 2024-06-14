// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package antler

import "context"

// Index is a reporter that creates an index.html file for a Group.
//
// TODO implement Index reporter
type Index struct {
}

// report implements multiReporter
func (*Index) report(ctx context.Context, work resultRW,
	data <-chan testData) error {
	return nil
}
