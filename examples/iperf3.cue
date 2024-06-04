// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

package examples

// _iperf3 uses iperf3 to test a stream from the left to the right endpoint,
// demonstrating how Antler can use external testing tools. The middlebox
// (mid namespace) has the cake qdisc added at 50 Mbps.
_iperf3: {
	// _rtt is the path RTT, in milliseconds
	_rtt: 80

	// _rate is the bitrate, in Mbps
	_rate: 50

	// Name is the name of the Group.
	Name: "iperf3"

	Test: [{
		Serial: [
			_rig.setup,
			_server,
			_do,
		]
	}]

	// _rig defines the dumbbell Test setup.
	_rig: _dumbbell & {
		serverAddr: "\(right.addr):777"
		left: post: [
		]
		mid: post: [
			"ip link add dev imid.l type ifb",
			"tc qdisc add dev imid.l root handle 1: netem delay \(_rtt)ms limit 1000000",
			"tc qdisc add dev mid.l handle ffff: ingress",
			"ip link set dev imid.l up",
			"tc filter add dev mid.l parent ffff: protocol all prio 10 u32 match u32 0 0 flowid 1:1 action mirred egress redirect dev imid.l",
			"tc qdisc add dev mid.r root cake bandwidth \(_rate)Mbit",
		]
		right: post: [
		]
	}

	// _server runs the iperf3 server in the right namespace
	_server: {
		Child: {
			Node: _rig.right.node
			System: {
				Command:    "iperf3 -s -1"
				Background: true
			}
		}
	}

	// _do runs the test using iperf3 from the left namespace to the right
	_do: {
		Child: {
			Node: _rig.left.node
			Serial: [
				_tcpdump & {_iface: "left.r"},
				{Sleep:             "1s"},
				{System: Command:   "iperf3 -t 30 -c \(_dumbbell.right.addr)"},
				{Sleep:             "1s"},
			]
		}
	}
}
