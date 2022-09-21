// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
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

	// TCPStream embeds the TCP stream parameters.
	TCPStream

	errc chan error
}

// Run implements runner
func (s *TCPStreamServer) Run(ctx context.Context, arg runArg) (ofb Feedback,
	err error) {
	c := net.ListenConfig{Control: s.control}
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
				go s.serve(t, rec, ec)
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
func (s *TCPStreamServer) serve(conn *net.TCPConn, rec *recorder,
	errc chan error) {
	var e error
	defer func() {
		conn.Close()
		if e != nil {
			errc <- e
		}
		errc <- errDone
	}()
	if s.Download {
		b := make([]byte, s.WriteBufLen)
		for i := 0; i < s.WriteBufLen; i++ {
			b[i] = 0xfe
		}
		in, dur := s.Interval.Duration(), s.Duration.Duration()
		t0 := time.Now()
		rec.Send(TCPStreamInfo{t0, s.TCPStream})
		ts := t0
		var l uint64
		var done bool
		for !done {
			var n int
			n, e = conn.Write(b)
			t := time.Now()
			dt, ds := t.Sub(t0), t.Sub(ts)
			l += uint64(n)
			done = dt > dur || e != nil
			if n > 0 && (ds > in || done) {
				rec.Send(TCPByteTotal{s.Series, dt, l})
				ts = t
			}
		}
	} else {
		e = fmt.Errorf("upload not supported")
	}
}

// TCPByteTotal is a time series data point containing a total number of bytes
// sent or received by the TCPStream runner.
type TCPByteTotal struct {
	Series Series        // series the ByteCount belongs to
	Time   time.Duration // duration since stream began (T0 in TCPStreamInfo)
	Total  uint64        // total byte count sent or received
}

// init registers TCPByteTotal with the gob encoder
func init() {
	gob.Register(TCPByteTotal{})
}

// flags implements message
func (TCPByteTotal) flags() flag {
	return flagForward
}

// handle implements event
func (i TCPByteTotal) handle(node *node) {
	node.parent.Send(i)
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
	TCPStream
}

// Run implements runner
func (s *TCPStreamClient) Run(ctx context.Context, arg runArg) (ofb Feedback,
	err error) {
	var a string
	if a, err = s.addr(arg.ifb); err != nil {
		return
	}
	d := net.Dialer{Control: s.control}
	var c net.Conn
	if c, err = d.Dial("tcp", a); err != nil {
		return
	}
	defer c.Close()
	if s.Download {
		b := make([]byte, s.ReadBufLen)
		in := s.Interval.Duration()
		t0 := time.Now()
		arg.rec.Send(TCPStreamInfo{t0, s.TCPStream})
		ts := t0
		var l uint64
		for {
			var n int
			n, err = c.Read(b)
			t := time.Now()
			dt, ds := t.Sub(t0), t.Sub(ts)
			l += uint64(n)
			if n > 0 && (ds > in || err != nil) {
				arg.rec.Send(TCPByteTotal{s.Series, dt, l})
				ts = t
			}
			if err != nil {
				if err == io.EOF {
					err = nil
				}
				break
			}
		}
	} else {
		err = fmt.Errorf("upload not supported")
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

// TCPStreamInfo is a data point at the beginning of the TCP stream containing
// meta-information about the stream.
type TCPStreamInfo struct {
	T0        time.Time // T0 is the stream start time
	TCPStream           // TCPStream contains the stream parameters
}

// init registers TCPStreamInfo with the gob encoder
func init() {
	gob.Register(TCPStreamInfo{})
}

// flags implements message
func (TCPStreamInfo) flags() flag {
	return flagForward
}

// handle implements event
func (i TCPStreamInfo) handle(node *node) {
	node.parent.Send(i)
}

// TCPStream contains the parameters for a TCP stream, used in the client,
// server and TCPStreamInfo.
type TCPStream struct {
	// Series is the series name.
	Series Series

	// Download indicates whether to run the test from server to client (true)
	// or client to server (false).
	Download bool

	// CCA sets the Congestion Control Algorithm used for the stream.
	CCA string

	// Duration is the length of time the stream runs.
	Duration Duration

	// Interval is the minimum time between ByteCount samples. If Interval is 0,
	// a ByteCount sample will be returned for every read and write.
	Interval Duration

	// ReadBufLen is the size of the buffer used to read from the conn.
	ReadBufLen int

	// WriteBufLen is the size of the buffer used to write to the conn.
	WriteBufLen int
}

// control provides ListenConfig.Control and Dialer.Control.
func (s *TCPStream) control(network, address string, conn syscall.RawConn) (
	err error) {
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
