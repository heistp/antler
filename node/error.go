// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import "errors"

// errDone is an internal error sent on error channels to indicate the
// completion of a goroutine.
var errDone = errors.New("done")
