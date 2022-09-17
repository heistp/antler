// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

// This Antler example config has a single test that creates a netns dumbbell,
// and runs an iperf3 test between the left and right endpoints. The middlebox
// (mid namespace) has the cake qdisc added at 50 Mbit.

package netns_iperf3

// stream includes logs for streaming during the test
stream: {ResultStream: Include: Log: true}

// Run contains a single Test that runs setup and run in serial
Run: {
	Test: {
		ID: {"Name": "netns-iperf3"}
		Serial: [stream, setup, server, run]
	}
	Report: [
		{EmitLog: {}},
		{SaveFiles: {}},
	]
}

// setup runs the setup commands in each namespace
setup: {
	Serial: [
		for n in [ ns.right, ns.mid, ns.left] {
			Child: {
				Node: n.node
				Serial: [stream, for c in n.setup {System: Command: c}]
			}
		},
	]
}

// ns defines the namespaces and their setup commands
ns: {
	right: {
		setup: [
			"ip link add dev right.l type veth peer name mid.r",
			"ip link set dev mid.r netns mid",
			"ip addr add 10.0.0.2/24 dev right.l",
			"ip link set right.l up",
			"ethtool -K right.l \(#offloads)",
		]
	}
	mid: {
		setup: [
			"ip link set mid.r up",
			"ip link add dev mid.l type veth peer name left.r",
			"ip link set dev left.r netns left",
			"ip link set dev mid.l up",
			"ip link add name mid.b type bridge",
			"ip link set dev mid.r master mid.b",
			"ip link set dev mid.l master mid.b",
			"ip link set dev mid.b up",
			"ethtool -K mid.l \(#offloads)",
			"ethtool -K mid.r \(#offloads)",
			"tc qdisc add dev mid.r root cake bandwidth 50Mbit datacentre",
		]
	}
	left: {
		setup: [
			"ip addr add 10.0.0.1/24 dev left.r",
			"ip link set left.r up",
			"ethtool -K left.r \(#offloads)",
		]
	}
}

// ns template adds a node for each namespace, without having to define each
ns: [id=_]: node: {
	ID:       id
	Platform: "linux-amd64"
	Launcher: Local: {}
	Netns: {Create: true}
}

// offloads contains the features arguments for ethtool to disable offloads
#offloads: "rx off tx off sg off tso off gso off gro off rxvlan off txvlan off"

// server runs the iperf3 server in the right namespace
server: {
	Child: {
		Node: ns.right.node
		System: {
			Command:    "iperf3 -s"
			Background: true
		}
	}
}

// run runs the test using iperf3 from the left namespace to the right
run: {
	Child: {
		Node: ns.left.node
		Serial: [
			{System: {
				Command:    "tcpdump -i left.r -s 128 -w -"
				Background: true
				Stdout:     "left.pcap"
			}},
			{Sleep:           "500ms"},
			{System: Command: "iperf3 -t 10 -c 10.0.0.2"},
		]
	}
}
