// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2024 Pete Heist

package examples

// _packets runs a single isochronous UDP flow from the left to the right
// endpoint, with a netem instance in the middlebox that delays, induces jitter,
// drops and re-orders packets, in order to demonstrate stats for packet flows.
_packets: {
	// _delay is the netem induced delay
	_delay: 20

	// _jitter is the netem induced jitter
	_jitter: 2.5

	// _duration is the test duration, in seconds
	_duration: 10

	// _qdisc is the qdisc to apply
	_qdisc: "netem delay \(_delay)ms \(_jitter)ms distribution pareto loss random 1% duplicate 1%"

	// ID is the Test ID.
	ID: name: "packets"

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
				"udp": "UDP"
			}
			Options: {
				title: "UDP flow | \(_qdisc)"
				series: {
					"0": {
						targetAxisIndex: 0
						lineWidth:       0
						pointSize:       0.2
						color:           _dark2[0]
					}
				}
				vAxes: {
					"0": {
						title: "Delay (ms)"
						viewWindow: {
							max: _delay * 2
						}
					}
				}
			}
		}},
	]

	// _rig defines the dumbbell Test setup.
	_rig: _dumbbell & {
		serverAddr: "\(right.addr):7777"
		left: post: [
		]
		mid: post: [
			"tc qdisc add dev mid.r root \(_qdisc)",
		]
		right: post: [
		]
	}

	// _server runs PacketServer in the right namespace
	_server: {
		Child: {
			Node: _rig.right.node
			Serial: [
				_tcpdump & {_iface:         "right.l"},
				{PacketServer: {ListenAddr: _rig.serverAddr}},
			]
		}
	}

	// _do runs the test using the PacketClient
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
								Echo: true
								Wait: ["10ms"]
								Length: [160]
								Duration: "\(_duration)s"
							}},
						]
					}},
				]},
			]
		}
	}
}
