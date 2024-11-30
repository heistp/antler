// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2023 Pete Heist

package version

import (
	"runtime/debug"
)

// version represents the Antler version.
const version = "0.7.0"

// Version returns the version string.
func Version() string {
	v := version
	var c, m string

	if i, ok := debug.ReadBuildInfo(); ok {
		for _, s := range i.Settings {
			switch s.Key {
			case "vcs.revision":
				c = s.Value
			case "vcs.modified":
				m = s.Value
			}
		}
	}
	if c != "" {
		v += "-"
		v += c[:8]
	}
	if m == "true" {
		v += "+"
	}
	return v
}
