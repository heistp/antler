// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package metric

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
