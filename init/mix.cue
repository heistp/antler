package {{.Package}}

// _mix runs two TCP streams from the left to the right endpoint of a netns
// dumbbell, together with 8 packet UDP bursts every 10 ms.  The middlebox (mid
// namespace) has single queue CoDel added, without ECN.
_mix: {
	// _rtt is the path RTT, in milliseconds
	_rtt: 20

	// _duration is the test duration in seconds
	_duration: 60

	// _bandwidth is the bottleneck bandwidth in Mbps
	_bandwidth: 100

	// _qdisc is the qdisc to use
	_qdisc: "codel noecn"

	// ID is the Test ID.
	ID: name: "mix"

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
				"bbr":   "BBR"
				"udp":   "UDP"
			}
			Options: {
				title: "CUBIC vs BBR / \(_rtt)ms Path RTT / \(_qdisc)"
				series: {
					"0": {
						color: _dark2[0]
					}
					"1": {
						color: _dark2[1]
					}
					"2": {
						color: _dark2[2]
					}
					"3": {
						color: _dark2[3]
					}
					"4": {
						targetAxisIndex: 1
						lineWidth:       0
						pointSize:       0.2
						color:           _dark2[4]
					}
				}
				vAxes: {
					"0": viewWindow: {
						max: _bandwidth + 5
					}
					"1": viewWindow: {
						min: 0
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
		]
		mid: post: [
			"tc qdisc add dev mid.l root netem delay \(_rtt/2)ms limit 1000000",
			"ip link add dev imid.l type ifb",
			"tc qdisc add dev imid.l root handle 1: netem delay \(_rtt/2)ms limit 1000000",
			"tc qdisc add dev mid.l handle ffff: ingress",
			"ip link set dev imid.l up",
			"tc filter add dev mid.l parent ffff: protocol all prio 10 u32 match u32 0 0 flowid 1:1 action mirred egress redirect dev imid.l",
			"tc qdisc add dev mid.r root handle 1: htb default 1",
			"tc class add dev mid.r parent 1: classid 1:1 htb rate \(_bandwidth)mbit quantum 1514",
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
								Wait: [
									"0ms",
									"0ms",
									"0ms",
									"0ms",
									"0ms",
									"0ms",
									"0ms",
									"\(_rtt/2)ms",
								]
								Length: [1458]
								Duration: "\(_duration)s"
							}},
						]
					}},
					{StreamClient: {
						Addr: _rig.serverAddr
						Upload: {
							Flow:            "cubic"
							CCA:             "cubic"
							Duration:        "\(_duration)s"
							TCPInfoInterval: "\(_rtt*4)ms"
						}
					}},
					{Serial: [
						{Sleep: "\(_duration/3)s"},
						{StreamClient: {
							Addr: _rig.serverAddr
							Upload: {
								Flow:            "bbr"
								CCA:             "bbr"
								Duration:        "\(_duration/3)s"
								TCPInfoInterval: "\(_rtt*4)ms"
							}
						}},
					]},
				]},
				{Sleep: "1s"},
			]
		}
	}
}
