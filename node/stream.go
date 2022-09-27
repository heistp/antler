// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"syscall"
	"time"

	"github.com/heistp/antler/node/metric"
	"golang.org/x/sys/unix"
)

// Stream contains the parameters for a stream, used in the client, server and
// StreamInfo.
type Stream struct {
	// Flow identifies the stream.
	Flow Flow

	// Download indicates whether to run the test from server to client (true)
	// or client to server (false).
	Download bool

	// CCA sets the Congestion Control Algorithm used for the stream.
	CCA string

	// Duration is the length of time the stream runs.
	Duration metric.Duration

	// SampleIOInterval is the minimum time between StreamSamples. Zero means a
	// sample will be returned for every read and write.
	SampleIOInterval metric.Duration

	// ReadBufLen is the size of the buffer used to read from the conn.
	ReadBufLen int

	// WriteBufLen is the size of the buffer used to write to the conn.
	WriteBufLen int
}

// tcpControl provides ListenConfig.Control and Dialer.Control for TCP.
func (s *Stream) tcpControl(network, address string, conn syscall.RawConn) (
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
func (s *Stream) send(ctx context.Context, w io.Writer, rec *recorder) (
	err error) {
	b := make([]byte, s.WriteBufLen)
	for i := 0; i < s.WriteBufLen; i++ {
		b[i] = 0xfe
	}
	in, dur := s.SampleIOInterval.Duration(), s.Duration.Duration()
	t0 := time.Now()
	//rec.Send(StreamInfo{t0, *s})
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
				//rec.Send(Sent{s.Flow, dt, l})
				ts = t
			}
		}
	}
	return
}

// receive runs the receive side of a stream.
func (s *Stream) receive(r io.Reader, rec *recorder) (err error) {
	b := make([]byte, s.ReadBufLen)
	in := s.SampleIOInterval.Duration()
	t0 := time.Now()
	rec.Send(StreamInfo{t0, *s})
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

func (s *Stream) String() string {
	return fmt.Sprintf("Stream[Flow:%s Download:%t CCA:%s]",
		s.Flow, s.Download, s.CCA)
}

// StreamInfo contains meta-information about a stream.
type StreamInfo struct {
	T0     time.Time // T0 is the stream start time
	Stream           // Stream contains the stream parameters
}

// init registers StreamInfo with the gob encoder
func init() {
	gob.Register(StreamInfo{})
}

// flags implements message
func (StreamInfo) flags() flag {
	return flagForward
}

// handle implements event
func (i StreamInfo) handle(node *node) {
	node.parent.Send(i)
}

func (i StreamInfo) String() string {
	return fmt.Sprintf("StreamInfo[T0:%s Stream:%s]",
		i.T0, i.Stream.String())
}

// Sent is a time series data point containing a total number of sent bytes.
type Sent struct {
	Flow  Flow          // flow that this Sent belongs to
	T     time.Duration // duration since sending began (e.g. T0 in StreamInfo)
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

// Received is a time series data point containing a total number of received
// bytes.
type Received struct {
	Flow  Flow          // flow that this Received belongs to
	T     time.Duration // duration since receiving began (e.g. T0 in StreamInfo)
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
