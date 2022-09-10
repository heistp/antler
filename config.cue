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
		#DoArg
	} | {
		Serial?: [#TestRun, ...#TestRun]
	} | {
		Parallel?: [#TestRun, ...#TestRun]
	}
}

// antler.DoArg
#DoArg: {
	Log?: bool
}

// antler.Test
#Test: {
	//_check_props: len(Props) & >0
	Props: {...}
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
		Sleep?: #Sleep
	} | {
		ResultStream?: #ResultStream
	} | {
		System?: #System
	}
}

// node.Series
#Series: string & !=""

// node.Sleep
#Sleep: string & =~"^[0-9]+(ns|us|µs|ms|s|m|h)$"

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
