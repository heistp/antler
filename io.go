// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

package antler

import "io"

// multiWriteCloser provides an analog to io.MultiWriter for WriteClosers,
// returning a Writer that
func multiWriteCloser(wc ...io.WriteCloser) io.Writer {
	var ww []io.Writer
	for _, w := range wc {
		ww = append(ww, w)
	}
	return io.MultiWriter(ww...)
}
