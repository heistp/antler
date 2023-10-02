// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

// This CUE file defines the Antler configuration schema. For documentation,
// see the corresponding types referenced in the comments.

package antler

import (
	"list"
)

// Run is the top-level antler.TestRun, and consists of a hierarchy of TestRuns
// and associated Reports. It is the only field that test packages must define.
Run: #TestRun

// Results configures the destination paths for results and reports.
Results: #Results

// Server configures the builtin web server.
Server: #Server

//
// antler package
//

// antler.TestRun is used to orchestrate the execution of Tests. Each TestRun
// can have one of Test, Serial or Parallel set, and may have Reports.
//
// Serial lists Tests to be executed sequentially, and Parallel lists Tests to
// be executed concurrently. It's up to the author to ensure that Parallel
// tests can be executed safely, for example by assigning separate namespaces
// to those Tests which may execute at the same time.
//
// Report is a pipeline of #Reports run after the Test completes, and by the
// report command, in parallel with the pipeline in Test.Report. See also the
// During and Report fields in Test.
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

// antler.Results configures the destination paths for results and reports.
//
// Antler writes results non-destructively, i.e. no result data is ever
// overwritten when running tests. Instead, results are saved to a working
// directory during the test, and this is moved to a dated directory when the
// test is complete. The directory structure is as follows:
//
// RootDir/
// RootDir/WorkDir/...
// RootDir/2006-01-02-150415Z/...
//
// RootDir is the top-level directory where all results are saved, relative to
// the test package. If this is changed, then the existing root directory must
// be renamed in order to retain and serve existing results.
//
// WorkDir is the name of the working directory, under RootDir.
//
// ResultDirUTC indicates whether to use UTC time for result directories (true)
// or local time (false). If this is changed, existing directories should be
// renamed to reflect the new time. Failing to do this may cause the lexical
// sorting of runs to be incorrect, with undefined consequences.
//
// ResultDirFormat is a Go time layout (https://pkg.go.dev/time#pkg-constants)
// used to create directory names below RootDir for each run. A fixed ISO 8601
// compliant format is used that contains sufficient precision and sorts runs
// lexically (inspired by Apple's Time Machine).
//
// LatestSymlink is the name of the symlink that links to the latest result
// directory. If empty, the latest symlink is not created.
//
// Codec defines some recognized file encoding (e.g. compression) formats.
#Results: {
	RootDir:      string & !="" | *"results"
	WorkDir:      string & !="" | *"\(RootDir)/in-progress"
	ResultDirUTC: bool | *true
	if !ResultDirUTC {
		ResultDirFormat: "2006-01-02-150405"
	}
	if ResultDirUTC {
		ResultDirFormat: "2006-01-02-150405Z"
	}
	LatestSymlink: string | *"\(RootDir)/latest"
	Codec: [_id=string & !=""]: #Codec & {ID: _id}
	Codec: {
		zstd: {
			Extension: [".zst", ".zstd"]
			DecodePriority: 100 // 0.35s
		}
		gzip: {
			Extension: [".gz"]
			DecodePriority: 200 // 1.78s
		}
		xz: {
			Extension: [".xz"]
			DecodePriority: 300 // 3.27s
		}
		bzip2: {
			Extension: [".bz2"]
			DecodePriority: 400 // 6.56s
		}
	}
}

// antler.Codec configures a file encoder/decoder. This may be for compression,
// or translation between file formats.
//
// ID is a string ID to identify the Codec. Using the name of the command for
// the ID often allows the Encode and Decode defaults to work automatically.
//
// Encode and Decode are the names of the commands used to encode and decode a
// file from stdin to stdout, respectively. EncodeArg and DecodeArg list their
// corresponding command line arguments.
//
// Extension is a list of filename extensions recognized by the Codec.
//
// DecodePriority sets an order to be used when selecting a Codec to decode a
// file, in case there are multiple encoded versions of a file available.
// Codecs with a lower DecodePriority are preferred first, and should generally
// be the ones with better decoding characteristics (e.g. faster).
//
// EncodePriority sets an order to be used when selecting a Codec to encode a
// file, in case there are multiple Codecs defined with the same Extension.
// Codecs with a lower EncodePriority are preferred first, and should generally
// be the ones with better encoding characteristics (e.g. faster).
#Codec: {
	ID: string & !=""
	Extension: [string & =~"\\..*", ...string & =~"\\..*"]
	Decode:         string & !="" | *ID
	DecodeArg:      [...string & !=""] | *["-d"]
	DecodePriority: int
	Encode:         string & !="" | *ID
	EncodeArg:      [...string & !=""] | *[]
	EncodePriority: int | *DecodePriority
}

// antler.Server configures the builtin web server.
//
// ListenAddr is the listen address in the form ":port" or "host:port".
//
// RootDir is fixed to serve the results.
#Server: {
	ListenAddr: string & !="" | *":8080"
	RootDir:    Results.RootDir
}

