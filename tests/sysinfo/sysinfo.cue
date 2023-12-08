// SPDX-License-Identifier: GPL-3.0
// Copyright 2023 Pete Heist

// This Antler example tests gathering system information in the root node.

package sysinfo

Run: {
	Test: {
		// SysInfo gathers system information.
		SysInfo: {
			Command: [
				{Command: "uname -a"},
				{Command: "git -C /home/heistp/src/linux/sce show --summary"},
			]
			File: [
				{Name: "/proc/cmdline"},
				{Name: "/proc/cpuinfo"},
				{Name: "/proc/version"},
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
			{EmitSysInfo: {
			}},
		]
	}
}
