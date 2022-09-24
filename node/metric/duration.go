// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package metric

import "time"

// Duration wraps a time.Duration and adds a TextUnmarshaler for conversion from
// a string, using time.ParseDuration.
type Duration time.Duration

// UnmarshalText implements encoding.TextUnmarshaler.
func (d *Duration) UnmarshalText(text []byte) (err error) {
	var dd time.Duration
	if dd, err = time.ParseDuration(string(text)); err != nil {
		return
	}
	*d = Duration(dd)
	return
}

// Duration returns the time.Duration.
func (d *Duration) Duration() time.Duration {
	return time.Duration(*d)
}
