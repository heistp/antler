// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

// Veth contains the information needed to create a new virtual Ethernet
// interface in Linux (man ip-link(8)).
type Veth struct {
	Name          string   // the interface's name, unique to a Node
	PeerNamespace string   // the owning namespace of the veth's peer
	PeerName      string   // name of the peer veth device
	Addrs         []string // addresses in CIDR notation (e.g. 192.168.0.0/24)
}

// Route contains the information used to create a static route.
type Route struct {
	Prefix string // route prefix, in CIDR notation (e.g. 192.168.0.0/24)
	Via    string // IP address of the nexthop router
	Dev    string // output device name
}
