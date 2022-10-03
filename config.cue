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
		PacketClient?: #PacketClient
	} | {
		PacketServer?: #PacketServer
	} | {
		StreamClient?: #StreamClient
	} | {
		StreamServer?: #StreamServer
	}
}

// node.Duration
#Duration: string & =~"^[0-9]+(ns|us|µs|ms|s|m|h)$"

// node.Flow (TODO restrict length to <=255 chars)
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
	Flow:      #Flow
	Direction: #Direction
	CCA?:      string & !=""
}

// node.Direction
#Direction: "up" | "down"

// node.PacketClient (TODO min MaxPacketSize is arbitrary)
#PacketClient: {
	Addr:          string & !=""
	Protocol:      #PacketProtocol
	Flow:          #Flow
	MaxPacketSize: #MaxPacketSize
	Sender: [#PacketSenders, ...#PacketSenders]
}

// MaxPacketSize is the maximum size for PacketClient/PacketServer
#MaxPacketSize: int & >=32 & *(1500 - 20)

// node.PacketSenders
#PacketSenders: {
	{} | {
		Unresponsive?: #Unresponsive
	}
}

// node.Unresponsive
#Unresponsive: {
	Interval: #Duration | *"100ms"
	Length?:  int
	Duration: #Duration
	Echo:     bool | *true
}

// node.PacketProtocol
#PacketProtocol: *"udp" | "udp4" | "udp6"

// node.PacketServer
#PacketServer: {
	ListenAddr:    string
	Protocol:      #PacketProtocol
	MaxPacketSize: #MaxPacketSize
}

// node.StreamClient
#StreamClient: {
	{} | {
		Addr?: string & !=""
	} | {
		AddrKey?: string & !=""
	}
	Protocol: #StreamProtocol
	#Streamers
}

// node.streamers
#Streamers: {
	{} | {
		Upload?: #Upload
	} | {
		Download?: #Download
	}
}

// node.Upload
#Upload: {
	#Transfer
	Direction: "up"
}

// node.Download
#Download: {
	#Transfer
	Direction: "down"
}

// node.transfer
#Transfer: {
	Duration:         #Duration | *"1m"
	SampleIOInterval: #Duration | *"100ms"
	BufLen:           int & >0 | *(1024 * 128)
	#Stream
}

// node.StreamServer
#StreamServer: {
	{} | {
		ListenAddr?: string & !=""
	} | {
		ListenAddrKey?: string & !=""
	}
	Protocol: #StreamProtocol
}

// StreamProtocol is the protocol used for StreamClient and StreamServer
#StreamProtocol: *"tcp" | "tcp4" | "tcp6"

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
