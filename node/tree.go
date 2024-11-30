// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2022 Pete Heist

package node

// Tree is a self-referencing type that maps Nodes to a child Tree,
// creating a tree of Nodes.
type Tree map[Node]Tree

// NewTree returns a tree of Nodes used in the given Run hierarchy.
func NewTree(run *Run) (t Tree) {
	t = make(map[Node]Tree)
	buildTree(run, t)
	return
}

// buildTree is called recursively to create a Node tree.
func buildTree(run *Run, tre Tree) {
	var rr []Run
	switch {
	case len(run.Serial) > 0:
		rr = run.Serial
	case len(run.Parallel) > 0:
		rr = run.Parallel
	case run.Schedule != nil:
		rr = run.Schedule.Run
	case run.Child != nil:
		var ok bool
		var t Tree
		if t, ok = tre[run.Child.Node]; !ok {
			t = make(map[Node]Tree)
			tre[run.Child.Node] = t
		}
		buildTree(&run.Child.Run, t)
		return
	}
	for _, r := range rr {
		buildTree(&r, tre)
	}
	return
}

// Platforms returns a list of unique platforms for each Node in the Tree.
func (t Tree) Platforms() (platform []string) {
	m := make(map[string]struct{})
	t.Walk(func(n Node) bool {
		m[n.Platform] = struct{}{}
		return true
	})
	platform = make([]string, 0, len(m))
	for p := range m {
		platform = append(platform, p)
	}
	return
}

// Walk calls the given visitor func for each Node in the Tree. If visitor
// returns false, the walk is terminated and false is returned.
func (t Tree) Walk(visitor func(Node) bool) bool {
	for n, r := range t {
		if !visitor(n) {
			return false
		}
		if !r.Walk(visitor) {
			return false
		}
	}
	return true
}
