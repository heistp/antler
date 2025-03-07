// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2023 Pete Heist

// This Antler test gathers system information in the root node.

package sysinfo

// Test contains a single Test that tests system information.
Test: [{
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

	// disable saving of gob data
	DataFile: ""
	// remove default reports
	AfterDefault: []
	// add just SysInfo report
	After: [
		{EmitSysInfo: {}},
	]
}]
