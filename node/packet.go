// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"context"
	"net"
)

// PacketServer is a server used for packet oriented protocols.
type PacketServer struct {
	// ListenAddr is the listen address, as specified to the address parameter
	// in net.ListenPacket (e.g. ":port" or "addr:port").
	ListenAddr string

	// Protocol is the protocol to use (udp, udp4 or udp6).
	Protocol string

	Packeters
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
		p := s.packeter()
		e = p.handleServer(ctx, conn, rec)
	}()
}

// PacketClient is a client used for packet oriented protocols.
type PacketClient struct {
	// Addr is the dial address, as specified to the address parameter in
	// net.Dial (e.g. "addr:port").
	Addr string

	// Protocol is the protocol to use (udp, udp4 or udp6).
	Protocol string

	Packeters
}

// Run implements runner
func (p *PacketClient) Run(ctx context.Context, arg runArg) (ofb Feedback,
	err error) {
	k := p.packeter()
	d := net.Dialer{}
	if r, ok := k.(dialController); ok {
		d.Control = r.dialControl
	}
	var c net.Conn
	if c, err = d.DialContext(ctx, p.Protocol, p.Addr); err != nil {
		return
	}
	defer c.Close()
	err = k.handleClient(ctx, c.(net.PacketConn), arg.rec)
	return
}

// A packeter handles connections in PacketClient and PacketServer.
type packeter interface {
	// handleClient handles a client connection.
	handleClient(context.Context, net.PacketConn, *recorder) error

	// handleServer handles a server connection.
	handleServer(context.Context, net.PacketConn, *recorder) error
}

// Packeters is the union of available packeter implementations.
type Packeters struct {
	Isochronous *Isochronous
}

// packeter returns the only non-nil packeter implementation.
func (p *Packeters) packeter() packeter {
	switch {
	case p.Isochronous != nil:
		return p.Isochronous
	default:
		panic("no packeter set in packeters union")
	}
}

// Isochronous sends and echoes fixed size packets on an isochronous schedule.
type Isochronous struct {
}

// handleClient implements packeter
func (*Isochronous) handleClient(context.Context, net.PacketConn,
	*recorder) error {
	return nil
}

// handleServer implements packeter
func (*Isochronous) handleServer(context.Context, net.PacketConn,
	*recorder) error {
	return nil
}
