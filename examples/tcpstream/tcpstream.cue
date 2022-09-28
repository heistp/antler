// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

// This Antler example config has a single test that creates a chain of four
// network namespaces, and runs two TCP streams from the left to the right
// endpoint. The middleboxes mr and ml are used to provide a delay, and add the
// CAKE qdisc at 50 Mbit.

package tcpstream

// stream includes logs for streaming during the test
// This is passed to all nodes before setup.
stream: {ResultStream: Include: Log: true}

// rtt is the path RTT, in milliseconds
#rtt: 80

// qdisc is the qdisc to apply
#qdisc: "cake bandwidth 50Mbit flowblind"

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
		{GTimeSeries: {
			Title: "CUBIC vs Reno Goodput / \(#qdisc) / \(#rtt)ms RTT"
			To:    "throughput.html"
			FlowLabel: {
				"cubic": "TCP CUBIC"
				"reno":  "TCP Reno"
			}
		}},
	]
}

// setup runs the setup commands in each namespace
setup: {
	Serial: [
		for n in [ ns.right, ns.mr, ns.ml, ns.left] {
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
			"ip link add dev right.l type veth peer name mr.r",
			"ip link set dev mr.r netns mr",
			"ip addr add 10.0.0.2/24 dev right.l",
			"ip link set right.l up",
			"ethtool -K right.l \(#offloads)",
		]
	}
	mr: {
		setup: [
			"sysctl -w net.ipv6.conf.all.disable_ipv6=1",
			"ip link set mr.r up",
			"ip link add dev mr.l type veth peer name ml.r",
			"ip link set dev ml.r netns ml",
			"ip link set dev mr.l up",
			"ip link add name mr.b type bridge",
			"ip link set dev mr.r master mr.b",
			"ip link set dev mr.l master mr.b",
			"ip link set dev mr.b up",
			"ethtool -K mr.l \(#offloads)",
			"ethtool -K mr.r \(#offloads)",
			"tc qdisc add dev mr.r root netem delay \(#rtt/2)ms limit 100000",
		]
	}
	ml: {
		setup: [
			"sysctl -w net.ipv6.conf.all.disable_ipv6=1",
			"ip link set ml.r up",
			"ip link add dev ml.l type veth peer name left.r",
			"ip link set dev left.r netns left",
			"ip link set dev ml.l up",
			"ip link add name ml.b type bridge",
			"ip link set dev ml.r master ml.b",
			"ip link set dev ml.l master ml.b",
			"ip link set dev ml.b up",
			"ethtool -K ml.l \(#offloads)",
			"ethtool -K ml.r \(#offloads)",
			"tc qdisc add dev ml.l root netem delay \(#rtt/2)ms limit 100000",
			"tc qdisc add dev ml.r root \(#qdisc)",
			//"tc qdisc add dev ml.r root handle 1: htb default 1",
			//"tc class add dev ml.r parent 1: classid 1:1 htb rate 50Mbit",
			//"tc qdisc add dev ml.r parent 1:1 pfifo limit 100",
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

// server runs StreamServer in the right namespace
server: {
	Child: {
		Node: ns.right.node
		Serial: [
			{System: {
				Command:    "tcpdump -i right.l -s 128 -w -"
				Background: true
				Stdout:     "right.pcap"
			}},
			{StreamServer: {ListenAddr: #serverAddr}},
		]
	}
}

// run runs the test using StreamClient, to download from the right
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
			{Sleep: "500ms"},
			{Parallel: [
				{StreamClient: {
					Addr:             #serverAddr
					Flow:             "cubic"
					CCA:              "cubic"
					Duration:         "30s"
					Direction:        "upload"
					SampleIOInterval: "\(#rtt/2)ms"
				}},
				{Serial: [
					{Sleep: "10s"},
					{StreamClient: {
						Addr:             #serverAddr
						Flow:             "reno"
						CCA:              "reno"
						Duration:         "10s"
						Direction:        "upload"
						SampleIOInterval: "\(#rtt/2)ms"
					}},
				]},
			]},
		]
	}
}
