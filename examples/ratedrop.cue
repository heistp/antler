// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2023 Pete Heist

package examples

// _ratedrop tests a drop in the bottleneck rate while a single BBR stream runs
// from the left to the right endpoint. A third of the way through the test,
// the rate drops from rate0 to rate1. Two-thirds through the test, the rate
// returns to rate0.
_ratedrop: {
	// _duration is the test duration, in seconds
	_duration: 120

	// _rtt is the path RTT, in milliseconds
	_rtt: 80

	// _rate0 is the initial rate
	_rate0: "50mbit"

	// _rate1 is the rate after the drop
	_rate1: "10mbit"

	// _quantum is the HTB quantum
	_quantum: 1514

	// _qdisc is the qdisc to apply
	_qdisc: "fq_codel flows 1 noecn"

	// ID is the Test ID.
	ID: name: "ratedrop"

	Serial: [
		_rig.setup,
		_server,
		_do,
	]

	After: [
		{Analyze: {}},
		{ChartsTimeSeries: {
			To: ["timeseries.html"]
			FlowLabel: {
				"bbr": "BBR"
				"udp": "UDP"
			}
			Options: {
				title: "BBR Rate Drop \(_rate0) to \(_rate1) | \(_rtt)ms Path RTT | \(_qdisc) "
				series: {
					"0": {
						color: _dark2[0]
					}
					"1": {
						targetAxisIndex: 1
						lineWidth:       0
						pointSize:       0.2
						color:           _dark2[1]
					}
				}
				vAxes: {
					"0": viewWindow: {
						max: 55
					}
					"1": viewWindow: {
						min: _rtt / 2
						max: _rtt * 10
					}
				}
			}
		}},
	]

	// _rig defines the dumbbell Test setup.
	_rig: _dumbbell & {
		serverAddr: "\(right.addr):7777"
		left: post: [
			"sysctl -w net.ipv4.tcp_ecn=0",
			"modprobe tcp_bbr",
		]
		mid: post: [
			"tc qdisc add dev mid.l root netem delay \(_rtt/2)ms limit 1000000",
			"ip link add dev imid.l type ifb",
			"tc qdisc add dev imid.l root handle 1: netem delay \(_rtt/2)ms limit 1000000",
			"tc qdisc add dev mid.l handle ffff: ingress",
			"ip link set dev imid.l up",
			"tc filter add dev mid.l parent ffff: protocol all prio 10 u32 match u32 0 0 flowid 1:1 action mirred egress redirect dev imid.l",
			"tc qdisc add dev mid.r root handle 1: htb default 1",
			"tc class add dev mid.r parent 1: classid 1:1 htb rate \(_rate0) quantum \(_quantum)",
			"tc qdisc add dev mid.r parent 1:1 \(_qdisc)",
		]
		right: post: []
	}

	// _server runs StreamServer in the right namespace
	_server: {
		Child: {
			Node: _rig.right.node
			Serial: [
				_tcpdump & {_iface:         "right.l"},
				{StreamServer: {ListenAddr: _rig.serverAddr}},
				{PacketServer: {ListenAddr: _rig.serverAddr}},
			]
		}
	}

	// _do runs the test
	_do: {
		{Parallel: [
			{Child: {
				Node: _rig.left.node
				Serial: [
					_tcpdump & {_iface: "left.r"},
					{Sleep:             "1s"},
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
						}},
						{StreamClient: {
							Addr: _rig.serverAddr
							Upload: {
								Flow:             "bbr"
								CCA:              "bbr"
								Duration:         "\(_duration)s"
								IOSampleInterval: "\(_rtt*8)ms"
							}
						}},
					]},
					{Sleep: "1s"},
				]
			}},
			{Child: {
				Node: _rig.mid.node
				Serial: [
					{Sleep: "\(div(_duration, 3))s"},
					{System: Command:
						"tc class change dev mid.r parent 1: classid 1:1 htb rate \(_rate1) quantum \(_quantum)"
					},
					{Sleep: "\(div(_duration, 3))s"},
					{System: Command:
						"tc class change dev mid.r parent 1: classid 1:1 htb rate \(_rate0) quantum \(_quantum)"
					},
				]
			}},
		]}
	}
}
