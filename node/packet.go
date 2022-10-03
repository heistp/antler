// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/heistp/antler/node/metric"
)

// Seq is a packet sequence number.
type Seq uint64

// seqSrc provides a concurrent-safe source of monotonically increasing sequence
// numbers. The zero value is ready to use.
type seqSrc struct {
	seq Seq
	mtx sync.Mutex
}

// Next returns the next sequence number.
func (s *seqSrc) Next() (seq Seq) {
	s.mtx.Lock()
	seq = s.seq
	s.seq++
	s.mtx.Unlock()
	return
}

// packetFlag represents the flag bits on a packet.
type packetFlag byte

const (
	// pFlagEcho indicates that the packet requests an echo.
	pFlagEcho packetFlag = 1 << iota

	// pFlagReply indicates that the packet is a reply to an echo request.
	pFlagReply
)

// packetMagic is the 7-byte magic sequence at the beginning of a packet.
var packetMagic = []byte{0xaa, 0x49, 0x7c, 0x06, 0x31, 0xe9, 0x45}

// packet represents a packet sent in either direction between a PacketClient
// and PacketServer. Only the exported fields are included in the body of the
// packet.
type packet struct {
	// Flag contains the packet flags.
	Flag packetFlag

	// Seq is the sequence number assigned by the client.
	Seq Seq

	// Flow is the flow identifier, and corresponds to a client and server pair.
	Flow Flow

	// length is the total length of the packet, in bytes, including the header.
	length int

	// addr is the address the packet is from or to.
	addr net.Addr

	// err is an error that supersedes the remaining fields.
	err error
}

// Write implements io.Writer to "write" from bytes to the packet.
func (p *packet) Write(b []byte) (n int, err error) {
	if p.headerLen() > len(b) {
		err = fmt.Errorf("packet header len %d > buf len %d", p.headerLen(),
			len(b))
		return
	}
	if !bytes.Equal(b[0:7], packetMagic) {
		err = fmt.Errorf("invalid packet magic: %x", b[0:7])
	}
	p.Flag = packetFlag(b[7])
	p.Seq = Seq(binary.LittleEndian.Uint64(b[8:16]))
	p.Flow = Flow(string(b[17 : 17+b[16]]))
	n = p.headerLen()
	return
}

// Read implements io.Reader to "read" from the packet to bytes.
func (p *packet) Read(b []byte) (n int, err error) {
	if len(b) < p.headerLen() {
		err = fmt.Errorf("buf len %d < packet header len %d", len(b),
			p.headerLen())
		return
	}
	if len(p.Flow) > 255 {
		err = fmt.Errorf("flow name %s > 255 characters", len(p.Flow))
		return
	}
	copy(b, packetMagic)
	b[7] = byte(p.Flag)
	binary.LittleEndian.PutUint64(b[8:16], uint64(p.Seq))
	b[16] = byte(len(p.Flow))
	copy(b[17:], []byte(p.Flow))
	n = p.headerLen()
	return
}

// headerLen returns the length of the header, in bytes.
func (p *packet) headerLen() int {
	return len(packetMagic) + 1 + 8 + 1 + len(p.Flow)
}

// PacketServer is a server used for packet oriented protocols.
type PacketServer struct {
	// ListenAddr is the listen address, as specified to the address parameter
	// in net.ListenPacket (e.g. ":port" or "addr:port").
	ListenAddr string

	// Protocol is the protocol to use (udp, udp4 or udp6).
	Protocol string

	// MaxPacketSize is the maximum size of a received packet.
	MaxPacketSize int

	errc chan error
}

// Run implements runner
func (s *PacketServer) Run(ctx context.Context, arg runArg) (ofb Feedback,
	err error) {
	g := net.ListenConfig{}
	var c net.PacketConn
	if c, err = g.ListenPacket(ctx, s.Protocol, s.ListenAddr); err != nil {
		return
	}
	s.errc = make(chan error)
	s.start(ctx, c, arg.rec)
	arg.cxl <- s
	return
}

// Cancel implements canceler
func (s *PacketServer) Cancel(rec *recorder) error {
	return <-s.errc
}

// start starts the main and packet handling goroutines.
func (s *PacketServer) start(ctx context.Context, conn net.PacketConn,
	rec *recorder) {
	ec := make(chan error)
	// main goroutine
	go func() {
		var err error
		defer func() {
			if err != nil {
				s.errc <- err
			}
			close(s.errc)
		}()
		dc := ctx.Done()
		var d bool
		for d {
			select {
			case <-dc:
				dc = nil
				d = true
				err = conn.Close()
			case e, ok := <-ec:
				if !ok {
					d = true
					break
				}
				if dc == nil {
					rec.Logf("post-cancel error: %s", e)
					break
				}
				rec.SendErrore(e)
			}
		}
	}()
	// packet handling goroutine
	go func() {
		var e error
		defer func() {
			if e != nil {
				ec <- e
			}
			close(ec)
		}()
		var p packet
		var n int
		var a net.Addr
		b := make([]byte, s.MaxPacketSize)
		for {
			if n, a, e = conn.ReadFrom(b); e != nil {
				return
			}
			if _, e = p.Write(b[:n]); e != nil {
				return
			}
			if p.Flag&pFlagEcho != 0 {
				p.Flag = pFlagReply
				if n, e = p.Read(b); e != nil {
					return
				}
				if _, e = conn.WriteTo(b[:n], a); e != nil {
					return
				}
			}
		}
	}()
}

