// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/sys/unix"
)

// setSockoptString sets a string option on a TCP socket.
func setTCPSockoptString(conn *net.TCPConn, level, opt int, what, value string) (
	err error) {
	var f *os.File
	if f, err = conn.File(); err != nil {
		return
	}
	defer f.Close()
	err = setSockoptString(int(f.Fd()), level, opt, what, value)
	return
}

// setSockoptString sets a string socket option.
func setSockoptString(fd, level, opt int, what, value string) (err error) {
	if err = unix.SetsockoptString(fd, level, opt, value); err != nil {
		err = fmt.Errorf("error setting %s to '%s': %w", what, value, err)
	}
	return
}
