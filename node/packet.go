// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"math/rand"
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

// PacketFlag represents the flag bits on a packet.
type PacketFlag byte

const (
	// FlagEcho indicates that the packet requests an echo.
	FlagEcho PacketFlag = 1 << iota

	// FlagReply indicates that the packet is a reply to an echo request.
	FlagReply
)

// packetMagic is the 7-byte magic sequence at the beginning of a packet.
var packetMagic = []byte{0xaa, 0x49, 0x7c, 0x06, 0x31, 0xe9, 0x45}

// Packet represents a Packet sent in either direction between a PacketClient
// and PacketServer. Only the header is included in the body of the Packet.
// Padding is added to reach the Packet Length.
type Packet struct {
	PacketHeader

	// Len is the total length of the packet, in bytes, including the header.
	Len int

	// addr is the address the packet is from or to.
	addr net.Addr

	// done, if true, indicates that a packetSender is done.
	done bool

	// err is an error that supersedes the remaining fields.
	err error
}

// PacketHeader represents the header of the packet.
type PacketHeader struct {
	// Flag contains the packet flags.
	Flag PacketFlag

	// Seq is the sequence number assigned by the client.
	Seq Seq

	// Flow is the flow identifier, and corresponds to a client and server pair.
	Flow Flow
}

// Write implements io.Writer to "write" from bytes to the packet.
func (p *PacketHeader) Write(b []byte) (n int, err error) {
	if p.Len() > len(b) {
		err = fmt.Errorf("packet header len %d > buf len %d", p.Len(), len(b))
		return
	}
	if !bytes.Equal(b[0:7], packetMagic) {
		err = fmt.Errorf("invalid packet magic: %x", b[0:7])
	}
	p.Flag = PacketFlag(b[7])
	p.Seq = Seq(binary.LittleEndian.Uint64(b[8:16]))
	p.Flow = Flow(string(b[17 : 17+b[16]]))
	n = p.Len()
	return
}

