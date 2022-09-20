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

// Cancel implements canceler
func (s *TCPStreamServer) Cancel(rec *recorder) error {
	return <-s.errc
}

// run is the entry point for the server goroutine.
func (s *TCPStreamServer) run(ctx context.Context, lst net.Listener,
	rec *recorder) {
	s.errc = make(chan error)
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
		if e != nil {
			errc <- e
		}
		errc <- errDone
	}()
	if s.Download {
		b := make([]byte, 0, s.WriteBufLen)
		for i := 0; i < s.WriteBufLen; i++ {
			b[i] = 0xfe
		}
		in, dur := s.Interval.Duration(), s.Duration.Duration()
		t0 := time.Now()
		rec.Send(TCPStreamInfo{t0, s.TCPStream})
		ts := t0
		var done bool
		for !done {
			var n int
			var l uint64
			n, e = conn.Write(b)
			t := time.Now()
			dt, ds := t.Sub(t0), t.Sub(ts)
			l += uint64(n)
			done = dt > dur
			if n > 0 && (e != nil || ds > in || done) {
				rec.Send(TCPByteTotal{s.Series, dt, l})
			}
			if e != nil {
				if e == io.EOF {
					e = nil
				}
				break
			}
		}
	} else {
		e = fmt.Errorf("upload not supported")
		return
	}
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
