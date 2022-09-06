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
		System?: #System
	}
}

// node.System
#System: {
	Command: string & !=""
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
	Launcher: #launchers
	Netns?:   #Netns
}

// node.launchers
#launchers: {
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
