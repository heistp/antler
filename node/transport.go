// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

import (
	"encoding/gob"
	"fmt"
	"io"
	"runtime/debug"
)

// A transport can send and receive messages, and is used for the underlying
// communication in conn. Implementations are expected to deliver messages
// reliably and in order. Callers of transport must call Close, exactly once,
// after Send/Receive.
type transport interface {
	Send(message) error        // sends a message
	Receive() (message, error) // receives a message
	io.Closer
}

// gobTransport is a transport that uses gob.
type gobTransport struct {
	closer io.Closer
	enc    *gob.Encoder
	dec    *gob.Decoder
}

// newGobTransport returns a new gobTransport for the given underlying conn.
func newGobTransport(conn io.ReadWriteCloser) *gobTransport {
	return &gobTransport{conn, gob.NewEncoder(conn), gob.NewDecoder(conn)}
}

// Send implements transport
func (g *gobTransport) Send(m message) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("encode panic: %s\n%s\nmessage: '%s'",
				r, string(debug.Stack()), m)
		}
	}()
	err = g.enc.Encode(&m)
	return
}

// Receive implements transport
func (g *gobTransport) Receive() (m message, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("decode panic: %s\n%s\ndata: '%s'",
				r, string(debug.Stack()), m)
		}
	}()
	err = g.dec.Decode(&m)
	return
}

// Close implements transport/io.Closer
func (g *gobTransport) Close() error {
	return g.closer.Close()
}

// channelTransport is a transport that uses channels.
type channelTransport struct {
	recv chan message
	send chan message
}

// newChannelTransport returns a new channelTransport instance.
func newChannelTransport() *channelTransport {
	return &channelTransport{
		make(chan message),
		make(chan message),
	}
}

// peer returns a transport with the send and receive channels flipped.
func (c *channelTransport) peer() *channelTransport {
	return &channelTransport{c.send, c.recv}
}

// Send implements transport
func (c *channelTransport) Send(m message) error {
	c.send <- m
	return nil
}

// Receive implements transport
func (c *channelTransport) Receive() (message, error) {
	return <-c.recv, nil
}

// Close implements transport
func (c *channelTransport) Close() error {
	close(c.send)
	return nil
}
