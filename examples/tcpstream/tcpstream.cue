// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

// This Antler example config has a single test that creates a netns dumbbell,
// and runs two TCP streams from the left to the right endpoint. The middlebox
// (mid namespace) has the cake qdisc added at 50 Mbit.

package tcpstream

// stream includes logs for streaming during the test
// This is passed to all nodes before setup.
stream: {ResultStream: Include: Log: true}

// rtt is the path RTT, in milliseconds
#rtt: 80

// duration is the test duration, in seconds
#duration: 120

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
		{ChartsTimeSeries: {
			To: "throughput.html"
			FlowLabel: {
				"cubic": "TCP CUBIC"
				"reno":  "TCP Reno"
			}
			Options: {
				title: "CUBIC vs Reno Goodput / \(#qdisc) / \(#rtt)ms RTT"
				vAxis: viewWindow: max: 55
			}
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
			"ethtool -K mid.r \(#offloads)",
			"ip link add dev mid.l type veth peer name left.r",
			"ip link set dev left.r netns left",
			"ip link set dev mid.l up",
			"ethtool -K mid.l \(#offloads)",
			"ip link add name mid.b type bridge",
			"ip link set dev mid.r master mid.b",
			"ip link set dev mid.l master mid.b",
			"ip link set dev mid.b up",
			"tc qdisc add dev mid.l root netem delay \(#rtt/2)ms limit 1000000",
			"ip link add dev imid.l type ifb",
			"tc qdisc add dev imid.l root handle 1: netem delay \(#rtt/2)ms limit 1000000",
			"tc qdisc add dev mid.l handle ffff: ingress",
			"ip link set dev imid.l up",
			"tc filter add dev mid.l parent ffff: protocol all prio 10 u32 match u32 0 0 flowid 1:1 action mirred egress redirect dev imid.l",
			"tc qdisc add dev mid.r root cake bandwidth 50Mbit",
		]
	}
	left: {
		setup: [
			"sysctl -w net.ipv6.conf.all.disable_ipv6=1",
			"ip addr add 10.0.0.1/24 dev left.r",
			"ip link set left.r up",
			"ethtool -K left.r \(#offloads)",
			"ping -c 3 -i 0.1 10.0.0.2",
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
			{PacketServer: {ListenAddr: #serverAddr}},
		]
	}
}

// run runs the test using two StreamClients
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
				{PacketClient: {
					Addr: #serverAddr
					Flow: "udp"
					Sender: [
						{Unresponsive: {
							Duration: "\(#duration)s"
						}},
					]
				}},
				{StreamClient: {
					Addr: #serverAddr
					Upload: {
						Flow:             "cubic"
						CCA:              "cubic"
						Duration:         "\(#duration)s"
						SampleIOInterval: "\(#rtt*4)ms"
					}
				}},
				{Serial: [
					{Sleep: "\(#duration/3)s"},
					{StreamClient: {
						Addr: #serverAddr
						Upload: {
							Flow:             "reno"
							CCA:              "reno"
							Duration:         "\(#duration/3)s"
							SampleIOInterval: "\(#rtt*4)ms"
						}
					}},
				]},
			]},
		]
	}
}
