// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
)

// Seq is a packet sequence number.
type Seq uint64

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
// packet. The unexported fields are used by the client.
type packet struct {
	// Flag contains the packet flags.
	Flag packetFlag

	// Seq is the sequence number assigned by the client.
	Seq Seq

	// Flow is the flow identifier, and corresponds to a client and server pair.
	Flow Flow

	// length is the total length of the packet, in bytes. After the exported
	// fields are encoded to the packet, padding is added to reach this length.
	length int

	// reply is a channel on which to send replies to this packet.
	reply chan packet

	// to is the server address to send the packet to.
	to net.Addr
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

// start starts the main and packet handling goroutine.
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
		b := make([]byte, 1500)
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

	Scheduler []Schedulers
}

// Run implements runner
func (p *PacketClient) Run(ctx context.Context, arg runArg) (ofb Feedback,
	err error) {
	d := net.Dialer{}
	var c net.Conn
	if c, err = d.DialContext(ctx, p.Protocol, p.Addr); err != nil {
		return
	}
	defer c.Close()
	// TODO run goroutine for each scheduler

	// on each tick, send a packet to the server
	// if Reply is set in the tick, set Echo to true in the packet, and save the
	// scheduler's in channel in a map, for somewhere to write the reply to

	// on each reply, send a tick to the corresponding scheduler

	// increment seqnos and record data along the way: PktSent, PktReturned

	return
}

type PktRcvd struct {
}