// antler.Test defines a test to run.
//
// ID is a compound identifier for the Test. It must uniquely identify the Test
// within the package, and its keys and values must match _IDregex. ID is not
// required for a single Test.
//
// ResultPrefix is the base path for any output files. It may use Go template
// syntax (https://pkg.go.dev/text/template), with the Test ID passed to the
// template as its data. Any path separators (e.g. '/') in the string generated
// by the template will result in the creation of directories.
//
// DataFile sets the name suffix of the gob output file used to save the raw
// result data (by default, "data.gob"). If empty, it will not be saved. In
// that case, the runtime overhead for saving the raw data is avoided (a
// minimal gain), but the Test must always be re-run to generate reports, and
// the report command will not work.
//
// Run defines the Run hierarchy, and is documented in more detail in #Run.
//
// During is a pipeline of Reports that runs during the Test. Since these
// Reports only run during the Test, they may not be used to generate reports
// from result data, otherwise those reports would be lost during incremental
// test runs. If the antler nodes are running on the same machine as antler,
// then this pipeline should not be resource intensive, so as not to perturb
// the test.
//
// Report is a pipeline of Reports that is run after the Test, and by the
// report command, in parallel with the pipeline in TestRun.Report.
#Test: {
	_IDregex: "[a-zA-Z0-9][a-zA-Z0-9_-]*"
	ID: [string & =~_IDregex]: string & =~_IDregex
	ResultPrefix: string & !="" | *"{{range $v := .}}{{$v}}_{{end}}"
	DataFile:     string | *"data.gob"
	#Run
	During: [...#Report] | *[
		{SaveFiles: {Consume: true}},
		{EmitLog: {To: ["-"]}},
	]
	Report: [...#Report] | *[
		{EmitLog: {To: ["node.log"], Sort: true}},
	]
}

// antler.Report defines one Report for execution in a TestRun. Reports are
// documented in more detail in their individual types.
#Report: {
	{} | {
		Analyze?: #Analyze
	} | {
		Encode?: #Encode
	} | {
		EmitLog?: #EmitLog
	} | {
		ChartsTimeSeries?: #ChartsTimeSeries
	} | {
		ChartsFCT?: #ChartsFCT
	} | {
		SaveFiles?: #SaveFiles
	}
}

// antler.Analyze is a report that analyzes data used by other reports. This
// must be in the Report pipeline *before* reporters that require it.
#Analyze: {
}

// antler.Encode is a report that encodes, re-encodes and decodes files.
//
// File is a list of glob patterns of files to handle.
//
// Extension is the new extension for the files, indicating the encoding format
// (e.g. ".gz"), which must be supported by one of the Codecs. If blank, the
// file is decoded.
//
// ReEncode, if true, allows re-encoding from and to the same file. This could
// be permitted, for example, to re-encode files from one compression level to
// another. If ReEncode is true, Destructive is implied as false.
//
// Destructive, if true, indicates to remove the original file upon success, if
// the original and destination files are not the same.
#Encode: {
	File: [string & !="", ...string & !=""]
	Extension:   string
	ReEncode:    bool | *false
	Destructive: bool | *false
}

// antler.EmitLog is a report that emits logs. Multiple destinations may be
// listed in To, either filenames, or the '-' character for stdout.
//
// If Sort is true, logs are first gathered, then emitted sorted by time when
// the pipeline stage EmitLog runs in completes.
#EmitLog: {
	To?: [string & !="", ...]
	Sort?: bool
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

// antler.SaveFiles is a Report that saves any FileData from the node, such as
// that created by the System Runner's Stdout and Stderr fields. If Consume is
// true, FileData items are not forwarded to the next stage in the pipeline.
#SaveFiles: {
	Consume: bool | *true
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

// node.Node contains the connection parameters for a node.
//
// ID is a string identifier for the node. This must uniquely identify the
// Node's other fields within the test package.
//
// Platform defines the GOOS-GOARCH combination for the node, e.g. linux-amd64.
// The specified platform must be built into the antler binary (see the
// Makenode script). An exhaustive list of Go supported platforms is here:
// https://github.com/golang/go/blob/master/src/go/build/syslist.go
//
// Launchers, Netns and Env are documented in their respective types.
#Node: {
	ID:       string & !=""
	Platform: string & !=""
	Launcher: #Launchers
	Netns?:   #Netns
	Env?:     #Env
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

// node.Env may be used to set environment variables for the node.
//
// Vars is a list of variables. Each entry must be in the form "key=value".
// See https://pkg.go.dev/os/exec#Cmd. The maximum number of elements must be
// respected, and kept in sync with the definition for Node.Env in Go.
//
// If Inherit is true (the default), the environment of the parent process is
// included.
#Env: {
	Vars?:   [string, ...string] & list.MaxItems(8)
	Inherit: bool | *true
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

// node.Flow is a string flow identifier. Flow identifiers give a relevant
// label to a network flow (e.g. for TCP and UDP, a 5-tuple of protocol,
// src/dst host and src/dst port). To establish a readable convention, flow
// identifiers are lowercase, must start with a-z, and may use digits 0-9,
// '.' or '-'. They are limited to 16 characters, as they may be passed in
// data points. Flow identifiers are best kept small to reduce the size,
// transfer and processing time of results.
#Flow: string & !="" & =~"^[a-z][a-z0-9\\.-]{0,16}$"

// node.ResultStream defines Include and Exclude filters that select which
// results are included and excluded from realtime streaming during a Test.
// Additional documentation is in #MessageFilter.
#ResultStream: {
	Include?: #MessageFilter
	Exclude?: #MessageFilter
}

// node.MessageFilter selects results (messages) based on some simple type and
// field criteria.
//
// File lists glob patterns matching FileData Names to accept. The pattern
// syntax is documented here: https://pkg.go.dev/path/filepath#Match. Use '*'
// to include all files.
//
// Log, if true, means to accept LogEntry's.
//
// Flow lists flow results to stream, by their flow identifier.
//
// All, if true, means to accept all messages.
#MessageFilter: {
	File?: [string, ...string]
	Log?:  bool
	Flow?: #Flow
	All?:  bool
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

// node.PacketClient
#PacketClient: {
	Addr:          string & !=""
	Protocol:      #PacketProtocol
	Flow:          #Flow
	MaxPacketSize: #MaxPacketSize
	Sender: [#PacketSenders, ...#PacketSenders]
}

// MaxPacketSize is the maximum size for PacketClient/PacketServer
// TODO minimum MaxPacketSize seems arbitrary
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
