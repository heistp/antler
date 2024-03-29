// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

// This Antler example tests gathering system information in the root node.

package sysinfo

Run: {
	Test: {
		// SysInfo gathers system information.
		SysInfo: {
			OS: {
				Command: {Command: "uname -a"}
			}
			//KernSrcInfo: {
			//	Command: {Command: "git -C /usr/local/src/sce show --summary"}
			//}
			//KernSrcVer: {
			//	Command: {Command: "git -C /usr/local/src/sce show HEAD --pretty=format:%h --no-patch"}
			//}
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
		// remove default report that writes node.log
		Report: [
			{EmitSysInfo: {}},
		]
	}
}
