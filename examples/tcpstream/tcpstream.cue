// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

// This Antler example config has a single test that creates a netns dumbbell,
// and runs a single TCP stream from the right to the left endpoint. The
// middlebox ("mid" namespace) has the cake qdisc added at 50 Mbit.

package tcpstream

// stream includes logs for streaming during the test
// This is passed to all nodes before setup.
stream: {ResultStream: Include: Log: true}

// Run contains a single Test. After log streaming is configured, setup is
// run, the server is started, then the test is run.
Run: {
	Test: {
		ID: {"Name": "tcpstream"}
		Serial: [stream, setup, server, run]
	}
	Report: [
		{EmitLog: {To: ["-", "node.log"]}},
		{SaveFiles: {}},
		{ExecuteTemplate: {
			From: ["throughput.tpl"]
			To: "throughput.html"
		}},
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
			"sysctl -w net.ipv6.conf.all.disable_ipv6=1",
			"ip link add dev right.l type veth peer name mid.r",
			"ip link set dev mid.r netns mid",
			"ip addr add 10.0.0.2/24 dev right.l",
			"ip link set right.l up",
			"ethtool -K right.l \(#offloads)",
		]
	}
	mid: {
		setup: [
			"sysctl -w net.ipv6.conf.all.disable_ipv6=1",
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
			"tc qdisc add dev mid.r root cake bandwidth 50Mbit",
			"tc qdisc add dev mid.l root netem delay 20ms limit 100000",
			//"modprobe ifb",
			//"ip link add dev i.mid.r type ifb",
			//"tc qdisc add dev i.mid.r root handle 1: netem delay 10ms limit 100000",
			//"tc qdisc add dev mid.r handle ffff: ingress",
			//"ip link set i.mid.r up",
			//"tc filter add dev i.mid.r parent ffff: protocol all prio 0 u32 match u32 0 0 flowid 1:1 action mirred egress redirect dev i.mid.r",
		]
	}
	left: {
		setup: [
			"sysctl -w net.ipv6.conf.all.disable_ipv6=1",
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

// serverAddr is the server listen and client dial address
#serverAddr: "10.0.0.2:777"

// tcpStream contains common TCPStream parameters
tcpStream: {
	Duration:         "3s"
	Download:         false
	SampleIO:         true
	SampleIOInterval: "10ms"
}

// server runs TCPStreamServer in the right namespace
server: {
	Child: {
		Node:            ns.right.node
		TCPStreamServer: tcpStream & {
			ListenAddr: #serverAddr
			Series:     "bytes.server"
		}
	}
}

// run runs the test using TCPStreamClient, to download from the right
// namespace to the left
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
			{TCPStreamClient: tcpStream & {
				Addr:   #serverAddr
				Series: "bytes.client"
			}},
		]
	}
}

// throughputGchart is the throughput plot template for Google Charts.
#throughputGchart: """
	Hello World!
	"""
