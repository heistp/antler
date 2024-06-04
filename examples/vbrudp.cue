// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

package examples

// _vbrudp runs one TCP stream and one VBR UDP stream from the left to the
// right endpoint. The middlebox (mid namespace) has the cake qdisc added at
// 20 Mbit.
//
// The VBR UDP flow has two concurrent packet generators, one approximating a
// 64 Kbps VoIP codec with 160 byte UDP packets, and one approximating
// a 1.15 Mbps HD realtime video codec, with 8 packet bursts of 900 bytes
// every 50 ms.
_vbrudp: {
	// _rtt is the path RTT, in milliseconds
	_rtt: 20

	// _duration is the test duration, in seconds
	_duration: 60

	// _rate is the shaper rate
	_rate: "20Mbit"

	// _qdisc is the qdisc to apply
	_qdisc: "codel"

	// Name is the name of the Group.
	Name: "vbrudp"

	Test: [{
		Serial: [
			_rig.setup,
			_server,
			_do,
		]
	}]

	After: [
		{Analyze: {}},
		{ChartsTimeSeries: {
			To: ["timeseries.html"]
			FlowLabel: {
				"bbr":   "BBR Goodput"
				"cubic": "CUBIC Goodput"
				"udp":   "UDP OWD"
			}
			Options: {
				title: "CUBIC vs BBR | \(_rate) bottleneck | \(_rtt)ms Path RTT | \(_qdisc)"
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
						max: 25
					}
					"1": viewWindow: {
						min: _rtt / 2
						max: _rtt * 4
					}
				}
			}
		}},
	]

	// _rig defines the dumbbell Test setup.
	_rig: _dumbbell & {
		serverAddr: "\(right.addr):777"
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
			"tc class add dev mid.r parent 1: classid 1:1 htb rate \(_rate) quantum 1514",
			"tc qdisc add dev mid.r parent 1:1 \(_qdisc)",
		]
		right: post: [
		]
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

	// _do runs the test using two StreamClients
	_do: {
		Child: {
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
								Wait: ["20ms"]
								Length: [160]
								Duration: "\(_duration)s"
							}},
							{Unresponsive: {
								Wait: ["0ms", "0ms", "0ms", "0ms", "0ms", "0ms", "0ms", "50ms"]
								Length: [900]
								Duration: "\(_duration)s"
							}},
						]
					}},
					{Serial: [
						{Sleep: "\(_duration/5)s"},
						{StreamClient: {
							Addr: _rig.serverAddr
							Upload: {
								Flow:             "bbr"
								CCA:              "bbr"
								Duration:         "\(_duration*2/5)s"
								IOSampleInterval: "500ms"
							}
						}},
					]},
					{Serial: [
						{Sleep: "\(_duration*2/5)s"},
						{StreamClient: {
							Addr: _rig.serverAddr
							Upload: {
								Flow:             "cubic"
								CCA:              "cubic"
								Duration:         "\(_duration*2/5)s"
								IOSampleInterval: "250ms"
							}
						}},
					]},
				]},
				{Sleep: "1s"},
			]
		}
	}
}
