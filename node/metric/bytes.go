// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package metric

import "fmt"

// Bytes is a number of bytes.
type Bytes uint64

const (
	Byte     Bytes = 1
	Kilobyte       = 1000 * Byte
	Megabyte       = 1000 * Kilobyte
	Gigabyte       = 1000 * Megabyte
	Terabyte       = 1000 * Gigabyte
	Petabyte       = 1000 * Terabyte
	Kibibyte       = 1024 * Byte
	Mebibyte       = 1024 * Kibibyte
	Gibibyte       = 1024 * Mebibyte
	Tebibyte       = 1024 * Gibibyte
	Pebibyte       = 1024 * Tebibyte
)

// Kilobytes returns the Bytes in Kilobytes.
func (b Bytes) Kilobytes() float64 {
	return float64(b) / float64(Kilobyte)
}

// Megabytes returns the Bytes in Megabytes.
func (b Bytes) Megabytes() float64 {
	return float64(b) / float64(Megabyte)
}

// Gigabytes returns the Bytes in Gigabytes.
func (b Bytes) Gigabytes() float64 {
	return float64(b) / float64(Gigabyte)
}

// Terabytes returns the Bytes in Terabytes.
func (b Bytes) Terabytes() float64 {
	return float64(b) / float64(Terabyte)
}

// Petabytes returns the Bytes in Petabytes.
func (b Bytes) Petabytes() float64 {
	return float64(b) / float64(Petabyte)
}

// Kibibytes returns the Bytes in Kibibytes.
func (b Bytes) Kibibytes() float64 {
	return float64(b) / float64(Kibibyte)
}

// Mebibytes returns the Bytes in Mebibytes.
func (b Bytes) Mebibytes() float64 {
	return float64(b) / float64(Mebibyte)
}

// Gibibytes returns the Bytes in Gibibytes.
func (b Bytes) Gibibytes() float64 {
	return float64(b) / float64(Gibibyte)
}

// Tebibytes returns the Bytes in Tebibytes.
func (b Bytes) Tebibytes() float64 {
	return float64(b) / float64(Tebibyte)
}

// Pebibytes returns the Bytes in Pebibytes.
func (b Bytes) Pebibytes() float64 {
	return float64(b) / float64(Pebibyte)
}

func (b Bytes) String() string {
	return fmt.Sprintf("Bytes[%d]", b)
}
