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

	"github.com/heistp/antler/node/metric"
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

// IOSample is a time series data point containing a total number of bytes
// sent or received by the TCPStream runner.
type IOSample struct {
	Series Series        // series the IOSample belongs to
	T      time.Duration // duration since stream began (T0 in TCPStreamInfo)
	Total  metric.Bytes  // total byte count sent or received
}

// init registers IOSample with the gob encoder
func init() {
	gob.Register(IOSample{})
}

// flags implements message
func (IOSample) flags() flag {
	return flagForward
}

// handle implements event
func (i IOSample) handle(node *node) {
	node.parent.Send(i)
}

func (i IOSample) String() string {
	return fmt.Sprintf("IOSample[Series:%s T:%s Total:%d]",
		i.Series, i.T, i.Total)
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

func (i TCPStreamInfo) String() string {
	return fmt.Sprintf("TCPStreamInfo[T0:%s Stream:%s]",
		i.T0, i.TCPStream.String())
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
	Duration metric.Duration

	// SampleIO, if true, sends IOSamples to record the progress of read and
	// write syscalls.
	SampleIO bool

	// SampleIOInterval is the minimum time between IOSamples. Zero means a
	// sample will be returned for every read and write.
	SampleIOInterval metric.Duration

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

// send runs the send side of a stream.
func (s *TCPStream) send(ctx context.Context, w io.Writer, rec *recorder) (
	err error) {
	b := make([]byte, s.WriteBufLen)
	for i := 0; i < s.WriteBufLen; i++ {
		b[i] = 0xfe
	}
	in, dur := s.SampleIOInterval.Duration(), s.Duration.Duration()
	t0 := time.Now()
	rec.Send(TCPStreamInfo{t0, *s})
	ts := t0
	var l metric.Bytes
	var done bool
	for !done {
		var n int
		n, err = w.Write(b)
		t := time.Now()
		dt := t.Sub(t0)
		l += metric.Bytes(n)
		select {
		case <-ctx.Done():
			done = true
		default:
			done = dt > dur || err != nil
		}
		if s.SampleIO && n > 0 {
			ds := t.Sub(ts)
			if ds > in || done {
				rec.Send(IOSample{s.Series, dt, l})
				ts = t
			}
		}
	}
	return
}

// receive runs the receive side of a stream.
func (s *TCPStream) receive(r io.Reader, rec *recorder) (err error) {
	b := make([]byte, s.ReadBufLen)
	in := s.SampleIOInterval.Duration()
	t0 := time.Now()
	rec.Send(TCPStreamInfo{t0, *s})
	ts := t0
	var l metric.Bytes
	for {
		var n int
		n, err = r.Read(b)
		t := time.Now()
		dt := t.Sub(t0)
		l += metric.Bytes(n)
		if s.SampleIO && n > 0 {
			ds := t.Sub(ts)
			if ds > in || err != nil {
				rec.Send(IOSample{s.Series, dt, l})
				ts = t
			}
		}
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}
	}
	return
}

func (s *TCPStream) String() string {
	return fmt.Sprintf("TCPStream[Series:%s Download:%t CCA:%s]",
		s.Series, s.Download, s.CCA)
}
