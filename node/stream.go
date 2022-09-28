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

// StreamServer is a server used for TCP stream tests.
type StreamServer struct {
	// ListenAddr is the TCP listen address, as specified to the address
	// parameter in net.Listen (e.g. ":port" or "addr:port").
	ListenAddr string

	// ListenAddrKey is the key used in the returned Feedback for the listen
	// address, obtained using Listen.Addr.String(). If empty, the listen
	// address will not be included in the Feedback.
	ListenAddrKey string

	errc chan error
}

// Run implements runner
func (s *StreamServer) Run(ctx context.Context, arg runArg) (ofb Feedback,
	err error) {
	//c := net.ListenConfig{Control: s.tcpControl}
	c := net.ListenConfig{}
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
func (s *StreamServer) Cancel(rec *recorder) error {
	return <-s.errc
}

// start starts the main and accept goroutines.
func (s *StreamServer) start(ctx context.Context, lst net.Listener,
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
func (s *StreamServer) serve(ctx context.Context, conn *net.TCPConn,
	rec *recorder, errc chan error) {
	var e error
	defer func() {
		conn.Close()
		if e != nil {
			errc <- e
		}
		errc <- errDone
	}()
	var m Stream
	d := gob.NewDecoder(conn)
	if e = d.Decode(&m); e != nil {
		return
	}
	if m.CCA != "" {
		if e = setSockoptString(conn, unix.IPPROTO_TCP, unix.TCP_CONGESTION,
			m.CCA); e != nil {
			return
		}
	}
	switch m.Direction {
	case Download:
		e = m.send(ctx, conn, rec)
	case Upload:
		e = m.receive(conn, rec)
	default:
		e = fmt.Errorf("unknown Direction: %s", m.Direction)
	}
}

// StreamClient is a client used for TCP stream tests.
type StreamClient struct {
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
func (s *StreamClient) Run(ctx context.Context, arg runArg) (ofb Feedback,
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
	arg.rec.Send(s.Stream)
	e := gob.NewEncoder(c)
	if err = e.Encode(s.Stream); err != nil {
		return
	}
	switch s.Direction {
	case Download:
		err = s.receive(c, arg.rec)
	case Upload:
		err = s.send(ctx, c, arg.rec)
	default:
		err = fmt.Errorf("unknown Direction: %s", s.Direction)
	}
	return
}

// addr returns the dial address, from either Addr or AddrKey.
func (s *StreamClient) addr(ifb Feedback) (a string, err error) {
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

// Stream contains the parameters for a stream.
type Stream struct {
	// Flow is the flow identifier for the Stream.
	Flow Flow

	// Direction indicates in which direction the data flows.
	Direction Direction

	// Duration is the length of time the sender writes.
	Duration metric.Duration

	// CCA sets the sender's Congestion Control Algorithm.
	CCA string

	// SampleIOInterval is the minimum time between IO samples. Zero means a
	// sample will be recorded for every read and write.
	SampleIOInterval metric.Duration

	// BufLen is the size of the buffer used to read and write from the conn.
	BufLen int
}

// init registers Stream with the gob encoder
func init() {
	gob.Register(Stream{})
}

// tcpControl provides ListenConfig.Control and Dialer.Control for TCP.
func (s Stream) tcpControl(network, address string, conn syscall.RawConn) (
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
func (s Stream) send(ctx context.Context, w io.Writer, rec *recorder) (
	err error) {
	b := make([]byte, s.BufLen)
	for i := 0; i < s.BufLen; i++ {
		b[i] = 0xfe
	}
	in, dur := s.SampleIOInterval.Duration(), s.Duration.Duration()
	t0 := time.Now()
	rec.Send(SentMark{s.Flow, t0})
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
		if n > 0 {
			ds := t.Sub(ts)
			if ds > in || done {
				rec.Send(Sent{s.Flow, dt, l})
				ts = t
			}
		}
	}
	return
}

// receive runs the receive side of a stream.
func (s Stream) receive(r io.Reader, rec *recorder) (err error) {
	b := make([]byte, s.BufLen)
	in := s.SampleIOInterval.Duration()
	t0 := time.Now()
	rec.Send(ReceivedMark{s.Flow, t0})
	ts := t0
	var l metric.Bytes
	for {
		var n int
		n, err = r.Read(b)
		t := time.Now()
		dt := t.Sub(t0)
		l += metric.Bytes(n)
		if n > 0 {
			ds := t.Sub(ts)
			if ds > in || err != nil {
				rec.Send(Received{s.Flow, dt, l})
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

// flags implements message
func (Stream) flags() flag {
	return flagForward
}

// handle implements event
func (s Stream) handle(node *node) {
	node.parent.Send(s)
}

func (s Stream) String() string {
	return fmt.Sprintf("Stream[Flow:%s Direction:%s]", s.Flow, s.Direction)
}

// Direction indicates a sense for the flow of data, e.g. Upload for client to
// server, and Download for server to client.
type Direction string

const (
	Upload   Direction = "upload"
	Download           = "download"
)

// SentMark represents the base time that sending began for a flow.
type SentMark struct {
	Flow Flow      // Flow that this SentMark is for
	T0   time.Time // base time that sending began
}

// init registers SentMark with the gob encoder
func init() {
	gob.Register(SentMark{})
}

// flags implements message
func (SentMark) flags() flag {
	return flagForward
}

// handle implements event
func (s SentMark) handle(node *node) {
	node.parent.Send(s)
}

func (s SentMark) String() string {
	return fmt.Sprintf("SentMark[Flow:%s T0:%s]", s.Flow, s.T0)
}

// Sent is a time series data point containing a total number of sent bytes.
type Sent struct {
	Flow  Flow          // Flow that this Sent is for
	T     time.Duration // duration since sending began (SentMark.T0)
	Total metric.Bytes  // total sent bytes
}

// init registers Sent with the gob encoder
func init() {
	gob.Register(Sent{})
}

// flags implements message
func (Sent) flags() flag {
	return flagForward
}

// handle implements event
func (s Sent) handle(node *node) {
	node.parent.Send(s)
}

func (s Sent) String() string {
	return fmt.Sprintf("Sent[Flow:%s T:%s Total:%d]", s.Flow, s.T, s.Total)
}

// ReceivedMark represents the base time that receiving began for a flow.
type ReceivedMark struct {
	Flow Flow      // Flow that this ReceivedMark is for
	T0   time.Time // base time that receiving began
}

// init registers ReceivedMark with the gob encoder
func init() {
	gob.Register(ReceivedMark{})
}

// flags implements message
func (ReceivedMark) flags() flag {
	return flagForward
}

// handle implements event
func (s ReceivedMark) handle(node *node) {
	node.parent.Send(s)
}

func (s ReceivedMark) String() string {
	return fmt.Sprintf("ReceivedMark[Flow:%s T0:%s]", s.Flow, s.T0)
}

// Received is a time series data point containing a total number of received
// bytes.
type Received struct {
	Flow  Flow          // flow that this Received is for
	T     time.Duration // duration since sending began (ReceivedMark.T0)
	Total metric.Bytes  // total received bytes
}

// init registers Received with the gob encoder
func init() {
	gob.Register(Received{})
}

// flags implements message
func (Received) flags() flag {
	return flagForward
}

// handle implements event
func (r Received) handle(node *node) {
	node.parent.Send(r)
}

func (r Received) String() string {
	return fmt.Sprintf("Received[Flow:%s T:%s Total:%d]", r.Flow, r.T, r.Total)
}
