// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package metric

import (
	"fmt"
	"time"
)

// Bitrate is a bitrate in bits per second.
type Bitrate int64

const (
	Bps  Bitrate = 1
	Kbps         = 1000 * Bps
	Mbps         = 1000 * Kbps
	Gbps         = 1000 * Mbps
	Tbps         = 1000 * Gbps
)

var qdiscRateUnits = map[string]string{
	"K": "Kbit",
	"M": "Mbit",
	"G": "Gbit",
	"T": "Tbit",
}

var stdRateUnits = map[string]string{
	"K": "Kbps",
	"M": "Mbps",
	"G": "Gbps",
	"T": "Tbps",
}

func CalcBitrate(bytes Bytes, dur time.Duration) Bitrate {
	return Bitrate(8 * float64(bytes) / float64(dur.Seconds()))
}

func MaxBitrate(bitrate ...Bitrate) (max Bitrate) {
	for i, b := range bitrate {
		if i == 0 || b > max {
			max = b
		}
	}
	return
}

func MinBitrate(bitrate ...Bitrate) (min Bitrate) {
	for i, b := range bitrate {
		if i == 0 || b < min {
			min = b
		}
	}
	return
}

// Bps returns the Bitrate in bits per second.
func (b Bitrate) Bps() float64 {
	return float64(b) / float64(Bps)
}

// Kbps returns the Bitrate in kilobits per second.
func (b Bitrate) Kbps() float64 {
	return float64(b) / float64(Kbps)
}

// Mbps returns the Bitrate in megabits per second.
func (b Bitrate) Mbps() float64 {
	return float64(b) / float64(Mbps)
}

// Gbps returns the Bitrate in gigabits per second.
func (b Bitrate) Gbps() float64 {
	return float64(b) / float64(Gbps)
}

// Tbps returns the Bitrate in terabits per second.
func (b Bitrate) Tbps() float64 {
	return float64(b) / float64(Tbps)
}

// Qdisc returns a formatted string suitable for Linux qdisc parameters.
func (b Bitrate) Qdisc() string {
	return b.format(qdiscRateUnits)
}

func (b Bitrate) String() string {
	return b.format(stdRateUnits)
}

func (b Bitrate) format(units map[string]string) string {
	switch {
	case b < 1*Kbps:
		return fmt.Sprintf("%dbps", b)
	case b < 10*Kbps:
		return trimFloat(b.Kbps(), 3) + units["K"]
	case b < 100*Kbps:
		return trimFloat(b.Kbps(), 2) + units["K"]
	case b < 1*Mbps:
		return trimFloat(b.Kbps(), 1) + units["K"]
	case b < 10*Mbps:
		return trimFloat(b.Mbps(), 3) + units["M"]
	case b < 100*Mbps:
		return trimFloat(b.Mbps(), 2) + units["M"]
	case b < 1*Gbps:
		return trimFloat(b.Mbps(), 1) + units["M"]
	case b < 10*Gbps:
		return trimFloat(b.Gbps(), 3) + units["G"]
	case b < 100*Gbps:
		return trimFloat(b.Gbps(), 2) + units["G"]
	case b < 1*Tbps:
		return trimFloat(b.Gbps(), 1) + units["G"]
	default:
		return trimFloat(b.Tbps(), 3) + units["T"]
	}
}
