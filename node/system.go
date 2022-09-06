// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"context"
)

// System is a runner that executes a system command.
type System struct {
	Command string // the system command to run
	//IgnoreErrors bool   // if true, errors do not fail the test
	//NoLogStdout  bool   // if true, do not log output from stdout
}

func (s *System) Run(ctx context.Context, chl *child, ifb Feedback,
	rec *recorder) (ofb Feedback, err error) {
	x := newExecutor(rec.Logf)
	err = x.Runcs(ctx, s.Command)
	return
}
