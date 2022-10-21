// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package metric

import (
	"fmt"
	"time"
)

// Tinit is the base time used for RelativeTime values.
var Tinit = time.Now()

// RelativeTime represents a time relative to a base time.
type RelativeTime time.Duration

// Time returns the absolute time for the given base time.
func (r RelativeTime) Time(base time.Time) time.Time {
	return base.Add(time.Duration(r))
}

// Duration returns the RelativeTime as a time.Duration.
func (r RelativeTime) Duration() time.Duration {
	return time.Duration(r)
}

// Now returns the current RelativeTime, with Tinit as a base.
func Now() RelativeTime {
	return RelativeTime(time.Since(Tinit))
}

// Relative returns the RelativeTime for t, with Tinit as a base.
func Relative(t time.Time) RelativeTime {
	return RelativeTime(t.Sub(Tinit))
}

func (r RelativeTime) String() string {
	return fmt.Sprintf("RelativeTime[%s]", time.Duration(r))
}
