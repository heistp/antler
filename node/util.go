// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"fmt"
	"strings"
)

const (
	// thruputCap is the standard channel buffer capacity used for higher
	// throughput. Such usages must not depend on the buffer for correctness.
	thruputCap = 16
)

// typeBaseName returns the base type name for a (part after the last '.').
func typeBaseName(a interface{}) string {
	t := fmt.Sprintf("%T", a)
	return t[strings.LastIndex(t, ".")+1:]
}
