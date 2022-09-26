// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"context"
	"fmt"
	"net"
)

// TCPStreamServer is a server used for TCP stream tests.
type TCPStreamServer struct {
	// ListenAddr is the TCP listen address, as specified to the address
	// parameter in net.Listen (e.g. ":port" or "addr:port").
	ListenAddr string

	// ListenAddrKey is the key used in the returned Feedback for the listen
	// address, obtained using Listen.Addr.String(). If empty, the listen
	// address will not be included in the Feedback.
	ListenAddrKey string

	// Stream embeds the TCP stream parameters.
	Stream

	errc chan error
}

// Run implements runner
func (s *TCPStreamServer) Run(ctx context.Context, arg runArg) (ofb Feedback,
	err error) {
	c := net.ListenConfig{Control: s.tcpControl}
	var l net.Listener
	if l, err = c.Listen(ctx, "tcp", s.ListenAddr); err != nil {
		return
	}
	if s.ListenAddrKey != "" {
		ofb[s.ListenAddrKey] = l.Addr().String()
	}
	s.errc = make(chan error)
	s.start(ctx, l, arg.rec)
	arg.cxl <- s
	return
}

// Cancel implements canceler
func (s *TCPStreamServer) Cancel(rec *recorder) error {
	return <-s.errc
}

// start starts the main and accept goroutines.
func (s *TCPStreamServer) start(ctx context.Context, lst net.Listener,
	rec *recorder) {
	ec := make(chan error)
	cc := make(chan net.Conn)
	// accept goroutine
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
			cc <- c
		}
	}()
	// main goroutine
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
				t := c.(*net.TCPConn)
				g++
				go s.serve(ctx, t, rec, ec)
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
func (s *TCPStreamServer) serve(ctx context.Context, conn *net.TCPConn,
	rec *recorder, errc chan error) {
	var e error
	defer func() {
		conn.Close()
		if e != nil {
			errc <- e
		}
		errc <- errDone
	}()
	if s.Download {
		e = s.send(ctx, conn, rec)
	} else {
		e = s.receive(conn, rec)
	}
}

// TCPStreamClient is a client used for TCP stream tests.
type TCPStreamClient struct {
	// Addr is the TCP dial address, as specified to the address parameter in
	// net.Dial (e.g. "addr:port").
	Addr string

	// AddrKey is a key used to obtain the dial address from the incoming
	// Feedback, if Addr is not specified.
	AddrKey string

	// TCPStream embeds the TCP stream parameters.
	Stream
}

// Run implements runner
func (s *TCPStreamClient) Run(ctx context.Context, arg runArg) (ofb Feedback,
	err error) {
	var a string
	if a, err = s.addr(arg.ifb); err != nil {
		return
	}
	d := net.Dialer{Control: s.tcpControl}
	var c net.Conn
	if c, err = d.DialContext(ctx, "tcp", a); err != nil {
		return
	}
	defer c.Close()
	if s.Download {
		err = s.receive(c, arg.rec)
	} else {
		err = s.send(ctx, c, arg.rec)
	}
	return
}

// addr returns the dial address, from either Addr or AddrKey.
func (s *TCPStreamClient) addr(ifb Feedback) (a string, err error) {
	if a = s.Addr; a != "" {
		return
	}
	if v, ok := ifb[s.AddrKey]; ok {
		a = v.(string)
	} else {
		err = fmt.Errorf("no address specified in Addr or AddrKey")
	}
	return
}
