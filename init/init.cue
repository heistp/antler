// This package was created by the antler init command.  Feel free to remove
// any comments in this file and add your own doc.

package {{.Package}}

// Tests lists the tests to run.
Test: [
	for r in [10, 20] {
		_mix & {_rtt: r}
	}
]

// MultiReport adds an HTML index file.
MultiReport: [{
	Index: {
		Title: "Tests for the sample package"
	}
}]
