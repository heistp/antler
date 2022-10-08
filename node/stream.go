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
	var r streamRequest
	d := gob.NewDecoder(conn)
	if e = d.Decode(&r); e != nil {
		return
	}
	e = r.Streamer.handleServer(ctx, conn, r.Flow, r.End, rec)
}

// streamRequest is sent from StreamClient to StreamServer to communicate the
// parameters needed to serve the stream.
type streamRequest struct {
	Streamer streamer
	Flow     Flow
	End      StreamEnd
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

	// Flow is the base flow identifier.
	Flow Flow

	// StreamStart controls stream introductions.
	Start StreamStart

	// StreamEnd controls stream durations / lengths.
	End StreamEnd

	Streamers
}

// Run implements runner
func (s *StreamClient) Run(ctx context.Context, arg runArg) (ofb Feedback,
	err error) {
	dd := s.Start.durationer()
	ec := make(chan error)
	var g int
	defer func() {
		for g > 0 {
			if e := <-ec; e != nil && err == nil {
				err = e
			}
			g--
		}
	}()
	n := 1
	t := time.After(0)
	t0 := time.Now()
	for {
		select {
		case <-t:
			g++
			go s.run(ctx, arg, s.flow(n), ec)
			if n >= s.Start.Streams ||
				time.Since(t0) > s.Start.Duration.Duration() {
				return
			}
			n++
			t = time.After(dd.duration())
		case err = <-ec:
			g--
			if err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
	return
}

// flow returns the flow identifier for the nth flow.
func (s *StreamClient) flow(n int) Flow {
	if s.Start.Streams <= 1 {
		return s.Flow
	}
	return Flow(fmt.Sprintf("%s.%d", s.Flow, n))
}

// run runs one client.
func (s *StreamClient) run(ctx context.Context, arg runArg, flow Flow,
	errc chan error) {
	var e error
	defer func() {
		errc <- e
	}()
	var a string
	if a, e = s.addr(arg.ifb); e != nil {
		return
	}
	m := s.streamer()
	l := net.Dialer{}
	if r, ok := m.(dialController); ok {
		l.Control = r.dialControl
	}
	var c net.Conn
	if c, e = l.DialContext(ctx, s.Protocol, a); e != nil {
		return
	}
	defer c.Close()
	en := gob.NewEncoder(c)
	r := streamRequest{m, flow, s.End}
	if e = en.Encode(r); e != nil {
		return
	}
	e = m.handleClient(ctx, c, flow, s.End, arg.rec)
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
	handleClient(context.Context, net.Conn, Flow, StreamEnd, *recorder) error

	// handleServer handles a server connection.
	handleServer(context.Context, net.Conn, Flow, StreamEnd, *recorder) error
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
	}
	return nil
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
func (u Upload) handleClient(ctx context.Context, conn net.Conn, flow Flow,
	end StreamEnd, rec *recorder) error {
	rec.Send(u.Info(flow, false))
	return u.send(ctx, conn, flow, end, rec)
}

// handleServer implements streamer
func (u Upload) handleServer(ctx context.Context, conn net.Conn, flow Flow,
	end StreamEnd, rec *recorder) error {
	rec.Send(u.Info(flow, true))
	return u.receive(ctx, conn, flow, rec)
}

// Download is a stream transfer from server to client.
type Download struct {
	Transfer
}

// init registers Download with the gob encoder
func init() {
	gob.Register(Download{})
}

// handleClient implements streamer
func (d Download) handleClient(ctx context.Context, conn net.Conn, flow Flow,
	end StreamEnd, rec *recorder) error {
	rec.Send(d.Info(flow, false))
	return d.receive(ctx, conn, flow, rec)
}

// handleServer implements streamer
func (d Download) handleServer(ctx context.Context, conn net.Conn, flow Flow,
	end StreamEnd, rec *recorder) (err error) {
	if d.CCA != "" {
		if t, ok := conn.(*net.TCPConn); ok {
			if err = setTCPSockoptString(t, unix.IPPROTO_TCP,
				unix.TCP_CONGESTION, "CCA", d.CCA); err != nil {
				return
			}
		}
	}
	rec.Send(d.Info(flow, true))
	err = d.send(ctx, conn, flow, end, rec)
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

// Stream represents one direction of a stream oriented flow.
type Stream struct {
	// Direction is the client to server sense.
	Direction Direction

	// CCA is the sender's Congestion Control Algorithm.
	CCA string
}

// Info returns StreamInfo for this Stream.
func (s Stream) Info(flow Flow, server bool) StreamInfo {
	return StreamInfo{metric.Tinit, flow, s, server}
}

func (s Stream) String() string {
	return fmt.Sprintf("Stream[Direction:%s CCA:%s]", s.Direction, s.CCA)
}

// StreamInfo contains information for a stream flow.
type StreamInfo struct {
	// Tinit is the base time for the flow's RelativeTime values.
	Tinit time.Time

	// Flow is the flow identifier.
	Flow Flow

	Stream

	// Server indicates if this is from the server (true) or client (false).
	Server bool
}

// init registers StreamInfo with the gob encoder
func init() {
	gob.Register(StreamInfo{})
}

// Time returns an absolute from a node-relative time.
func (s StreamInfo) Time(r metric.RelativeTime) time.Time {
	return s.Tinit.Add(time.Duration(r))
}

// flags implements message
func (StreamInfo) flags() flag {
	return flagForward
}

// handle implements event
func (s StreamInfo) handle(node *node) {
	node.parent.Send(s)
}

func (s StreamInfo) String() string {
	return fmt.Sprintf("StreamInfo[Tinit:%s Stream:%s]", s.Tinit, s.Stream)
}

// Direction is the client to server sense for a Stream.
type Direction string

const (
	Up   Direction = "up"   // client to server
	Down Direction = "down" // server to client
)

// Transfer contains the parameters for an Upload or Download.
type Transfer struct {
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
func (x Transfer) send(ctx context.Context, w io.Writer, flow Flow,
	end StreamEnd, rec *recorder) (err error) {
	b := make([]byte, x.BufLen)
	for i := 0; i < x.BufLen; i++ {
		b[i] = 0xfe
	}
	in := x.SampleIOInterval.Duration()
	dur := end.Duration.durationer().duration()
	bytes := end.Bytes.byteser().bytes()
	t0 := metric.Now()
	rec.Send(StreamIO{flow, t0, 0, true})
	ts := t0
	var l metric.Bytes
	var done bool
	for !done {
		var n int
		n, err = w.Write(b)
		t := metric.Now()
		l += metric.Bytes(n)
		select {
		case <-ctx.Done():
			done = true
		default:
			// TODO clean up duration/bytes limit from StreamEnd
			done = time.Duration(t-t0) > dur || l > bytes || err != nil
		}
		if n > 0 {
			if time.Duration(t-ts) > in || done {
				rec.Send(StreamIO{flow, t, l, true})
				ts = t
			}
		}
	}
	return
}

// receive runs the receive side of a transfer.
func (x Transfer) receive(ctx context.Context, r io.Reader, flow Flow,
	rec *recorder) (err error) {
	b := make([]byte, x.BufLen)
	in := x.SampleIOInterval.Duration()
	t0 := metric.Now()
	rec.Send(StreamIO{flow, t0, 0, false})
	ts := t0
	var l metric.Bytes
	for {
		var n int
		n, err = r.Read(b)
		t := metric.Now()
		l += metric.Bytes(n)
		if n > 0 {
			if time.Duration(t-ts) > in || err != nil {
				rec.Send(StreamIO{flow, t, l, false})
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

// StreamIO is a time series data point that records the progress of a stream as
// measured after read or write calls.
type StreamIO struct {
	// Flow is the flow that this StreamIO is for.
	Flow Flow

	// T is the relative time this StreamIO was recorded.
	T metric.RelativeTime

	// Total is the total number of sent or received bytes.
	Total metric.Bytes

	// Sent is true for sent bytes, and false for received.
	Sent bool
}

// init registers StreamIO with the gob encoder
func init() {
	gob.Register(StreamIO{})
}

// flags implements message
func (StreamIO) flags() flag {
	return flagForward
}

// handle implements event
func (s StreamIO) handle(node *node) {
	node.parent.Send(s)
}

func (s StreamIO) String() string {
	return fmt.Sprintf("StreamIO[Flow:%s T:%s Total:%d Sent:%t]",
		s.Flow, s.T, s.Total, s.Sent)
}

// StreamStart contains the parameters for stream introductions.
type StreamStart struct {
	Durationers

	// Duration is the maximum length of time to start streams for.
	Duration metric.Duration

	// Streams is the maximum number of streams to introduce.
	Streams int
}

// StreamEnd contains the parameters for stream durations / lengths.
type StreamEnd struct {
	// Duration selects a durationer from which to get stream Durations.
	Duration Durationers

	// Bytes selects a byteser from which to get a number of bytes.
	Bytes Bytesers
}

// A durationer can return Durations.
type durationer interface {
	duration() time.Duration
}

// Durationers is a union of the available durationer implementations.
type Durationers struct {
	Fixed       *FixedDuration
	Isochronous *FixedDuration
	Exponential *ExpDuration
}

// durationer returns the only non-nil durationer implementation.
func (d *Durationers) durationer() durationer {
	switch {
	case d.Fixed != nil:
		return d.Fixed
	case d.Isochronous != nil:
		return d.Isochronous
	case d.Exponential != nil:
		return d.Exponential
	}
	return nil
}

// FixedDuration is a durationer that returns a constant Duration.
type FixedDuration metric.Duration

// duration implements durationer
func (d FixedDuration) duration() time.Duration {
	return time.Duration(d)
}

// ExpDuration is a durationer that returns Durations on an exponential
// distribution.
type ExpDuration struct {
	// Mean is the mean duration.
	Mean metric.Duration

	// Rate is the rate parameter for the exponential distribution.
	Rate float64
}

// duration implements durationer
func (d ExpDuration) duration() time.Duration {
	// TODO implement ExpDuration
	return time.Duration(d.Mean)
}

// Bytesers is a union of the available byteser implementations.
type Bytesers struct {
	Fixed *FixedBytes
}

// byteser returns the only non-nil byteser implementation.
func (b *Bytesers) byteser() byteser {
	switch {
	case b.Fixed != nil:
		return b.Fixed
	}
	return nil
}

// A byteser can return metric.Bytes values.
type byteser interface {
	bytes() metric.Bytes
}

// FixedBytes is a byteser that returns a constant metric.Bytes.
type FixedBytes metric.Bytes

// bytes implements byteser
func (b FixedBytes) bytes() metric.Bytes {
	return metric.Bytes(b)
}

// LognormalBytes returns metric.Bytes values on a lognormal distribution.
type LognormalBytes struct {
	// P5 is the 5th percentile of the lognormal distribution.
	P5 metric.Bytes

	// Mean is the mean metric.Bytes value.
	Mean metric.Bytes

	// P95 is the 95th percentile of the lognormal distribution.
	P95 metric.Bytes
}

// bytes implements byteser
func (l LognormalBytes) bytes() metric.Bytes {
	// TODO implement LognormalBytes
	return l.Mean
}
