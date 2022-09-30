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
	Report?: [#Report, ...#Report]
}

// antler.Report
#Report: {
	{} | {
		EmitLog?: #EmitLog
	} | {
		ExecuteTemplate?: #ExecuteTemplate
	} | {
		ChartsTimeSeries?: #ChartsTimeSeries
	} | {
		SaveFiles?: #SaveFiles
	}
}

// antler.EmitLog
#EmitLog: {
	To?: [string & !="", ...]
}

// antler.ExecuteTemplate
#ExecuteTemplate: {
	Name?: string & !=""
	To:    string & !=""
	{} | {
		Text?: string & !=""
	} | {
		From?: [string & !="", ...string & !=""]
	}
}

// antler.ChartsTimeSeries
#ChartsTimeSeries: {
	Title:  string & !="" | *"Time Series"
	VTitle: string & !="" | *"Goodput (Mbps)"
	VMin:   int | *0
	VMax?:  int
	FlowLabel: {
		[=~".*"]: string
	}
	To: string & !=""
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
		StreamClient?: #StreamClient
	} | {
		StreamServer?: #StreamServer
	}
}

// node.Duration
#Duration: string & =~"^[0-9]+(ns|us|µs|ms|s|m|h)$"

// node.Flow
#Flow: string & !=""

// node.ResultStream
#ResultStream: {
	Include?: #MessageFilter
	Exclude?: #MessageFilter
}

// node.MessageFilter
#MessageFilter: {
	File?: [string, ...string]
	Log?:  bool
	Flow?: #Flow
}

// node.System
#System: {
	Command?: string & !=""
	Args?: [string, ...string]
	Background?:   bool
	IgnoreErrors?: bool
	Stdout?:       string & !=""
	Stderr?:       string & !=""
	Kill?:         bool
}

// node.Stream
#Stream: {
	Flow?:            #Flow
	Direction:        #Direction | *"upload"
	Duration:         #Duration | *"1m"
	CCA?:             string & !=""
	SampleIOInterval: #Duration | *"10ms"
	BufLen:           int & >0 | *(1024 * 128)
}

// node.Direction
#Direction: "upload" | "download"

// node.StreamClient
#StreamClient: {
	{} | {
		Addr?: string & !=""
	} | {
		AddrKey?: string & !=""
	}
	#Stream
}

// node.StreamServer
#StreamServer: {
	{} | {
		ListenAddr?: string & !=""
	} | {
		ListenAddrKey?: string & !=""
	}
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
