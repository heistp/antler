// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import "time"

// readableTimeFormat is the time format used for human consumption.
const readableTimeFormat = "2006-01-02 15:04:05.000000"

// Duration wraps a time.Duration and adds a TextUnmarshaler for conversion from
// a string in CUE.
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
