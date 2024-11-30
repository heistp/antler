// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2022 Pete Heist

package metric

import (
	"strconv"
	"strings"
)

// trimFloat calls formatFloat with trim set to true.
func trimFloat(f float64, prec int) (s string) {
	return formatFloat(f, prec, true)
}

// formatFloat formats a float64 to the specified precision and trim.
func formatFloat(f float64, prec int, trim bool) (s string) {
	s = strconv.FormatFloat(f, 'f', prec, 64)
	if trim {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return
}
