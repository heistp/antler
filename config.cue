// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

// This CUE file defines the Antler configuration schema. For documentation,
// see the corresponding types referenced in the comments.

package antler

// Run is the top-level antler.TestRun, and only top-level concrete field.
// Run consists of a hierarchy of TestRuns, and associated Reports.
Run: #TestRun

//
// antler package
//

// antler.TestRun is used to orchestrate the execution of Tests. Each TestRun
// can have one of Test, Serial or Parallel set, and may have a Report. Serial
// lists Tests to be executed sequentially, and Parallel lists Tests to be
// executed concurrently. It's up to the author to ensure that Parallel tests
// can be executed safely, for example by assigning separate namespaces to
// those Tests which may execute at the same time.
#TestRun: {
	{} | {
		Test?: #Test
	} | {
		Serial?: [#TestRun, ...#TestRun]
	} | {
		Parallel?: [#TestRun, ...#TestRun]
	}
	Report?: [#Report, ...#Report]
}

// antler.Test defines a test to run. The given OutPath defines a base path for
// output. If it ends in '/', it will be treated as a directory to create,
// under which any output files will be placed. If not, it will be treated as a
// base filename (with optional preceding path), and each output filename is
// joined with this name using an '_' character to obtain the final filename.
//
// Run defines the Run hierarchy, and is documented in more detail in #Run.
#Test: {
	OutPath: string & !="" | *"./"
	#Run
}

// antler.Report defines one Report for execution in a TestRun. Reports are
// documented in more detail in their individual types.
#Report: {
	{} | {
		EmitLog?: #EmitLog
	} | {
		EmitTCPInfo?: #EmitTCPInfo
	} | {
		ExecuteTemplate?: #ExecuteTemplate
	} | {
		ChartsTimeSeries?: #ChartsTimeSeries
	} | {
		ChartsTCPInfo?: #ChartsTCPInfo
	} | {
		ChartsFCT?: #ChartsFCT
	} | {
		SaveFiles?: #SaveFiles
	}
}

// antler.EmitLog is a report that emits and writes logs. Multiple destinations
// may be listed in To, either filenames, or the '-' character for stdout.
#EmitLog: {
	To?: [string & !="", ...]
}

// antler.EmitTCPInfo is a report that emits and writes TCPInfo as text.
// Multiple destinations may be listed in To, either filenames, or the '-'
// character for stdout.
#EmitTCPInfo: {
	To?: [string & !="", ...]
}

// antler.ExecuteTemplate runs a Go template from the given source file From,
// or with the given Text. The template receives a .Data field, which is a
// channel that receives the raw data from the node, before it has been
// analyzed in any way. This may not be for the feint of heart.
#ExecuteTemplate: {
	Name?: string & !=""
	To:    string & !=""
	{} | {
		Text?: string & !=""
	} | {
		From?: [string & !="", ...string & !=""]
	}
}

// antler.ChartsTimeSeries runs a Go template to create a time series plot
// using Google Charts containing one or two axes, with the goodput for any
// stream flows, and delay times for any packet flows. The Options field may
// be used to set any Configuration Options that Google Charts supports:
//
// https://developers.google.com/chart/interactive/docs/gallery/linechart#configuration-options
#ChartsTimeSeries: {
	FlowLabel?: {
		[=~".*"]: string
	}
	To:      [string & !="", ...string & !=""] | *["timeseries.html"]
	Options: {...} & {
		title: string | *"Time Series"
		titleTextStyle: {
			fontSize: 18
			...
		}
		width:     1280
		height:    720
		lineWidth: 1
		//curveType: "function",
		vAxes: {
			"0": {
				title: string | *"Goodput (Mbps)"
				titleTextStyle: {
					italic: bool | *false
					...
				}
				viewWindow: {
					min: float | *0
					...
				}
				baselineColor: string | *"#cccccc"
				gridlines: {
					color: string | *"transparent"
					...
				}
				...
			}
			"1": {
				title: string | *"Delay (ms)"
				titleTextStyle: {
					italic: bool | *false
					...
				}
				viewWindow: {
					min: float | *0
					...
				}
				baselineColor: string | *"#cccccc"
				gridlines: {
					color: string | *"transparent"
					...
				}
				...
			}
			...
		}
		hAxis: {
			title: string | *"Time (sec)"
			titleTextStyle: {
				italic: bool | *false
				...
			}
			viewWindow: {
				min: int | *0
				...
			}
			baselineColor: string | *"#cccccc"
			gridlines: {
				color: string | *"transparent"
				...
			}
			...
		}
		chartArea: {
			backgroundColor: string | *"#f7f7f7"
			top:             int | *100
			width:           string | *"80%"
			//left:            int | *75
			//height:          string | *"75%"
			...
		}
		explorer: {
			actions:   [...string] | *["dragToZoom", "rightClickToReset"]
			maxZoomIn: float | *0.001
			...
		}
		...
	}
}

