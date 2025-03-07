// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2022 Pete Heist

package node

import (
	"context"
	"time"

	"github.com/heistp/antler/node/metric"
)

// Sleep is a runner that sleeps for the given Duration, or until canceled.
type Sleep metric.Duration

// UnmarshalText implements encoding.TextUnmarshaler.
func (s *Sleep) UnmarshalText(text []byte) (err error) {
	d := metric.Duration(*s)
	if err = d.UnmarshalText(text); err != nil {
		return
	}
	*s = Sleep(d)
	return
}

// Run implements runner
func (s *Sleep) Run(ctx context.Context, arg runArg) (ofb Feedback, err error) {
	select {
	case <-ctx.Done():
	case <-time.After(time.Duration(*s)):
	}
	return
}
