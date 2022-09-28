// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"net"
	"os"

	"golang.org/x/sys/unix"
)

// setSockoptString sets a string socket option.
func setSockoptString(conn *net.TCPConn, level, opt int, value string) (
	err error) {
	var f *os.File
	if f, err = conn.File(); err != nil {
		return
	}
	defer f.Close()
	err = unix.SetsockoptString(int(f.Fd()), level, opt, value)
	return
}