// antler.ChartsTCPInfo runs a Go template to create a time series plot using
// Google Charts containing two axes, with the cwnd and retransmission rate.
// The Options field may be used to set any Configuration Options that Google
// Charts supports:
//
// https://developers.google.com/chart/interactive/docs/gallery/linechart#configuration-options
#ChartsTCPInfo: {
	FlowLabel?: {
		[=~".*"]: string
	}
	To:      [string & !="", ...string & !=""] | *["tcpinfo.html"]
	Options: {...} & {
		title: string | *"TCP Info"
		titleTextStyle: {
			fontSize: 18
			...
		}
		width:     1280
		height:    720
		lineWidth: 1
		//curveType: "function",
		vAxes: {
			"0": {
				title: string | *"Cum. Avg. Retransmit Rate (rtx/sec)"
				titleTextStyle: {
					italic: bool | *false
					...
				}
				viewWindow: {
					min: float | *0
					...
				}
				baselineColor: string | *"#cccccc"
				gridlines: {
					color: string | *"transparent"
					...
				}
				...
			}
			"1": {
				title: string | *"CWND (units of MSS)"
				titleTextStyle: {
					italic: bool | *false
					...
				}
				viewWindow: {
					min: float | *0
					...
				}
				baselineColor: string | *"#cccccc"
				gridlines: {
					color: string | *"transparent"
					...
				}
				...
			}
			...
		}
		hAxis: {
			title: string | *"Time (sec)"
			titleTextStyle: {
				italic: bool | *false
				...
			}
			viewWindow: {
				min: int | *0
				...
			}
			baselineColor: string | *"#cccccc"
			gridlines: {
				color: string | *"transparent"
				...
			}
			...
		}
		chartArea: {
			backgroundColor: string | *"#f7f7f7"
			top:             int | *100
			width:           string | *"80%"
			//left:            int | *75
			//height:          string | *"75%"
			...
		}
		explorer: {
			actions:   [...string] | *["dragToZoom", "rightClickToReset"]
			maxZoomIn: float | *0.001
			...
		}
		...
	}
}

// antler.ChartsFCT runs a Go template to create a scatter plot of flow
// completion time vs length. The Options field may be used to set any
// Configuration Options that Google Charts supports:
//
// https://developers.google.com/chart/interactive/docs/gallery/scatterchart#configuration-options
#ChartsFCT: {
	FlowLabel?: {
		[=~".*"]: string
	}
	To: [string & !="", ...string & !=""]
	Series?: [...#FlowSeries]
	Options: {...} & {
		title: string | *"Flow Completion Time vs Length"
		titleTextStyle: {
			fontSize: 18
			...
		}
		width:     1280
		height:    720
		pointSize: 1
		vAxes: {
			"0": {
				title: string | *"Flow Completion Time (sec)"
				titleTextStyle: {
					italic: bool | *false
					...
				}
				viewWindow: {
					min: float | *0
					...
				}
				baselineColor: string | *"#cccccc"
				gridlines: {
					//color: string | *"transparent"
					...
				}
				...
			}
			...
		}
		hAxis: {
			title:     string | *"Length (kB)"
			scaleType: string | *"log"
			titleTextStyle: {
				italic: bool | *false
				...
			}
			viewWindow: {
				min: int | *0
				...
			}
			baselineColor: string | *"#cccccc"
			gridlines: {
				//color: string | *"transparent"
				...
			}
			...
		}
		chartArea: {
			backgroundColor: string | *"#f7f7f7"
			top:             int | *100
			width:           string | *"80%"
			//left:            int | *75
			//height:          string | *"75%"
			...
		}
		...
	}
}

