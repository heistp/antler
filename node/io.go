// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"io"
	"os"
)

// ioConn combines a ReadCloser and a WriteCloser into a ReadWriteCloser.
type ioConn struct {
	io.ReadCloser
	io.WriteCloser
	closeRead  bool
	closeWrite bool
}

// Close may call Close on the ReadCloser or WriteCloser according to the values
// of closeRead and closeWrite, returning up to one error.
func (c ioConn) Close() (err error) {
	if c.closeRead {
		err = c.ReadCloser.Close()
	}
	if c.closeWrite {
		if e := c.WriteCloser.Close(); e != nil && err == nil {
			err = e
		}
	}
	return
}

// StdioConn is a ReadWriteCloser used for stdio. On Close, only stdout is
// closed.
func StdioConn() io.ReadWriteCloser {
	return &ioConn{os.Stdin, os.Stdout, false, true}
}

// PipeConn returns a pair of ReadWriteClosers connected by two Pipes, one each
// direction.
func PipeConn() (conn1, conn2 io.ReadWriteCloser) {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()
	conn1 = &ioConn{r1, w2, true, true}
	conn2 = &ioConn{r2, w1, true, true}
	return
}
