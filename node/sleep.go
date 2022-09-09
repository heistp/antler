// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"context"
	"time"
)

// Sleep is a runner that sleeps for the given duration, with the format
// expected by time.ParseDuration.
type Sleep string

// Run implements runner
func (s Sleep) Run(ctx context.Context, arg runArg) (ofb Feedback, err error) {
	var d time.Duration
	if d, err = time.ParseDuration(string(s)); err != nil {
		return
	}
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
	return
}
