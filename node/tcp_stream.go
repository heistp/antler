// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"context"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

// TCPStreamServer is a server used for TCP stream tests.
type TCPStreamServer struct {
	// AddrKey is the key used in the returned Feedback for the listen address,
	// obtained using Listen.Addr.String(). If empty, the listen address will
	// not be included in the Feedback.
	AddrKey string

	// ListenAddr is the TCP listen address, as specified to the address
	// parameter in net.Listen (e.g. ":port" or "addr:port").
	ListenAddr string

	// TCPStream embeds the TCP stream parameters.
	TCPStream

	errc chan error
}

// Run implements runner
func (s *TCPStreamServer) Run(ctx context.Context, arg runArg) (ofb Feedback,
	err error) {
	c := net.ListenConfig{Control: s.listenControl}
	var l net.Listener
	if l, err = c.Listen(ctx, "tcp", s.ListenAddr); err != nil {
		return
	}
	if s.AddrKey != "" {
		ofb[s.AddrKey] = l.Addr().String()
	}
	s.run(ctx, l, arg.rec)
	arg.cxl <- s
	return
}

// listenControl is the ListenConfig.Control func for the server.
func (s *TCPStreamServer) listenControl(network, address string,
	conn syscall.RawConn) (err error) {
	c := func(fd uintptr) {
		if s.CCA != "" {
			err = unix.SetsockoptString(int(fd), unix.IPPROTO_TCP,
				unix.TCP_CONGESTION, s.CCA)
		}
	}
	if e := conn.Control(c); e != nil && err == nil {
		err = e
	}
	return
}

// Control implements
func (s *TCPStreamServer) Control(fd uintptr) {
}

// Cancel implements canceler
func (s *TCPStreamServer) Cancel(rec *recorder) error {
	return <-s.errc
}

// run is the entry point for the server goroutine.
func (s *TCPStreamServer) run(ctx context.Context, lst net.Listener,
	rec *recorder) {
	s.errc = make(chan error)
	ec := make(chan error)
	cc := make(chan *net.TCPConn)
	go func() {
		for {
			var e error
			defer func() {
				if e != nil {
					ec <- e
				}
				ec <- errDone
			}()
			var c net.Conn
			if c, e = lst.Accept(); e != nil {
				return
			}
			cc <- c.(*net.TCPConn)
		}
	}()
	go func() {
		var err error
		defer func() {
			if err != nil {
				s.errc <- err
			}
			close(s.errc)
		}()
		d := ctx.Done()
		g := 1
		for g > 0 {
			select {
			case c := <-cc:
				g++
				go s.serve(c, ec)
			case <-d:
				d = nil
				err = lst.Close()
			case e := <-ec:
				if e == errDone {
					g--
					break
				}
				if d == nil {
					rec.Logf("post-cancel error: %s", e)
					break
				}
				rec.SendErrore(e)
			}
		}
	}()
}

// serve serves one connection.
func (s *TCPStreamServer) serve(conn *net.TCPConn, errc chan error) {
	defer func() {
		errc <- errDone
	}()
}

// TCPStream contains the parameters for a TCP stream, used on both the client
// and server.
type TCPStream struct {
	// Download indicates whether to run the test from server to client (true)
	// or client to server (false).
	Download bool

	// CCA sets the Congestion Control Algorithm used for the stream.
	CCA string

	// Duration is the length of time the stream runs.
	Duration Duration

	// ReadBufLen is the size of the buffer used to read from the conn.
	ReadBufLen int

	// WriteBufLen is the size of the buffer used to write to the conn.
	WriteBufLen int
}
