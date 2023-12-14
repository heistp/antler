// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/sys/unix"
)

// Sockopt represents the information needed to set a socket option.
type Sockopt struct {
	// Type identifies the type of the option, and may be one of "string",
	// "int" or "byte".
	Type string

	// Level is the level argument passed to setsockopt().
	Level int

	// Opt is the option argument passed to setsockopt().
	Opt int

	// Name is a label for the socket option, used only for debugging purposes.
	Name string

	// Value is the value to set. For Type string, this must be a string. For
	// Type int or byte, this must be an int.
	Value any
}

// setTCP sets the socket option on the given TCPConn.
func (s Sockopt) setTCP(conn *net.TCPConn) (err error) {
	var f *os.File
	if f, err = conn.File(); err != nil {
		return
	}
	defer f.Close()
	err = s.set(int(f.Fd()))
	return
}

// set sets the socket option on the given file descriptor.
func (s Sockopt) set(fd int) (err error) {
	switch s.Type {
	case "string":
		err = unix.SetsockoptString(fd, s.Level, s.Opt, s.Value.(string))
	case "int":
		err = unix.SetsockoptInt(fd, s.Level, s.Opt, s.Value.(int))
	case "byte":
		err = unix.SetsockoptByte(fd, s.Level, s.Opt, byte(s.Value.(int)))
	default:
		err = fmt.Errorf("unknown Sockopt Type: '%s'", s.Type)
	}
	if err != nil {
		err = fmt.Errorf(
			"error setting sockopt %s (level=%d, opt=%d) to '%v': %w",
			s.Name, s.Level, s.Opt, s.Value, err)
	}
	return
}
