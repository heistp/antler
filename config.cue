// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2022 Pete Heist

// This CUE file defines the Antler configuration schema. For documentation,
// see the corresponding types referenced in the comments.

package antler

import (
	"list"
)

// Test lists the Tests to run. Test packages must set this field to run Tests.
Test: [...#Test]

// MultiReport is a list of multi-Test reports to run. 
MultiReport?: [...#MultiReport]

// Results configures the destination paths for results and reports.
Results: #Results

// Server configures the builtin web server.
Server: #Server

// _IDregex is used for text identifiers in various places.
_IDregex: "[a-zA-Z0-9][a-zA-Z0-9_-]*"

//
// antler package
//

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
// Path is the base path prefix for any output files. It may use Go template
// syntax (https://pkg.go.dev/text/template), with the Test ID passed to the
// template as its data. Any path separators (e.g. '/') in the string generated
// by the template will result in the creation of directories. Path must be
// unique for each Test, and may be empty for a single Test.
//
// DataFile sets the name suffix of the gob output file used to save the raw
// result data (by default, "data.gob"). If empty, it will not be saved. In
// that case, the runtime overhead for saving the raw data is avoided (a
// minimal gain), but the Test must always be re-run to generate reports, and
// the report command will not work.
//
// HMAC enables or disables HMAC protection for test traffic. Enabling HMAC
// prevents casual attackers from sending unauthorized traffic to test servers,
// but does not provide immunity from sophisticated attacks.
//
// Run defines the Run hierarchy, and is documented in more detail in #Run.
//
// Timeout sets the maximum amount of time the Test can run for, and defaults
// to 11 minutes, to comfortably accommodate 10 minute Tests.  A timeout of 0
// disables the timeout.
//
// DuringDefault and During are concatenated together to form a pipeline of
// Reports that are run *while* the Test is run. They may not be used to
// generate saved reports from result data, otherwise those reports would be
// lost during incremental test runs. DuringDefault defines some sensible
// defaults to run during the test, like saving file data and emitting logs,
// but these can be overridden for all Tests.
//
// AfterDefault and After are analogous to DuringDefault and During, but are
// run *after* the Test is run. These may be used to generate persistent reports
// from the result data. AfterDefault defines some sensible defaults to run
// after Tests, like saving sorted log files, and system information.
#Test: {
	ID?: [string & =~_IDregex]: string & =~_IDregex
	Path:     string | *"{{range $v := .}}{{$v}}_{{end}}"
	DataFile: string | *"data.gob"
	HMAC:     bool | *false
	#Run
	Timeout: #Duration | *"660s"
	During?: [...#Report]
	DuringDefault: [...#Report] | *[
			{SaveFiles: {Consume: true}},
			{EmitLog: {To: ["-"]}},
	]
	After?: [...#Report]
	AfterDefault: [...#Report] | *[
			{EmitLog: {To: ["log.txt"], Sort: true}},
			{EmitSysInfo: {To: ["sysinfo_%s.html"]}},
	]
}

// antler.Report contains the union of Report types. Only one field may be set.
// Reports are documented in more detail in their individual definitions.
#Report: {
	Analyze?:          #Analyze
	Encode?:           #Encode
	EmitLog?:          #EmitLog
	EmitSysInfo?:      #EmitSysInfo
	ChartsTimeSeries?: #ChartsTimeSeries
	ChartsFCT?:        #ChartsFCT
	SaveFiles?:        #SaveFiles
}

// antler.Analyze is a report that analyzes data used by other reports. This
// must be in the Report pipeline *before* reports that require it.
#Analyze: {
}

// antler.Encode is a report that encodes, re-encodes and decodes files.
//
// File is a list of glob patterns of files to handle.
//
// Extension is the new extension for the files, indicating the encoding format
// (e.g. ".gz"), which must be supported by one of the Codecs. If blank, the
// files are decoded.
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
// the pipeline stage (that EmitLog runs in) completes.
#EmitLog: {
	To:    [string & !="", ...string & !=""] | *["-"]
	Sort?: bool
}

