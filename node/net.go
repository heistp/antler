// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"fmt"
	"net"
	"os"
	"syscall"

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

// Sockopts contains the socket option fields used by streams and packets.
type Sockopts struct {
	// Sockopt lists the generic socket options to set.
	Sockopt []Sockopt

	// DS is the value to set for the DS (ToS/Traffic Class) byte.
	DS int

	// CCA is the sender's Congestion Control Algorithm (TCP only).
	CCA string
}

// sockopt returns a list of both the fixed field and generic socket options.
func (s Sockopts) sockopt() (opt []Sockopt) {
	if s.CCA != "" {
		opt = append(opt, Sockopt{"string", unix.IPPROTO_TCP,
			unix.TCP_CONGESTION, "CCA", s.CCA})
	}
	if s.DS != 0 {
		opt = append(opt, Sockopt{"int", unix.IPPROTO_IP,
			unix.IP_TOS, "ToS", s.DS})
	}
	opt = append(opt, s.Sockopt...)
	return
}

// dialControl is the Dialer.Control function and dialController implementation.
func (s Sockopts) dialControl(network, address string,
	conn syscall.RawConn) (err error) {
	c := func(fd uintptr) {
		for _, o := range s.sockopt() {
			if err = o.set(int(fd)); err != nil {
				return
			}
		}
	}
	if e := conn.Control(c); e != nil && err == nil {
		err = e
	}
	return
}
