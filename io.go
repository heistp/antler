// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2023 Pete Heist

package antler

import "io"

// multiWriteCloser returns an io.MultiWriter that uses the given WriteClosers
// as the Writers, so that writes to the returned Writer are duplicated to each
// of the given WriteClosers.
func multiWriteCloser(wc ...io.WriteCloser) io.Writer {
	var ww []io.Writer
	for _, w := range wc {
		ww = append(ww, w)
	}
	return io.MultiWriter(ww...)
}