// antler.EmitSysInfo is a report that emits system information. Multiple
// destinations may be listed in To, either filenames, or the '-' character for
// stdout. Filenames may contain a single %s verb, which is replaced by the
// Node ID the system information is for.
//
// By default, logs are emitted to sysinfo_%s.html.
#EmitSysInfo: {
	To: [string & !="", ...string & !=""] | *["sysinfo_%s.html"]
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

// antler.MultiReport contains one definition for a multi-Test report.
// MultiReports process all the data streams from the Tests they are run for.
// Their input comes from the output of the Test.After pipeline, so that
// single-Test reports run before multi-Test reports.
//
// ID is used to restrict which Tests the MultiReport is run for. The values
// in the key/value pairs are regular expressions used to match ID values for
// the corresponding keys. If no ID is specified, all Tests are matched.
//
// The individual MultiReport types are embedded, and only one may be specified
// for each MultiReport. They are documented in more detail in their individual
// definitions.
#MultiReport: {
	ID?: [string & =~_IDregex]: string & =~_IDregex

	Index?: #Index
}

// antler.Index is a MultiReport that generates an index page for Tests.
//
// To is the path to the index.html file to be generated.
//
// GroupBy is a Test ID key used to separate Tests into groups. It is
// recommended that Tests in a group share the same TestID keys.
//
// Title is a title for the index page.
//
// ExcludeFile is a list of glob patterns
// (https://pkg.go.dev/path/filepath#Match) matching files to exclude from the
// index.
#Index: {
	To:          string & !="" | *"index.html"
	GroupBy?:    string & !=""
	Title?:      string & !=""
	ExcludeFile: [...string] | *["*.gob"]
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
	#Runners
	Serial?: [...#Run]
	Parallel?: [...#Run]
	Schedule?: #Schedule
	Child?:    #Child
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
	Run: [...#Run]
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

// node.Launchers lists the available ways to start a node.
//
// One of either Local or SSH must be specified.
//
// If Local is specified, the node will be launched in a separate process on
// the local machine, using stdio for communication.
//
// If SSH is specified, the node will be executed on a host via the ssh command.
// Destination specifies the destination as given to the ssh binary, if
// different from the Node ID. It must be possible to connect to the ssh
// destination without a password.
//
// For both Local and SSH, Root may be used to run the node as root. If antler
// is run as a regular user and Root is true, sudo -n is used to run the
// node with root privileges. If antler is run as root and Root is false,
// sudo -n -u is used to run the node as the user specified by the SUDO_USER
// environment variable, so it's run as the user that ran antler. It must be
// possible for the user running antler to use sudo without a password (i.e.
// using NOPASSWD: in sudoers file).
//
// The Set fields are for internal use and must not be changed.
#Launchers: {
	SSH?: {
		Destination?: string & !=""
		Root?:        bool
		Set:          true
	}
	Local?: {
		Root?: bool
		Set:   true
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
	Vars?:   [...string] & list.MaxItems(16)
	Inherit: bool | *true
}

// node.Runners lists the Runners available for execution. Each is documented
// further in its corresponding value definition.
#Runners: {
	Sleep?:        #Duration
	ResultStream?: #ResultStream
	SysInfo?:      #SysInfo
	System?:       #System
	PacketClient?: #PacketClient
	PacketServer?: #PacketServer
	StreamClient?: #StreamClient
	StreamServer?: #StreamServer
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
	File?: [...string]
	Log?:  bool
	Flow?: #Flow
	All?:  bool
}

// node.SysInfo gathers system information. See the Go documentation in
// node/sysinfo.go for explanations of each field.
#SysInfo: {
	OS?:          #Texters
	KernSrcInfo?: #Texters
	KernSrcVer?:  #Texters
	Command?: [...#Command]
	File?: [...#File]
	Env?:    #EnvVars
	Sysctl?: #Sysctls
}

// node.Texters lists the available Texter implementations.
#Texters: {
	Command?: #Command
	File?:    #File
	EnvVar?:  #EnvVar
	Sysctl?:  #Sysctl
}

// node.Command represents the information needed to run a system command, and
// implements Texter.
#Command: {
	Command?: string & !=""
	Arg?: [...string]
	Root?: bool
}

// node.File represents a file name, and implements Texter.
#File: string & !=""

// node.EnvVar represents an environment variable name, and implements Texter.
#EnvVar: string & !=""

// node.EnvVars represents a list of patterns of environment variable names.
#EnvVars: [...string & !=""]

// node.Sysctl represents a sysctl parameter name, and implements Texter.
#Sysctl: string & !=""

// node.Sysctls represents a list of patterns of sysctl parameter names.
#Sysctls: [...string & !=""]

// node.System is a system command Runner. See the Go documentation in
// node/system.go for explanations of each field. Often the Command field is
// all that's required.
#System: {
	#Command
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
	DSCP?: int & <=0x3F
	ECN?:  int & <=0x3
	Sockopt?: [...#Sockopt]
}

// MaxPacketSize is the maximum size of a received packet for
// PacketClient/PacketServer. This should only need to be raised for >1500 byte
// MTU, e.g. jumbo frames.
#MaxPacketSize: int & >=0 | *(1500 - 20)

// node.PacketSenders
#PacketSenders: {
	Unresponsive?: #Unresponsive
}