// Read implements io.Reader to "read" from the packet to bytes.
func (p *PacketHeader) Read(b []byte) (n int, err error) {
	if len(b) < p.Len() {
		err = fmt.Errorf("buf len %d < packet header len %d", len(b), p.Len())
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
	n = p.Len()
	return
}

// Len returns the length of the header, in bytes.
func (p *PacketHeader) Len() int {
	return len(packetMagic) + 1 + 8 + 1 + len(p.Flow)
}

// PacketServer is the server used for packet oriented protocols.
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
func (s *PacketServer) Cancel() error {
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
		f := make(map[Flow]struct{})
		var p Packet
		var n int
		var a net.Addr
		b := make([]byte, s.MaxPacketSize)
		for {
			if n, a, e = conn.ReadFrom(b); e != nil {
				return
			}
			t := metric.Now()
			if _, e = p.Write(b[:n]); e != nil {
				return
			}
			if _, ok := f[p.Flow]; !ok {
				rec.Send(PacketInfo{metric.Tinit, p.Flow, true})
				f[p.Flow] = struct{}{}
			}
			rec.Send(PacketIO{p, t, false})
			if p.Flag&FlagEcho != 0 {
				p.Flag = FlagReply
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

// PacketClient is the client used for packet oriented protocols.
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
func (c *PacketClient) Run(ctx context.Context, arg runArg) (ofb Feedback,
	err error) {
	dl := net.Dialer{}
	var cn net.Conn
	if cn, err = dl.DialContext(ctx, c.Protocol, c.Addr); err != nil {
		return
	}
	arg.rec.Send(PacketInfo{metric.Tinit, c.Flow, false})
	out := make(chan Packet)
	var q seqSrc
	var in []chan Packet
	var g int
	for _, s := range c.Sender {
		g++
		p := make(chan Packet)
		in = append(in, p)
		go s.packetSender().send(&q, p, out)
	}
	g++
	rc := c.read(cn.(net.PacketConn), arg.rec)
	defer func() {
		for _, i := range in {
			close(i)
		}
		cn.Close()
		for g > 0 {
			select {
			case p := <-out:
				if p.done {
					g--
				}
				if p.err != nil && err == nil {
					err = p.err
				}
			case _, ok := <-rc:
				if !ok {
					g--
				}
			}
		}
	}()
	b := make([]byte, c.MaxPacketSize)
	for g > 0 {
		select {
		case p := <-out:
			if p.err != nil {
				err = p.err
				return
			}
			if p.done {
				if g--; g == 1 {
					return
				}
				break
			}
			p.Flow = c.Flow
			var n int
			if n, err = p.Read(b); err != nil {
				return
			}
			if p.Len == 0 {
				p.Len = n
			} else if p.Len < n {
				err = fmt.Errorf("requested packet len %d < header len %d",
					p.Len, n)
				return
			}
			if _, err = cn.Write(b[:p.Len]); err != nil {
				return
			}
			arg.rec.Send(PacketIO{p, metric.Now(), true})
		case p, ok := <-rc:
			if !ok {
				g--
				return
			}
			if p.err != nil {
				err = p.err
				return
			}
			for _, i := range in {
				i <- p
			}
		case <-ctx.Done():
			return
		}
	}
	return
}

// read is the entry point for the conn read goroutine.
func (p *PacketClient) read(conn net.PacketConn, rec *recorder) (
	rc chan Packet) {
	rc = make(chan Packet)
	go func() {
		b := make([]byte, p.MaxPacketSize)
		var n int
		var a net.Addr
		var e error
		defer func() {
			if e != nil {
				rc <- Packet{err: e}
			}
			close(rc)
		}()
		for {
			n, a, e = conn.ReadFrom(b)
			if e != nil {
				break
			}
			var p Packet
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
	send(seq *seqSrc, in, out chan Packet)
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

// Unresponsive sends packets on a schedule without regard to any congestion
// signals.
type Unresponsive struct {
	// Wait lists the wait times between packets, which are cycled through
	// either sequentially or randomly (according to RandomWait) until all
	// packets are sent.
	Wait []metric.Duration

	// WaitFirst, if true, indicates to wait before sending the first packet as
	// well.
	WaitFirst bool

	// RandomWait, if true, indicates to use random wait times from the list.
	// Otherwise, the wait times are taken from Wait sequentially.
	RandomWait bool

	// Length lists the lengths of the packets, which are cycled through either
	// sequentially or randomly (according to RandomLength) until all packets
	// are sent.
	Length []int

	// RandomLength, if true, indicates to use random lengths from the list.
	// Otherwise, the lengths are taken from Length sequentially.
	RandomLength bool

	// Duration is how long to send packets.
	Duration metric.Duration

	// Echo, if true, requests mirrored replies from the server.
	Echo bool

	// waitIndex is the current index in Wait.
	waitIndex int

	// lengthIndex is the current index in Length.
	lengthIndex int

	// rand provides random numbers.
	rand *rand.Rand
}

// send implements packetSender
func (u *Unresponsive) send(seq *seqSrc, in, out chan Packet) {
	var e error
	defer func() {
		out <- Packet{done: true, err: e}
	}()
	t0 := time.Now()
	var w <-chan time.Time
	if len(u.Wait) <= 1 {
		t := time.NewTicker(u.nextWait())
		defer t.Stop()
		w = t.C
	} else {
		w = time.After(u.firstWait())
	}
	for {
		select {
		case _, ok := <-in:
			if !ok {
				e = fmt.Errorf("PacketClient Unresponsive sender was canceled")
				return
			}
		case <-w:
			if time.Since(t0) >= u.Duration.Duration() {
				return
			}
			var f PacketFlag
			if u.Echo {
				f |= FlagEcho
			}
			out <- Packet{PacketHeader{f, seq.Next(), ""}, u.nextLength(),
				nil, false, nil}
			if len(u.Wait) > 1 {
				w = time.After(u.nextWait())
			}
		}
	}
}

// firstWait returns the first wait time.
func (u *Unresponsive) firstWait() time.Duration {
	if !u.WaitFirst {
		return 0
	}
	return u.nextWait()
}

// nextWait returns the next wait time.
func (u *Unresponsive) nextWait() (wait time.Duration) {
	if len(u.Wait) == 0 {
		return
	}
	if u.RandomWait {
		if u.rand == nil {
			u.rand = rand.New(rand.NewSource(time.Now().UnixNano()))
		}
		wait = time.Duration(u.Wait[u.rand.Intn(len(u.Wait))])
		return
	}
	wait = time.Duration(u.Wait[u.waitIndex])
	if u.waitIndex++; u.waitIndex >= len(u.Wait) {
		u.waitIndex = 0
	}
	return
}

// nextLength returns the next packet length.
func (u *Unresponsive) nextLength() (length int) {
	if len(u.Length) == 0 {
		return
	}
	if u.RandomLength {
		if u.rand == nil {
			u.rand = rand.New(rand.NewSource(time.Now().UnixNano()))
		}
		length = u.Length[u.rand.Intn(len(u.Length))]
		return
	}
	length = u.Length[u.lengthIndex]
	if u.lengthIndex++; u.lengthIndex >= len(u.Length) {
		u.lengthIndex = 0
	}
	return
}

// PacketInfo contains information for a packet flow.
type PacketInfo struct {
	// Tinit is the base time for the flow's RelativeTime values.
	Tinit time.Time

	// Flow is the flow identifier.
	Flow Flow

	// Server indicates if this is from the server (true) or client (false).
	Server bool
}

// init registers PacketInfo with the gob encoder
func init() {
	gob.Register(PacketInfo{})
}

// Time returns an absolute from a node-relative time.
func (p PacketInfo) Time(r metric.RelativeTime) time.Time {
	return p.Tinit.Add(time.Duration(r))
}

// flags implements message
func (PacketInfo) flags() flag {
	return flagForward
}

// handle implements event
func (p PacketInfo) handle(node *node) {
	node.parent.Send(p)
}

func (p PacketInfo) String() string {
	return fmt.Sprintf("PacketInfo[Tinit:%s Flow:%s]", p.Tinit, p.Flow)
}

// PacketIO is a time series data point that records packet send and receive
// times.
type PacketIO struct {
	// Packet is the packet.
	Packet

	// T is the node-relative time this PacketIO was recorded.
	T metric.RelativeTime

	// Sent is true for a sent packet, and false for received.
	Sent bool
}

// init registers PacketIO with the gob encoder
func init() {
	gob.Register(PacketIO{})
}

// flags implements message
func (PacketIO) flags() flag {
	return flagForward
}

// handle implements event
func (p PacketIO) handle(node *node) {
	node.parent.Send(p)
}

func (p PacketIO) String() string {
	return fmt.Sprintf("PacketIO[Packet:%s T:%s Sent:%t]",
		p.Packet, p.T, p.Sent)
}
