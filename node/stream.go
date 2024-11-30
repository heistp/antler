// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2022 Pete Heist

package node

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"hash"
	"io"
	"math"
	"net"
	"sync"
	"syscall"
	"time"

	"github.com/heistp/antler/node/metric"
)

// nonceLen is the length of nonce values for HMAC verification, in bytes.
const nonceLen = 32

// StreamServer is the server used for stream oriented protocols.
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

	// Key is a security key for HMAC verification.
	Key []byte

	nonce    map[string]struct{}
	nonceMtx sync.Mutex
	errc     chan error
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
	if len(s.Key) > 0 {
		s.nonce = make(map[string]struct{})
	}
	s.errc = make(chan error)
	s.start(ctx, l, arg)
	arg.cxl <- s
	return
}

// Cancel implements canceler
func (s *StreamServer) Cancel() error {
	return <-s.errc
}

// SetKey implements SetKeyer
func (s *StreamServer) SetKey(key []byte) {
	s.Key = key
}

// start starts the main and accept goroutines.
func (s *StreamServer) start(ctx context.Context, lst net.Listener,
	arg runArg) {
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
				if d == nil {
					c.Close()
					break
				}
				t := c.(*net.TCPConn)
				g++
				go s.serve(ctx, t, arg, ec)
			case <-d:
				d = nil
				err = lst.Close()
			case e := <-ec:
				if e == errDone {
					g--
					break
				}
				if d == nil {
					//rec.Logf("post-cancel error: %s", e)
					break
				}
				arg.rec.SendErrore(e)
			}
		}
	}()
}

// serve serves one connection.
func (s *StreamServer) serve(ctx context.Context, conn *net.TCPConn,
	arg runArg, errc chan error) {
	var e error
	defer func() {
		conn.Close()
		if e != nil {
			errc <- e
		}
		errc <- errDone
	}()
	var m streamer
	if m, e = s.header(conn); e != nil {
		return
	}
	e = m.handleServer(ctx, conn, arg)
}

// header reads the header and returns the streamer read from the header.
func (s *StreamServer) header(conn *net.TCPConn) (streamer streamer, err error) {
	var h hash.Hash
	var m, n []byte
	if len(s.Key) > 0 {
		h = hmac.New(sha256.New, s.Key)
		n = make([]byte, nonceLen)
		if _, err = io.ReadFull(conn, n); err != nil {
			return
		}
		if !s.validNonce(n) {
			err = fmt.Errorf("nonce replay:%x from:%s", n, conn.RemoteAddr())
			return
		}
		m = make([]byte, h.Size())
		if _, err = io.ReadFull(conn, m); err != nil {
			return
		}
	}
	var l uint16
	if err = binary.Read(conn, binary.LittleEndian, &l); err != nil {
		return
	}
	b := make([]byte, l)
	if _, err = io.ReadFull(conn, b); err != nil {
		return
	}
	if h != nil {
		h.Write(n)
		h.Write(b)
		x := h.Sum(nil)
		if !hmac.Equal(m, x) {
			err = fmt.Errorf("invalid HMAC:%x from:%s", m, conn.RemoteAddr())
			return
		}
	}
	d := gob.NewDecoder(bytes.NewReader(b))
	err = d.Decode(&streamer)
	return
}

// validNonce records the given nonce as having been used, and returns true for
// the first usage.
func (s *StreamServer) validNonce(nonce []byte) bool {
	s.nonceMtx.Lock()
	defer s.nonceMtx.Unlock()
	if _, ok := s.nonce[string(nonce)]; ok {
		return false
	}
	s.nonce[string(nonce)] = struct{}{}
	return true
}

// StreamClient is the client used for stream oriented protocols.
type StreamClient struct {
	// Addr is the dial address, as specified to the address parameter in
	// net.Dial (e.g. "addr:port").
	Addr string

	// AddrKey is a key used to obtain the dial address from the incoming
	// Feedback, if Addr is not specified.
	AddrKey string

	// Protocol is the protocol to use (tcp, tcp4 or tcp6).
	Protocol string

	// Key is a security key for HMAC signing.
	Key []byte

	Streamers
}