// node.Unresponsive
#Unresponsive: {
	Wait:        [...#Duration] | *["200ms"]
	WaitFirst?:  bool
	RandomWait?: bool
	Length?: [...int]
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
	Addr?:    string & !=""
	AddrKey?: string & !=""
	Protocol: #StreamProtocol
	#Streamers
}

// node.streamers
#Streamers: {
	Upload?:   #Upload
	Download?: #Download
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
	Duration:          #Duration | *"1m"
	Length?:           int & >0
	IOSampleInterval?: #Duration
	TCPInfoInterval?:  #Duration
	BufLen:            int & >0 | *(1024 * 128)
	#Stream
}

// node.Stream defines a stream flow. Flow and Direction are described in their
// corresponding definitions.
//
// CCA is the Congestion Control Algorithm to use.
//
// DSCP is the value for the Differentiated services codepoint.  This value is
// left shifted two places into the upper 6 bits of the former ToS byte /
// Traffic Class field.
//
// ECN is the value of the ECN field, and is used for the lowest two bits of
// the former ToS byte / Traffic Class field.  This is ordinarily set by
// enabling ECN on a socket or globally, but is included here for overriding
// the field for testing purposes.
//
// Sockopt may be used to set generic socket options.
#Stream: {
	Flow:      #Flow
	Direction: #Direction
	CCA?:      string & !=""
	DSCP?:     int & <=0x3F
	ECN?:      int & <=0x3
	Sockopt?: [...#Sockopt]
}

// node.Direction is the sense for a Stream, either "up" (client to server) or
// "down" (server to client).
#Direction: "up" | "down"

// node.Sockopt is a socket option. Type is the Antler-defined socket option
// type, one of the options below. Level and Opt are the int arguments passed
// to the underlying setsockopt() call. Name is a label used for debugging.
// Value is the sockopt value to set.
#Sockopt: {
	Type:  "string" | "int" | "byte"
	Level: int
	Opt:   int
	Name:  string & !=""
	if Type == "string" {
		Value: string & !=""
	}
	if Type == "int" || Type == "byte" {
		Value: int
	}
}

// node.StreamServer is a Runner that listens for a handles connections from
// the StreamClient. ListenAddr is a listen address, and ListenAddrKey is a
// string key that may be communicated to the client using node.Feedback.
#StreamServer: {
	ListenAddr?:    string & !=""
	ListenAddrKey?: string & !=""
	Protocol:       #StreamProtocol
}

// StreamProtocol is the protocol used for StreamClient and StreamServer. It
// defaults to tcp, which may use IPv4 or IPv6, depending on the given address.
// tcp4 or tcp6 forces the use of IPv4 or IPv6, respectively.
#StreamProtocol: *"tcp" | "tcp4" | "tcp6"

//
// Note on Templates
//
// When using Go template syntax in CUE, that will itself be used in a Go
// template file (.tmpl extension), it is necessary to escape the inner template
// similar to the following:
//
//     Path: "{{`{{.name}}_`}}"
//
// That way, the inner template {{.name}} will not be evaluated when the outer
// template file is evaluated. Alternatively, and preferably, any templated
// CUE files will only contain the values that need generation, so as not to
// interfere with other CUE syntax.
//
