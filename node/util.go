// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2022 Pete Heist

package node

import (
	"fmt"
	"strings"
)

// typeBaseName returns the base type name for a (part after the last '.').
func typeBaseName(a any) string {
	t := fmt.Sprintf("%T", a)
	return t[strings.LastIndex(t, ".")+1:]
}
