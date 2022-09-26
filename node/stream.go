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

// StreamSample is a time series data point containing a total number of bytes
// sent or received by a stream runner.
type StreamSample struct {
	Series Series        // series the StreamSample belongs to
	T      time.Duration // duration since stream began (T0 in StreamInfo)
	Total  metric.Bytes  // total byte count sent or received
}

// init registers StreamSample with the gob encoder
func init() {
	gob.Register(StreamSample{})
}

// flags implements message
func (StreamSample) flags() flag {
	return flagForward
}

// handle implements event
func (i StreamSample) handle(node *node) {
	node.parent.Send(i)
}

func (i StreamSample) String() string {
	return fmt.Sprintf("StreamSample[Series:%s T:%s Total:%d]",
		i.Series, i.T, i.Total)
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

// Stream contains the parameters for a stream, used in the client, server and
// StreamInfo.
type Stream struct {
	// Series is the series name.
	Series Series

	// Download indicates whether to run the test from server to client (true)
	// or client to server (false).
	Download bool

	// CCA sets the Congestion Control Algorithm used for the stream.
	CCA string

	// Duration is the length of time the stream runs.
	Duration metric.Duration

	// SampleIO, if true, sends StreamSamples to record the progress of read and
	// write syscalls.
	SampleIO bool

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
	rec.Send(StreamInfo{t0, *s})
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
				rec.Send(StreamSample{s.Series, dt, l})
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
		if s.SampleIO && n > 0 {
			ds := t.Sub(ts)
			if ds > in || err != nil {
				rec.Send(StreamSample{s.Series, dt, l})
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
	return fmt.Sprintf("Stream[Series:%s Download:%t CCA:%s]",
		s.Series, s.Download, s.CCA)
}
