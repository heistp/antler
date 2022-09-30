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

// StreamServer is a server used for stream oriented protocols.
type StreamServer struct {
	// ListenAddr is the listen address, as specified to the address parameter
	// in net.Listen (e.g. ":port" or "addr:port").
	ListenAddr string

	// ListenAddrKey is the key used in the returned Feedback for the listen
	// address, obtained using Listen.Addr.String(). If empty, the listen
	// address will not be included in the Feedback.
	ListenAddrKey string

	// Protocol is the protocol to use (tcp, tcp4 or tcp6).
	Protocol string

	errc chan error
}

// Run implements runner
func (s *StreamServer) Run(ctx context.Context, arg runArg) (ofb Feedback,
	err error) {
	c := net.ListenConfig{}
	var l net.Listener
	if l, err = c.Listen(ctx, s.Protocol, s.ListenAddr); err != nil {
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
	var m streamer
	d := gob.NewDecoder(conn)
	if e = d.Decode(&m); e != nil {
		return
	}
	e = m.handleServer(ctx, conn, rec)
}

// StreamClient is a client used for stream oriented protocols.
type StreamClient struct {
	// Addr is the dial address, as specified to the address parameter in
	// net.Dial (e.g. "addr:port").
	Addr string

	// AddrKey is a key used to obtain the dial address from the incoming
	// Feedback, if Addr is not specified.
	AddrKey string

	// Protocol is the protocol to use (tcp, tcp4 or tcp6).
	Protocol string

	Streamers
}

// Run implements runner
func (s *StreamClient) Run(ctx context.Context, arg runArg) (ofb Feedback,
	err error) {
	var a string
	if a, err = s.addr(arg.ifb); err != nil {
		return
	}
	m := s.streamer()
	d := net.Dialer{}
	if r, ok := m.(dialController); ok {
		d.Control = r.dialControl
	}
	var c net.Conn
	if c, err = d.DialContext(ctx, s.Protocol, a); err != nil {
		return
	}
	defer c.Close()
	e := gob.NewEncoder(c)
	if err = e.Encode(&m); err != nil {
		return
	}
	err = m.handleClient(ctx, c, arg.rec)
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

// A streamer handles connections in StreamClient and StreamServer.
type streamer interface {
	// handleClient handles a client connection.
	handleClient(context.Context, net.Conn, *recorder) error

	// handleServer handles a server connection.
	handleServer(context.Context, net.Conn, *recorder) error
}

// A dialController provides Dialer.Control for the StreamClient, and may be
// implemented by a streamer.
type dialController interface {
	dialControl(network, address string, c syscall.RawConn) error
}

// Streamers is the union of available streamer implementations.
type Streamers struct {
	Upload   *Upload
	Download *Download
}

// streamer returns the only non-nil streamer implementation.
func (s *Streamers) streamer() streamer {
	switch {
	case s.Upload != nil:
		return s.Upload
	case s.Download != nil:
		return s.Download
	default:
		panic("no streamer set in streamers union")
	}
}

// Upload is a stream transfer from client to server.
type Upload struct {
	Transfer
}

// init registers Upload with the gob encoder
func init() {
	gob.Register(Upload{})
}

// handleClient implements streamer
func (u Upload) handleClient(ctx context.Context, conn net.Conn,
	rec *recorder) error {
	rec.Send(u.Transfer.Stream)
	return u.send(ctx, conn, rec)
}

// handleServer implements streamer
func (u Upload) handleServer(ctx context.Context, conn net.Conn,
	rec *recorder) error {
	return u.receive(ctx, conn, rec)
}

func (u Upload) String() string {
	return fmt.Sprintf("Upload[Flow:%s]", u.Flow)
}

// Download is a stream transfer from server to client.
type Download struct {
	Transfer
}

// init registers Upload with the gob encoder
func init() {
	gob.Register(Download{})
}

// handleClient implements streamer
func (d Download) handleClient(ctx context.Context, conn net.Conn,
	rec *recorder) error {
	rec.Send(d.Transfer.Stream)
	return d.receive(ctx, conn, rec)
}

// handleServer implements streamer
func (d Download) handleServer(ctx context.Context, conn net.Conn,
	rec *recorder) (err error) {
	if d.CCA != "" {
		if t, ok := conn.(*net.TCPConn); ok {
			if err = setTCPSockoptString(t, unix.IPPROTO_TCP,
				unix.TCP_CONGESTION, "CCA", d.CCA); err != nil {
				return
			}
		}
	}
	err = d.send(ctx, conn, rec)
	return
}

// flags implements message
func (Download) flags() flag {
	return flagForward
}

// handle implements event
func (d Download) handle(node *node) {
	node.parent.Send(d)
}

func (d Download) String() string {
	return fmt.Sprintf("Download[Flow:%s]", d.Flow)
}

// Stream represents the information for one direction of a flow for a stream
// oriented connection.
type Stream struct {
	// Flow is the Stream's flow identifier.
	Flow Flow

	// Direction is the client to server sense.
	Direction Direction

	// CCA is the sender's Congestion Control Algorithm.
	CCA string
}

// init registers SentMark with the gob encoder
func init() {
	gob.Register(Stream{})
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

// Direction is the client to server sense for a Stream.
type Direction string

const (
	Up   Direction = "up"   // client to server
	Down Direction = "down" // server to client
)

// Transfer contains the parameters for an Upload or Download.
type Transfer struct {
	// Duration is the length of time the sender writes.
	Duration metric.Duration

	// SampleIOInterval is the minimum time between IO samples. Zero means a
	// sample will be recorded for every read and write.
	SampleIOInterval metric.Duration

	// BufLen is the size of the buffer used to read and write from the conn.
	BufLen int

	Stream
}

// dialControl implements dialController
func (x Transfer) dialControl(network, address string, conn syscall.RawConn) (
	err error) {
	c := func(fd uintptr) {
		if x.CCA != "" {
			err = setSockoptString(int(fd), unix.IPPROTO_TCP,
				unix.TCP_CONGESTION, "CCA", x.CCA)
		}
	}
	if e := conn.Control(c); e != nil && err == nil {
		err = e
	}
	return
}

// send runs the send side of a transfer.
func (x Transfer) send(ctx context.Context, w io.Writer, rec *recorder) (
	err error) {
	b := make([]byte, x.BufLen)
	for i := 0; i < x.BufLen; i++ {
		b[i] = 0xfe
	}
	in, dur := x.SampleIOInterval.Duration(), x.Duration.Duration()
	t0 := time.Now()
	rec.Send(SentMark{x.Flow, t0})
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
				rec.Send(Sent{x.Flow, dt, l})
				ts = t
			}
		}
	}
	return
}

// receive runs the receive side of a transfer.
func (x Transfer) receive(ctx context.Context, r io.Reader, rec *recorder) (
	err error) {
	b := make([]byte, x.BufLen)
	in := x.SampleIOInterval.Duration()
	t0 := time.Now()
	rec.Send(ReceivedMark{x.Flow, t0})
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
				rec.Send(Received{x.Flow, dt, l})
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
