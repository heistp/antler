// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

// This Antler example config has a single test that creates a netns dumbbell
// with a middlebox running codel (fq_codel flows 1). One BBR stream is run
// from the left to right endpoint. A third of the way through the test, the
// rate drops from rate0 to rate1. Two-thirds through the test, the rate
// returns to rate0.

package ratedrop

// stream includes logs for streaming during the test
// This is passed to all nodes before setup.
#stream: {ResultStream: Include: Log: true}

// rtt is the path RTT, in milliseconds
#rtt: 80

// rate0 is the initial rate
#rate0: "50mbit"

// rate1 is the rate after the drop
#rate1: "10mbit"

// duration is the test duration, in seconds
#duration: 120

// qdisc is the qdisc to apply
#qdisc: "fq_codel flows 1 noecn"

// Run contains a single Test. After log streaming is configured, setup is
// run, the server is started, then the test is run.
Run: {
	Test: {
		ID: {"Name": "ratedrop"}
		Serial: [#stream, #setup, server, do]
	}
	Report: [
		{EmitLog: {To: ["node.log", "-"]}},
		{SaveFiles: {}},
		{ChartsTimeSeries: {
			To: ["timeseries.html"]
			FlowLabel: {
				"bbr": "BBR Goodput"
				"udp": "UDP OWD"
			}
			Options: {
				title: "BBR Rate Drop \(#rate0) to \(#rate1) | \(#rtt)ms Path RTT | \(#qdisc) "
				series: {
					"1": {
						targetAxisIndex: 1
						lineWidth:       0
						pointSize:       0.2
						color:           "#4f9634"
					}
				}
				vAxes: {
					"0": viewWindow: {
						max: 55
					}
					"1": viewWindow: {
						min: #rtt / 2
						max: #rtt * 10
					}
				}
			}
		}},
	]
}

// setup runs the setup commands in each namespace
#setup: {
	Serial: [
		for n in [ ns.right, ns.mid, ns.left] {
			Child: {
				Node: n.node
				Serial: [#stream, for c in n.setup {System: Command: c}]
			}
		},
	]
}

// ns defines the namespaces and their setup commands
ns: {
	right: {
		setup: [
			"sysctl -w net.ipv6.conf.all.disable_ipv6=1",
			"sysctl -w net.ipv4.tcp_ecn=0",
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
			"tc qdisc add dev mid.r root handle 1: htb default 1",
			"tc class add dev mid.r parent 1: classid 1:1 htb rate \(#rate0)",
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

// do runs the test
do: {
	{Parallel: [
		{Child: {
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
								Interval: "\(#rtt/2)ms"
								Duration: "\(#duration)s"
							}},
						]
					}},
					{StreamClient: {
						Addr: #serverAddr
						Upload: {
							Flow:             "bbr"
							CCA:              "bbr"
							Duration:         "\(#duration)s"
							SampleIOInterval: "\(#rtt*8)ms"
						}
					}},
				]},
			]
		}},
		{Child: {
			Node: ns.mid.node
			Serial: [
				{Sleep: "\(#duration/3)s"},
				{System: Command:
					"tc class change dev mid.r parent 1: classid 1:1 htb rate \(#rate1)"
				},
				{Sleep: "\(#duration/3)s"},
				{System: Command:
					"tc class change dev mid.r parent 1: classid 1:1 htb rate \(#rate0)"
				},
			]
		}},
	]}
}