// antler.FlowSeries groups Flows into a chart series named Name, using the
// given Pattern, an RE2 regular expression:
//
// https://github.com/google/re2/wiki/Syntax
#FlowSeries: {
	Pattern: string & !=""
	Name:    string & !=""
}

// antler.SaveFiles is a parameterless Report that saves any FileData from the
// node, such as that created by the System Runner's Stdout and Stderr fields.
// If SaveFiles is not included, FileData is discarded.
#SaveFiles: {
}

//
// node package
//

// node.Run defines the test Run hierarchy. Run contains either an embedded
// Runner defined in Runners, or one of Serial, Parallel, Schedule or Child.
//
// If a Runner is set, this run is a leaf, and the Runner is executed.
//
// Serial contains a list of Runs to execute sequentially.
//
// Parallel contains a list of Runs to execute concurrently.
//
// Schedule defines arbitrary timings for Run execution, and is documented in
// more detail in the #Schedule definition.
#Run: {
	{} | {
		#Runners
	} | {
		Serial?: [#Run, ...#Run]
	} | {
		Parallel?: [#Run, ...#Run]
	} | {
		Schedule?: #Schedule
	} | {
		Child?: #Child
	}
}

// node.Schedule schedules execution of the given Runs, using the given
// Durations in Wait to sleep between the execution of each Run. If Random is
// true, random times are chosen from Wait, otherwise they are taken
// sequentially from Wait, wrapping as necessary. If Sequential is true, the
// Runs are executed in succession with wait times between each, otherwise the
// Runs are executed concurrently. If WaitFirst is true, a wait occurs before
// the first Run as well.
#Schedule: {
	Wait?: [...#Duration]
	Random?:     bool
	Sequential?: bool
	WaitFirst?:  bool
	Run: [#Run, ...#Run]
}

// node.Child defines a Run to execute on a child Node. In this way, entire Run
// hierarchies may be passed to a child Node at once. Nodes are launched
// automatically and recursively at the start of each Test by walking the Run
// tree to connect to and set up child nodes, so that node startup times do not
// affect the Test results.
#Child: {
	Node: #Node
	#Run
}

// node.Node contains the connection parameters for a node. ID is a string
// identifier for the node. Platform defines the GOOS-GOARCH combination for
// the node, e.g. linux-amd64. The specified platform must be built into the
// antler binary (see the Makenode script). An exhaustive list of Go supported
// platforms is here:
// https://github.com/golang/go/blob/master/src/go/build/syslist.go
// Launchers and Netns are documented in their respective types.
#Node: {
	ID:       string & !=""
	Platform: string & !=""
	Launcher: #Launchers
	Netns?:   #Netns
}

// node.Launchers lists the available ways to start a node. For SSH, Destination
// specifies the destination as given to the ssh binary, if different from the
// Node ID. If Local is specified, the node will be launched in a separate
// process on the local machine, using stdio for communication.
//
// It must be possible to connect to the ssh destination without a password, and
// for Linux, the root user is required to use network namespaces.
#Launchers: {
	{} | {
		SSH?: {Destination?: string & !=""}
	} | {
		Local?: {}
	}
}

// node.Netns may be set to launch the node in a Linux network namespace.
//
// Create indicates whether to create a namespace (true) or use an existing one
// (false). If Create is true with no Name set, the Node ID will be used as the
// network namespace name.
//
// Name is the name of the namespace. If set, this namespace will either be
// created or used, depending on the value of the Create field.
#Netns: {
	Create?: bool
	Name?:   string & !=""
}