// Run implements runner
func (s *StreamClient) Run(ctx context.Context, arg runArg) (ofb Feedback,
	err error) {
	var a string
	if a, err = s.addr(arg.ifb); err != nil {
		return
	}
	r := s.streamer()
	d := net.Dialer{}
	if r, ok := r.(dialController); ok {
		d.Control = r.dialControl
	}
	var c net.Conn
	if c, err = d.DialContext(ctx, s.Protocol, a); err != nil {
		return
	}
	defer c.Close()
	done := make(chan struct{})
	defer close(done)
	go func() {
		var t <-chan time.Time
		d := ctx.Done()
		for done != nil {
			select {
			case <-d:
				d = nil
				t = time.After(1 * time.Second)
			case <-t:
				arg.rec.Logf("StreamClient closing after 1s cancel timeout")
				c.Close()
				done = nil
			case <-done:
				done = nil
			}
		}
	}()
	var h []byte
	if h, err = s.header(r); err != nil {
		return
	}
	if _, err = c.Write(h); err != nil {
		return
	}
	err = r.handleClient(ctx, c, arg)
	return
}

// header returns the client header as a byte slice.
func (s *StreamClient) header(streamer streamer) (hdr []byte, err error) {
	var b bytes.Buffer // buf to hold gobbed streamer
	if err = gob.NewEncoder(&b).Encode(&streamer); err != nil {
		return
	}
	if b.Len() > math.MaxUint16 {
		err = fmt.Errorf("encoded streamer too large, %d > %d",
			b.Len(), math.MaxUint16)
		return
	}
	r := b.Bytes() // gobbed streamer bytes
	if len(s.Key) > 0 {
		n := make([]byte, nonceLen) // nonce
		if _, err = rand.Read(n); err != nil {
			return
		}
		h := hmac.New(sha256.New, s.Key)
		h.Write(n)
		h.Write(r)
		m := h.Sum(nil)
		hdr = append(hdr, n...)
		hdr = append(hdr, m...)
	}
	l := uint16(b.Len())
	if hdr, err = binary.Append(hdr, binary.LittleEndian, l); err != nil {
		return
	}
	hdr = append(hdr, r...)
	return
}

// SetKey implements SetKeyer
func (s *StreamClient) SetKey(key []byte) {
	s.Key = key
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
	handleClient(context.Context, net.Conn, runArg) error

	// handleServer handles a server connection.
	handleServer(context.Context, net.Conn, runArg) error
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
	arg runArg) error {
	arg.rec.Send(u.Info(false))
	return u.send(ctx, conn, arg)
}

// handleServer implements streamer
func (u Upload) handleServer(ctx context.Context, conn net.Conn,
	arg runArg) error {
	arg.rec.Send(u.Info(true))
	return u.receive(ctx, conn, arg)
}

