// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

// This CUE file defines the Antler configuration schema. For documentation,
// see the corresponding types referenced in the comments.

package antler

// top-level embedded antler.TestRun
Run: #TestRun

//
// antler package
//

// antler.TestRun
#TestRun: {
	// the syntax below ensures only one of Test, Serial or Parallel are set.
	{} | {
		Test?: #Test
	} | {
		Serial?: [#TestRun, ...#TestRun]
	} | {
		Parallel?: [#TestRun, ...#TestRun]
	}
	Report?: [#Report, ...]
}

// antler.Report
#Report: {
	{} | {
		EmitLog?: #EmitLog
	} | {
		SaveFiles?: #SaveFiles
	}
}

// antler.EmitLog
#EmitLog: {
	To?: [string & !="", ...]
}

// antler.SaveFiles
#SaveFiles: {
}

// antler.Test
#Test: {
	ID: {...}
	OutputPath: string & !="" | *"./"
	#Run
}

//
// node package
//

// node.Run
#Run: {
	{} | {
		#Runners
	} | {
		Serial?: [#Run, ...#Run]
	} | {
		Parallel?: [#Run, ...#Run]
	} | {
		Child?: #Child
	}
}

// node.Runners
#Runners: {
	{} | {
		Sleep?: #Duration
	} | {
		ResultStream?: #ResultStream
	} | {
		System?: #System
	} | {
		TCPStreamClient?: #TCPStreamClient
	} | {
		TCPStreamServer?: #TCPStreamServer
	}
}

// node.Duration
#Duration: string & =~"^[0-9]+(ns|us|µs|ms|s|m|h)$"

// node.Series
#Series: string & !=""

// node.ResultStream
#ResultStream: {
	Include?: #MessageFilter
	Exclude?: #MessageFilter
}

// node.MessageFilter
#MessageFilter: {
	File?: [string, ...]
	Log?:    bool
	Series?: #Series
}

// node.System
#System: {
	Command?: string & !=""
	Args?: [string, ...]
	Background?:   bool
	IgnoreErrors?: bool
	Stdout?:       string & !=""
	Stderr?:       string & !=""
	Kill?:         bool
}

// node.TCPStream
#TCPStream: {
	Series:       #Series
	Download?:    bool
	CCA?:         string & !=""
	Duration?:    #Duration | *"1m"
	Interval?:    #Duration | *"50ms"
	ReadBufLen?:  int | *16384
	WriteBufLen?: int | *16384
}

// node.TCPStreamClient
#TCPStreamClient: {
	{} | {
		Addr?: string & !=""
	} | {
		AddrKey?: string & !=""
	}
	#TCPStream
}

// node.TCPStreamServer
#TCPStreamServer: {
	{} | {
		ListenAddr?: string & !=""
	} | {
		ListenAddrKey?: string & !=""
	}
	#TCPStream
}

// node.Child
#Child: {
	Node: #Node
	#Run
}

// node.Node
#Node: {
	ID:       string & !=""
	Platform: string & !=""
	Launcher: #Launchers
	Netns?:   #Netns
}

// node.launchers
#Launchers: {
	{} | {
		SSH?: {Destination?: string & !=""} // node.SSH
	} | {
		Local?: {} // node.Local
	}
}

// node.Netns
#Netns: {
	Name?:   string & !=""
	Create?: bool
}
