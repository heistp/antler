// SPDX-License-Identifier: GPL-3.0
// Copyright 2022 Pete Heist

package node

// tree is a self-referencing type that maps Nodes to a child tree,
// creating a tree of Nodes.
type tree map[Node]tree

// newTree returns a tree of Nodes used in the given Run tree.
func newTree(run *Run) (t tree) {
	t = make(map[Node]tree)
	buildTree(run, t)
	return
}

// buildTree is called recursively to create a Node tree.
func buildTree(run *Run, tre tree) {
	switch {
	case len(run.Serial) > 0:
		for _, r := range run.Serial {
			buildTree(&r, tre)
		}
	case len(run.Parallel) > 0:
		for _, r := range run.Parallel {
			buildTree(&r, tre)
		}
	case run.Child != nil:
		var ok bool
		var t tree
		if t, ok = tre[run.Child.Node]; !ok {
			t = make(map[Node]tree)
			tre[run.Child.Node] = t
		}
		buildTree(&run.Child.Run, t)
	}
	return
}

// Platforms returns a list of unique Platforms for each Node in the tree.
func (t tree) Platforms() (platform []string) {
	m := make(map[string]struct{})
	t.walk(func(n Node) {
		m[n.Platform] = struct{}{}
	})
	platform = make([]string, 0, len(m))
	for p := range m {
		platform = append(platform, p)
	}
	return
}

// walk calls the given visitor func for each Node in this tree.
func (t tree) walk(visitor func(Node)) {
	for n, r := range t {
		visitor(n)
		r.walk(visitor)
	}
}