func (u Upload) String() string {
	return fmt.Sprintf("Upload[Flow:%s]", u.Flow)
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
func (d Download) handleClient(ctx context.Context, conn net.Conn,
	arg runArg) error {
	arg.rec.Send(d.Info(false))
	return d.receive(ctx, conn, arg)
}

// handleServer implements streamer
func (d Download) handleServer(ctx context.Context, conn net.Conn,
	arg runArg) (err error) {
	if len(d.Sockopt) > 0 {
		var t *net.TCPConn
		var ok bool
		if t, ok = conn.(*net.TCPConn); !ok {
			err = fmt.Errorf("not a TCPConn for setting Sockopts: %T")
			return
		}
		for _, o := range d.sockopt() {
			if err = o.setTCP(t); err != nil {
				return
			}
		}
	}
	arg.rec.Send(d.Info(true))
	err = d.send(ctx, conn, arg)
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

// Stream represents one direction of a stream oriented flow.
type Stream struct {
	// Flow is the Stream's flow identifier.
	Flow Flow

	// Direction is the client to server sense.
	Direction Direction

	// Sockopts provides support for socket options.
	Sockopts
}

// Info returns StreamInfo for this Stream.
func (s Stream) Info(server bool) StreamInfo {
	return StreamInfo{metric.Tinit, s, server}
}

func (s Stream) String() string {
	return fmt.Sprintf("Stream[Flow:%s Direction:%s CCA:%s]",
		s.Flow, s.Direction, s.CCA)
}

// StreamInfo contains information for a stream flow.
type StreamInfo struct {
	// Tinit is the base time for the flow's RelativeTime values.
	Tinit time.Time

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

// Transfer contains the parameters for an Upload or Download.
type Transfer struct {
	// Duration is the length of time after which the sender stops writing.
	Duration metric.Duration

	// Length is the number of bytes after which the sender stops writing.
	Length metric.Bytes

	// IOSampleInterval is the minimum time between IO samples. Zero disables
	// IO sampling. A value of 1ns typically means a sample will be recorded for
	// every read and write.
	IOSampleInterval metric.Duration

	// TCPInfoInterval is the sampling interval for TCPInfo from Linux. Zero
	// means TCPInfo sampling is disabled.
	TCPInfoInterval metric.Duration

	// BufLen is the size of the buffer used to read and write from the conn.
	BufLen int

	// Nonce is a secure random number used for client authentication.
	Nonce []byte

	// HMAC is the hash value for the nonce.
	HMAC []byte

	Stream
}

const (
	transferFill  byte = 0xf0 // fill byte for transfers
	transferFinal      = 0xfe // final byte for transfers
	transferACK        = 0xff // ack byte for transfers
)

// send runs the send side of a transfer.
func (x Transfer) send(ctx context.Context, conn net.Conn, arg runArg) (
	err error) {
	b := make([]byte, x.BufLen)
	for i := 0; i < x.BufLen; i++ {
		b[i] = transferFill
	}
	in, dur := x.IOSampleInterval.Duration(), x.Duration.Duration()
	t0 := metric.Now()
	arg.rec.Send(StreamIO{x.Flow, t0, 0, true})
	if x.TCPInfoInterval > 0 {
		a := sockAddrConn(conn)
		id := TCPInfoID{x.Flow, Client}
		i := x.TCPInfoInterval.Duration()
		arg.sockdiag.Add(a, id, i)
		defer arg.sockdiag.Remove(a, i)
	}
	t := t0
	ts := t0
	var l metric.Bytes
	var done bool
	var n int
	for !done {
		bl := len(b)
		if dur > 0 && time.Duration(t-t0) >= dur {
			bl = 1
			done = true
		} else if x.Length > 0 && x.Length-l <= metric.Bytes(bl) {
			bl = int(x.Length - l)
			done = true
		}
		if done {
			b[bl-1] = transferFinal
		}
		n, err = conn.Write(b[:bl])
		t = metric.Now()
		l += metric.Bytes(n)
		if n > 0 && in > 0 {
			if time.Duration(t-ts) > in || done {
				arg.rec.Send(StreamIO{x.Flow, t, l, true})
				ts = t
			}
		}
		if err != nil {
			return
		}
		select {
		case <-ctx.Done():
			err = context.Cause(ctx)
			return
		default:
		}
	}
	if n, err = conn.Read(b); err != nil {
		return
	}
	if n != 1 {
		err = fmt.Errorf("unexpected read length: %d", n)
	} else if b[0] != transferACK {
		err = fmt.Errorf("unexpected ACK byte: %x", b[0])
	}
	return
}

// receive runs the receive side of a transfer.
func (x Transfer) receive(ctx context.Context, conn io.ReadWriter, arg runArg) (
	err error) {
	b := make([]byte, x.BufLen)
	in := x.IOSampleInterval.Duration()
	t0 := metric.Now()
	arg.rec.Send(StreamIO{x.Flow, t0, 0, false})
	ts := t0
	var l metric.Bytes
	var done bool
	var n int
	for !done {
		r := x.BufLen
		if x.Length > 0 && x.Length-l < metric.Bytes(x.BufLen) {
			r = int(x.Length - l)
		}
		n, err = conn.Read(b[:r])
		t := metric.Now()
		l += metric.Bytes(n)
		if n > 0 {
			if b[n-1] == transferFinal {
				done = true
			}
			if in > 0 && time.Duration(t-ts) > in || done || err != nil {
				arg.rec.Send(StreamIO{x.Flow, t, l, false})
				ts = t
			}
		}
		if err != nil {
			return
		}
		select {
		case <-ctx.Done():
			err = context.Cause(ctx)
			return
		default:
		}
	}
	b[0] = transferACK
	if n, err = conn.Write(b[:1]); n != 1 && err == nil {
		err = fmt.Errorf("unexpected ack write len: %d", n)
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
