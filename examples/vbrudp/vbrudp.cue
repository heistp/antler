// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

// This Antler example config has a single test that creates a netns dumbbell,
// and runs one TCP stream and one VBR UDP stream from the left to the right
// endpoint. The middlebox (mid namespace) has the cake qdisc added at 20 Mbit.
//
// The VBR UDP flow has two concurrent packet generators, one approximating a
// 64 Kbps VoIP codec with 160 byte UDP packets, and one approximating
// a 1.15 Mbps HD realtime video codec, with 8 packet bursts of 900 bytes
// every 50 ms.

package vbrudp

// stream includes logs for streaming during the test
// This is passed to all nodes before setup.
stream: {ResultStream: Include: Log: true}

// rtt is the path RTT, in milliseconds
#rtt: 20

// duration is the test duration, in seconds
#duration: 60

// rate is the shaper rate
#rate: "20Mbit"

// qdisc is the qdisc to apply
#qdisc: "codel"
//#qdisc: "cake bandwidth \(#rate) flowblind"

// Run contains a single Test. After log streaming is configured, setup is
// run, the server is started, then the test is run.
Run: {
	Test: {
		ID: {"Name": "vbrudp"}
		Serial: [stream, setup, server, do]
	}
	Report: [
		{EmitLog: {To: ["node.log", "-"]}},
		{SaveFiles: {}},
		{ChartsTimeSeries: {
			To: ["timeseries.html"]
			FlowLabel: {
				"bbr":   "BBR Goodput"
				"cubic": "CUBIC Goodput"
				"udp":   "UDP OWD"
			}
			Options: {
				title: "CUBIC vs Reno | \(#rate) bottleneck | \(#rtt)ms Path RTT | \(#qdisc)"
				series: {
					"2": {
						targetAxisIndex: 1
						lineWidth:       0
						pointSize:       0.2
						color:           "#4f9634"
					}
				}
				vAxes: {
					"0": viewWindow: {
						max: 25
					}
					"1": viewWindow: {
						min: #rtt / 2
						max: #rtt * 4
					}
				}
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
			//"tc qdisc add dev mid.r root \(#qdisc)",
			"tc qdisc add dev mid.r root handle 1: htb default 1",
			"tc class add dev mid.r parent 1: classid 1:1 htb rate \(#rate)",
			"tc qdisc add dev mid.r parent 1:1 \(#qdisc)",
		]
	}
	left: {
		setup: [
			"sysctl -w net.ipv6.conf.all.disable_ipv6=1",
			"sysctl -w net.ipv4.tcp_ecn=0",
			"ip addr add 10.0.0.1/24 dev left.r",
			"ip link set left.r up",
			"ethtool -K left.r \(#offloads)",
			"ping -c 3 -i 0.1 10.0.0.2",
			"modprobe tcp_bbr",
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

// do runs the test using two StreamClients
do: {
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
							Wait: ["20ms"]
							Length: [160]
							Duration: "\(#duration)s"
						}},
						{Unresponsive: {
							Wait: ["0ms", "0ms", "0ms", "0ms", "0ms", "0ms", "0ms", "50ms"]
							Length: [900]
							Duration: "\(#duration)s"
						}},
					]
				}},
				{Serial: [
					{Sleep: "\(#duration/5)s"},
					{StreamClient: {
						Addr: #serverAddr
						Upload: {
							Flow:             "bbr"
							CCA:              "bbr"
							Duration:         "\(#duration*2/5)s"
							SampleIOInterval: "500ms"
						}
					}},
				]},
				{Serial: [
					{Sleep: "\(#duration*2/5)s"},
					{StreamClient: {
						Addr: #serverAddr
						Upload: {
							Flow:             "cubic"
							CCA:              "cubic"
							Duration:         "\(#duration*2/5)s"
							SampleIOInterval: "250ms"
						}
					}},
				]},
			]},
		]
	}
}