// node.Runners lists the Runners available for execution. Each is documented
// further in its corresponding value definition.
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

// node.Duration is a time duration with mandatory units, as defined here:
//
// https://pkg.go.dev/time#ParseDuration
#Duration: string & =~"^([0-9]*\\.)?[0-9]+(ns|us|µs|ms|s|m|h)$"

// node.Flow is a string flow identifier. Flow identifiers typically give a
// relevant label to a network flow, which for TCP and UDP is often a 5-tuple
// of protocol, src/dst host and src/dst port. Flow identifiers are limited to
// lowercase characters, '.' and '-', merely to set a readable convention. They
// are limited to 255 characters as they may be passed with data points and
// inside negotiating packets.
#Flow: string & !="" & =~"^[a-z][a-z\\.-]{0,255}$"

// node.ResultStream defines Include and Exclude filters that select which
// results are included and excluded from realtime streaming during a Test.
// Additional documentation is in #MessageFilter.
#ResultStream: {
	Include?: #MessageFilter
	Exclude?: #MessageFilter
}

// node.MessageFilter selects results (messages) based on some simple type and
// field criteria. File lists glob patterns matching FileData Names to accept.
// The pattern syntax is documented here:
// https://pkg.go.dev/path/filepath#Match
// Log, if true, means to accept LogEntry's.
#MessageFilter: {
	File?: [string, ...string]
	Log?:  bool
	Flow?: #Flow
}

// node.System is a system command Runner. See the Go documentation in
// node/system.go for explanations of each field. Often the Command field is
// all that's required.
#System: {
	Command?: string & !=""
	Args?: [string, ...string]
	Background?:   bool
	IgnoreErrors?: bool
	Stdout?:       string & !=""
	Stderr?:       string & !=""
	Kill?:         bool
}

// node.PacketClient (TODO min MaxPacketSize is arbitrary)
#PacketClient: {
	Addr:          string & !=""
	Protocol:      #PacketProtocol
	Flow:          #Flow
	MaxPacketSize: #MaxPacketSize
	Sender: [#PacketSenders, ...#PacketSenders]
}

// MaxPacketSize is the maximum size for PacketClient/PacketServer
#MaxPacketSize: int & >=32 | *(1500 - 20)

// node.PacketSenders
#PacketSenders: {
	{} | {
		Unresponsive?: #Unresponsive
	}
}

// node.Unresponsive
#Unresponsive: {
	Wait:        [...#Duration] | *["200ms"]
	WaitFirst?:  bool
	RandomWait?: bool
	Length?: [int, ...int]
	RandomLength?: bool
	Duration:      #Duration
	Echo:          bool | *false
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
	Length?:          int & >0
	IOSampleInterval: #Duration | *"100ms"
	TCPInfoInterval?: #Duration
	BufLen:           int & >0 | *(1024 * 128)
	#Stream
}

// node.Stream defines a stream flow. Flow and Direction are described in their
// corresponding definitions. CCA is the Congestion Control Algorithm to use.
#Stream: {
	Flow:      #Flow
	Direction: #Direction
	CCA?:      string & !=""
}

// node.Direction is the sense for a Stream, either "up" (client to server) or
// "down" (server to client).
#Direction: "up" | "down"

// node.StreamServer is a Runner that listens for a handles connections from
// the StreamClient. ListenAddr is a listen address, and ListenAddrKey is a
// string key that may be communicated to the client using node.Feedback.
#StreamServer: {
	{} | {
		ListenAddr?: string & !=""
	} | {
		ListenAddrKey?: string & !=""
	}
	Protocol: #StreamProtocol
}

// StreamProtocol is the protocol used for StreamClient and StreamServer. It
// defaults to tcp, which may use IPv4 or IPv6, depending on the given address.
// tcp4 or tcp6 forces the use of IPv4 or IPv6, respectively.
#StreamProtocol: *"tcp" | "tcp4" | "tcp6"
