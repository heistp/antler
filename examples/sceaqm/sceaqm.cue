// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

// This Antler example config has tests of the FIFO, CoDel, COBALT, PIE and
// DelTiC AQMs. One cubic-sce stream is run from the left to right endpoint. A
// third of the way through the test, the rate drops from rate0 to rate1, then
// returns to rate0 two-thirds through the test.

package sceaqm

import (
	"list"
	"strings"
)

// rtt is the path RTT, in milliseconds
#rtt: 80

// rate0 is the initial rate
#rate0: "50mbit"

// rate1 is the rate after the drop
#rate1: "10mbit"

// quantum is the HTB quantum
#quantum: 1514

// duration is the test duration, in seconds
#duration: 120

// offloads contains the features arguments for ethtool to disable offloads
#offloads: "rx off tx off sg off tso off gso off gro off rxvlan off txvlan off"

// serverAddr is the server listen and client dial address
#serverAddr: "10.0.0.2:777"

// stream includes logs for streaming during the test
// This is passed to all nodes before setup.
#stream: {ResultStream: Include: Log: true}

// cakeyQdiscs are CAKE-like qdiscs with an embedded shaper and alternate
// tc syntax.
#cakeyQdiscs: ["cake", "cnq_codel_af", "cnq_cobalt", "twin_codel_af",
	"lfq_cobalt"]

// Run defines a serial list of qdisc Tests to run.
Run: {
	Serial: [
		for b in [ true, false]
		for q in [
				"pfifo limit 50",
				"pie",
				"codel",
				"cobalt",
				"cake sce flowblind",
				"cnq_cobalt sce sce-thresh 16",
				"deltic",
		] {{_qdisc: q, _bursty_udp: b} & _qdiscTest},
	]
}

