// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

// Location represents a position on a path.
type Location string

const (
	Client Location = "client"
	Server          = "server"
)

// Direction is the client to server sense for a Stream.
type Direction string

const (
	Up   Direction = "up"   // client to server
	Down           = "down" // server to client
)
