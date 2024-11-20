// This package was created by the antler init command.  Feel free to remove
// any comments in this file and add your own doc.

package {{.Package}}

// Tests lists the tests to run.
Test: [
	_mix
]

// MultiReport adds an HTML index file.
MultiReport: [{
	Index: {
		Title: "Tests for the sample package"
	}
}]