// qdiscTest is the per-qdisc test
_qdiscTest: {
	// qdisc and bursty_udp are parameters for each TestRun
	_qdisc:      string
	_bursty_udp: bool

	// Test is the qdisc test
	Test: {
		// setup runs the setup commands in each namespace
		_setup: {
			Serial: [
				for n in [ _ns.right, _ns.mid, _ns.left] {
					Child: {
						Node: n.node
						Serial: [#stream, for c in n.setup {System: Command: c}]
					}
				},
			]
		}

		// server runs StreamServer in the right namespace
		_server: {
			Child: {
				Node: _ns.right.node
				Serial: [
					{System: {
						Command:    "tcpdump -i right.l -s 128 -w -"
						Background: true
						Stdout:     "right.pcap"
					}},
					{StreamServer: {ListenAddr: #serverAddr}},
					{PacketServer: {ListenAddr: #serverAddr}},
					{Sleep:                     "1s"},
				]
			}
		}

		// ID is the Test ID
		ID: {
			qdisc: strings.Fields(_qdisc)[0]
			cca:   _cca
			if _bursty_udp {
				udp: "bursty-udp"
			}
			if !_bursty_udp {
				udp: "no-udp"
			}
		}

		Serial: [#stream, _setup, _server, _do]
	}

	// Report defines reports for the qdisc test
	_cca: "cubic-sce"
	Report: [
		{ChartsTimeSeries: {
			To: ["timeseries.html"]
			FlowLabel: {
				"cubic-sce": "CUBIC-SCE"
				"udp":       "UDP OWD"
			}
			Options: {
				if !_bursty_udp {
					title: "\(FlowLabel[_cca]) Rate Drop \(#rate0) to \(#rate1) | \(#rtt)ms Path RTT | \(_qdisc) "
				}
				if _bursty_udp {
					title: "\(FlowLabel[_cca]) Rate Drop \(#rate0) to \(#rate1) w/ bursty UDP | \(#rtt)ms Path RTT | \(_qdisc) "
				}
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
					"1": {
						viewWindow: {
							min: #rtt / 2
							max: #rtt * 5
						}
						scaleType: "log"
					}
				}
			}
		}},
	]

	// qdiscSetup defines the commands for qdisc setup
	_qdiscSetup: {
		if list.Contains(#cakeyQdiscs, strings.Fields(_qdisc)[0]) {
			["tc-sce qdisc add dev mid.r root \(_qdisc) bandwidth \(#rate0)"]
		}
		if !list.Contains(#cakeyQdiscs, strings.Fields(_qdisc)[0]) {
			[
				"tc qdisc add dev mid.r root handle 1: htb default 1",
				"tc class add dev mid.r parent 1: classid 1:1 htb rate \(#rate0) quantum \(#quantum)",
				"tc qdisc add dev mid.r parent 1:1 \(_qdisc)",
			]
		}
	}

	// qdiscChange defines the commands for qdisc change
	_qdiscSetup: {
		if list.Contains(#cakeyQdiscs, strings.Split(_qdisc, " ")[0]) {
			["tc-sce qdisc add dev mid.r root \(_qdisc) bandwidth \(#rate0)"]
		}
		if !list.Contains(#cakeyQdiscs, strings.Split(_qdisc, " ")[0]) {
			[
				"tc qdisc add dev mid.r root handle 1: htb default 1",
				"tc class add dev mid.r parent 1: classid 1:1 htb rate \(#rate0) quantum \(#quantum)",
				"tc qdisc add dev mid.r parent 1:1 \(_qdisc)",
			]
		}
	}

	// ns defines the namespaces and their setup commands
	_ns: {
		right: {
			setup: [
				"sysctl -w net.ipv6.conf.all.disable_ipv6=1",
				"sysctl -w net.ipv4.tcp_sce=1",
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
			] + _qdiscSetup
		}
		left: {
			setup: [
				"sysctl -w net.ipv6.conf.all.disable_ipv6=1",
				"sysctl -w net.ipv4.tcp_ecn=1",
				"ip addr add 10.0.0.1/24 dev left.r",
				"ip link set left.r up",
				"ethtool -K left.r \(#offloads)",
				"ping -c 3 -i 0.1 10.0.0.2",
				"modprobe tcp_cubic_sce",
			]
		}
	}

	// ns template adds a node for each namespace, without having to define each
	_ns: [id=_]: node: {
		ID:       id
		Platform: "linux-amd64"
		Launcher: Local: {}
		Netns: {Create: true}
	}

	// do runs the test
	_do: {Parallel: [
		{Child: {
			Node: _ns.left.node
			Serial: [
				{System: {
					Command:    "tcpdump -i left.r -s 128 -w -"
					Background: true
					Stdout:     "left.pcap"
				}},
				{Sleep: "1s"},
				{Parallel: [
					if _bursty_udp {
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
						}}
					},
					{StreamClient: {
						Addr: #serverAddr
						Upload: {
							Flow:             "cubic-sce"
							CCA:              "cubic-sce"
							Duration:         "\(#duration)s"
							IOSampleInterval: "\(#rtt*4)ms"
						}
					}},
				]},
				{Sleep: "1s"},
			]
		}},
		{Child: {
			Node: _ns.mid.node
			Serial: [
				{Sleep: "\(#duration/3)s"},
				if list.Contains(#cakeyQdiscs, strings.Split(_qdisc, " ")[0]) {
					System: Command: "tc-sce qdisc change dev mid.r root \(_qdisc) bandwidth \(#rate1)"
				},
				if !list.Contains(#cakeyQdiscs, strings.Split(_qdisc, " ")[0]) {
					System: Command: "tc class change dev mid.r parent 1: classid 1:1 htb rate \(#rate1) quantum \(#quantum)"
				},
				{Sleep: "\(#duration/3)s"},
				if list.Contains(#cakeyQdiscs, strings.Split(_qdisc, " ")[0]) {
					System: Command: "tc-sce qdisc change dev mid.r root \(_qdisc) bandwidth \(#rate0)"
				},
				if !list.Contains(#cakeyQdiscs, strings.Split(_qdisc, " ")[0]) {
					System: Command: "tc class change dev mid.r parent 1: classid 1:1 htb rate \(#rate0) quantum \(#quantum)"
				},
			]
		}},
	]
	}

}
