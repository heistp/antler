// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package metric

import "time"

// Tinit is the base time used for RelativeTIme values.
var Tinit = time.Now()

// RelativeTime represents a time relative to Tinit.
type RelativeTime time.Duration

// Time returns the absolute time for the given Tinit.
func (r RelativeTime) Time(tinit time.Time) time.Time {
	return tinit.Add(time.Duration(r))
}

// Duration returns the RelativeTime as a time.Duration.
func (r RelativeTime) Duration() time.Duration {
	return time.Duration(r)
}

// Now returns the current RelativeTime.
func Now() RelativeTime {
	return RelativeTime(time.Since(Tinit))
}

// Relative returns the RelativeTime for the given time.
func Relative(t time.Time) RelativeTime {
	return RelativeTime(t.Sub(Tinit))
}
