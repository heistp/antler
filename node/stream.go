// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import "context"

type Stream struct {
}

// Run implements runner
func (s *Stream) Run(ctx context.Context, chl *child, ifb Feedback,
	rec *recorder, cxl chan canceler) (ofb Feedback, err error) {
	return
}
