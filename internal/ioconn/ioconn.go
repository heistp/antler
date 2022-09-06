// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

// Package ioconn provides conn implementations backed by an io.Read/WriteCloser.

package ioconn

import (
	"io"
	"os"
)

// Conn combines a ReadCloser and a WriteCloser.
type Conn struct {
	io.ReadCloser
	io.WriteCloser
	closeRead  bool
	closeWrite bool
}

// Close may call Close on the ReadCloser or WriteCloser according to the values
// of closeRead and closeWrite, returning up to one error.
func (c Conn) Close() (err error) {
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

// Stdio is a Conn used for stdio. On Close, only stdout is closed.
func Stdio() *Conn {
	return &Conn{os.Stdin, os.Stdout, false, true}
}

// Pipes returns a pair of Conns connected by two Pipes, one each direction.
func Pipes() (conn1, conn2 io.ReadWriteCloser) {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()
	conn1 = &Conn{r1, w2, true, true}
	conn2 = &Conn{r2, w1, true, true}
	return
}
