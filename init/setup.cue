package {{.Package}}

// _platform sets the node platform used for all tests (must match the local
// machine).
_platform: "linux-amd64"

// _stream selects what is streamed from nodes during tests.
_stream: {ResultStream: Include: Log: true}

// _sysinfo selects what system information is retrieved.
_sysinfo: {
	// SysInfo gathers system information.
	SysInfo: {
		OS: {
			Command: {Command: "uname -a"}
		}
		Command: [
			{Command: "lscpu"},
			{Command: "lshw -sanitize"},
		]
		File: [
			"/proc/cmdline",
			"/sys/devices/system/clocksource/clocksource0/available_clocksource",
			"/sys/devices/system/clocksource/clocksource0/current_clocksource",
		]
		Sysctl: [
			"^net\\.core\\.",
			"^net\\.ipv4\\.tcp_",
			"^net\\.ipv4\\.udp_",
		]
	}
}

// _offloads contains the ethtool arguments for offloads config.
_offloads: "rx off tx off sg off tso off gso off gro off rxvlan off txvlan off"

// _netnsNode defines common fields for a netns node.
_netnsNode: {
	ID:       string & !=""
	Platform: _platform
	Launcher: Local: {}
	Netns: {Create: true}
}

// _dumbbell defines setup commands for a standard three-node dumbbell, with
// nodes left, mid and right.
_dumbbell: {
	setup: {
		Serial: [
			_stream,
			_sysinfo,
			for n in [ right, mid, left] {
				Child: {
					Node: n.node
					Serial: [
						_stream,
						for c in n.setup {System: Command: c},
					]
				}
			},
		]
	}

	right: {
		post: [...string]
		node:  _netnsNode & {ID: "right"}
		addr:  "10.0.0.2"
		setup: [
			"sysctl -w net.ipv6.conf.all.disable_ipv6=1",
			"ip link add dev right.l type veth peer name mid.r",
			"ip link set dev mid.r netns mid",
			"ip addr add \(addr)/24 dev right.l",
			"ip link set right.l up",
			"ethtool -K right.l \(_offloads)",
		] + post
	}

	mid: {
		post: [...string]
		node:  _netnsNode & {ID: "mid"}
		setup: [
			"sysctl -w net.ipv6.conf.all.disable_ipv6=1",
			"ip link set mid.r up",
			"ip link add dev mid.l type veth peer name left.r",
			"ip link set dev left.r netns left",
			"ip link set dev mid.l up",
			"ip link add name mid.b type bridge",
			"ip link set dev mid.r master mid.b",
			"ip link set dev mid.l master mid.b",
			"ip link set dev mid.b up",
			"ethtool -K mid.r \(_offloads)",
			"ethtool -K mid.l \(_offloads)",
		] + post
	}

	left: {
		post: [...string]
		node:  _netnsNode & {ID: "left"}
		addr:  "10.0.0.1"
		setup: [
			"sysctl -w net.ipv6.conf.all.disable_ipv6=1",
			"ip addr add \(addr)/24 dev left.r",
			"ip link set left.r up",
			"ethtool -K left.r \(_offloads)",
			"ping -c 3 -i 0.1 \(_dumbbell.right.addr)",
		] + post
	}
}
