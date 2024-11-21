// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

package examples

// _tcpstream runs two TCP streams from the left to the right endpoint of a
// netns dumbbell. The middlebox (mid namespace) has the cake qdisc added, at a
// rate of 50 Mbps.
_tcpstream: {
	// _rtt is the path RTT, in milliseconds
	_rtt: 80

	// _duration is the test duration in seconds
	_duration: 60

	// _qdisc is the qdisc to use
	_qdisc: "cake bandwidth 50Mbit flowblind"

	// ID is the Test ID.
	ID: name: "tcpstream"

	Serial: [
		_rig.setup,
		_server,
		_do,
	]

	// After is the report pipeline for the Test.
	After: [
		{Analyze: {}},
		{ChartsTimeSeries: {
			To: ["timeseries.html"]
			FlowLabel: {
				"cubic": "CUBIC"
				"reno":  "Reno"
				"udp":   "UDP"
			}
			Options: {
				title: "CUBIC vs Reno / \(_rtt)ms Path RTT / \(_qdisc)"
				series: {
					"0": {
						color: _dark2[0]
					}
					"1": {
						color: _dark2[1]
					}
					"2": {
						targetAxisIndex: 1
						lineWidth:       0
						pointSize:       0.2
						color:           _dark2[2]
					}
				}
				vAxes: {
					"0": viewWindow: {
						max: 55
					}
					"1": viewWindow: {
						min: _rtt / 2
						max: _rtt * 2
					}
				}
			}
		}},
	]

	// _rig defines the dumbbell Test setup.
	_rig: _dumbbell & {
		serverAddr: "\(right.addr):777"
		left: post: [
			"sysctl -w net.ipv4.tcp_ecn=1",
		]
		mid: post: [
			"tc qdisc add dev mid.l root netem delay \(_rtt/2)ms limit 1000000",
			"ip link add dev imid.l type ifb",
			"tc qdisc add dev imid.l root handle 1: netem delay \(_rtt/2)ms limit 1000000",
			"tc qdisc add dev mid.l handle ffff: ingress",
			"ip link set dev imid.l up",
			"tc filter add dev mid.l parent ffff: protocol all prio 10 u32 match u32 0 0 flowid 1:1 action mirred egress redirect dev imid.l",
			"tc qdisc add dev mid.r root \(_qdisc)",
		]
		right: post: []
	}

	// _server runs StreamServer in the right namespace
	_server: {
		Child: {
			Node: _rig.right.node
			Serial: [
				_tcpdump & {_iface: "right.l"},
				{StreamServer: {ListenAddr: _rig.serverAddr}},
				{PacketServer: {ListenAddr: _rig.serverAddr}},
			]
		}
	}

	// _do runs the test using two StreamClients
	_do: {
		Child: {
			Node: _rig.left.node
			Serial: [
				_tcpdump & {_iface: "left.r"},
				{Sleep: "1s"},
				{Parallel: [
					{PacketClient: {
						Addr: _rig.serverAddr
						Flow: "udp"
						Sender: [
							{Unresponsive: {
								Wait: ["\(_rtt/2)ms"]
								Duration: "\(_duration)s"
							}},
						]
						DSCP: 10 // AF11
						ECN:  2  // ECT(0)
					}},
					{StreamClient: {
						Addr: _rig.serverAddr
						Upload: {
							Flow:             "cubic"
							CCA:              "cubic"
							Duration:         "\(_duration)s"
							IOSampleInterval: "\(_rtt*4)ms"
						}
					}},
					{Serial: [
						{Sleep: "\(div(_duration, 3))s"},
						{StreamClient: {
							Addr: _rig.serverAddr
							Upload: {
								Flow:             "reno"
								CCA:              "reno"
								Duration:         "\(div(_duration, 3))s"
								IOSampleInterval: "\(_rtt*4)ms"
							}
						}},
					]},
				]},
				{Sleep: "1s"},
			]
		}
	}
}