// PacketClient is a client used for packet oriented protocols.
type PacketClient struct {
	// Addr is the dial address, as specified to the address parameter in
	// net.Dial (e.g. "addr:port").
	Addr string

	// Protocol is the protocol to use (udp, udp4 or udp6).
	Protocol string

	// Flow is the flow identifier for traffic between the client and server.
	Flow Flow

	// MaxPacketSize is the maximum size of a received packet.
	MaxPacketSize int

	Sender []PacketSenders
}

// Run implements runner
func (p *PacketClient) Run(ctx context.Context, arg runArg) (ofb Feedback,
	err error) {
	dl := net.Dialer{}
	var c net.Conn
	if c, err = dl.DialContext(ctx, p.Protocol, p.Addr); err != nil {
		return
	}
	out := make(chan packet)
	var q seqSrc
	var in []chan packet
	var g int
	for _, s := range p.Sender {
		g++
		i := make(chan packet)
		in = append(in, i)
		go s.packetSender().send(&q, i, out)
	}
	g++
	rc := p.read(c.(net.PacketConn))
	defer func() {
		for _, i := range in {
			close(i)
		}
		c.Close()
		for g > 0 {
			select {
			case k := <-out:
				if k == (packet{}) {
					g--
				}
				if k.err != nil && err == nil {
					err = k.err
				}
			case _, ok := <-rc:
				if !ok {
					g--
				}
			}
		}
	}()
	b := make([]byte, p.MaxPacketSize)
	for g > 0 {
		select {
		case k := <-out:
			if k == (packet{}) {
				if g--; g == 1 {
					return
				}
				break
			}
			if k.err != nil {
				err = k.err
				return
			}
			k.Flow = p.Flow
			var n int
			if n, err = k.Read(b); err != nil {
				return
			}
			if k.length == 0 {
				k.length = n
			} else if k.length < n {
				err = fmt.Errorf("requested packet len %d < header len %d",
					k.length, n)
				return
			}
			if _, err = c.Write(b[:k.length]); err != nil {
				return
			}
		case k, ok := <-rc:
			if !ok {
				g--
				return
			}
			if k.err != nil {
				err = k.err
				return
			}
			for _, i := range in {
				i <- k
			}
		case <-ctx.Done():
			return
		}
	}
	return
}

// read is the entry point for the conn read goroutine.
func (p *PacketClient) read(conn net.PacketConn) (rc chan packet) {
	rc = make(chan packet)
	go func() {
		b := make([]byte, p.MaxPacketSize)
		var n int
		var a net.Addr
		var e error
		defer func() {
			if e != nil {
				rc <- packet{err: e}
			}
			close(rc)
		}()
		for {
			n, a, e = conn.ReadFrom(b)
			if e != nil {
				break
			}
			var p packet
			p.addr = a
			if _, e = p.Write(b[:n]); e != nil {
				return
			}
			rc <- p
		}
	}()
	return
}

// A packetSender can send outgoing and react to incoming packets. The send
// method must read from the in channel until it's closed, at which point send
// should complete as soon as possible, and send a zero value packet to out,
// with an error if the sender was forced to completed abnormally.
type packetSender interface {
	send(seq *seqSrc, in, out chan packet)
}

// PacketSenders is the union of available packetSender implementations.
type PacketSenders struct {
	Unresponsive *Unresponsive
}

// packetSender returns the only non-nil packetSender implementation.
func (p *PacketSenders) packetSender() packetSender {
	switch {
	case p.Unresponsive != nil:
		return p.Unresponsive
	default:
		panic("no packetSender set in packetSender union")
	}
}

// Unresponsive sends packets on a periodic schedule with fixed interval.
type Unresponsive struct {
	// Interval is the fixed time between ticks.
	Interval metric.Duration

	// Length is the length of the packets.
	Length int

	// Duration is how long to send packets.
	Duration metric.Duration

	// Echo, if true, requests mirrored replies from the server.
	Echo bool
}

// send implements packetSender
func (i *Unresponsive) send(seq *seqSrc, in, out chan packet) {
	var e error
	defer func() {
		out <- packet{err: e}
	}()
	sendPacket := func() {
		out <- packet{pFlagEcho, seq.Next(), "", i.Length, nil, nil}
	}
	t0 := time.Now()
	t := time.NewTicker(i.Interval.Duration())
	defer t.Stop()
	sendPacket()
	for {
		select {
		case _, ok := <-in:
			if !ok {
				e = fmt.Errorf("%s isochronous sender did not complete",
					i.Interval)
				return
			}
		case <-t.C:
			if time.Since(t0) >= i.Duration.Duration() {
				return
			}
			sendPacket()
		}
	}
}
