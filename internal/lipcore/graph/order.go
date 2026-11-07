package graph

import (
	"container/heap"
	"slices"
	"sort"
)

// InstallOrder orders a package set for installation (protocol/workspace.md
// §5; GraphSolver.InstallOrder): every package comes after the packages it
// depends on; packages in a dependency cycle, and remaining ties, are
// ordered by folded identity (ordinal), then version (sort order), then
// commit. Packages of one PackageKey count once (the first one given).
// Dependencies outside the set are ignored. Reverse the result to get an
// uninstall order.
func InstallOrder(set []Candidate, dependenciesOf func(Candidate) []Candidate) []Candidate {
	sortedSet := slices.Clone(set)
	sort.SliceStable(sortedSet, func(i, j int) bool {
		a, b := sortedSet[i], sortedSet[j]
		if c := compareOrdinal(Fold(a.Identity()), Fold(b.Identity())); c != 0 {
			return c < 0
		}
		if c := compareSortOrder(a.Version(), b.Version()); c != 0 {
			return c < 0
		}
		return compareOrdinal(a.Commit(), b.Commit()) < 0
	})
	var nodes []Candidate
	indexOf := map[PackageKey]int{}
	for _, c := range sortedSet {
		key := KeyOf(c)
		if _, ok := indexOf[key]; !ok {
			indexOf[key] = len(nodes)
			nodes = append(nodes, c)
		}
	}
	order := InstallOrderIndices(len(nodes), func(i int) []int {
		var deps []int
		for _, d := range dependenciesOf(nodes[i]) {
			if j, ok := indexOf[KeyOf(d)]; ok {
				deps = append(deps, j)
			} else {
				deps = append(deps, -1)
			}
		}
		return deps
	})
	out := make([]Candidate, len(order))
	for k, i := range order {
		out[k] = nodes[i]
	}
	return out
}

// InstallOrderIndices is InstallOrder for count packages given in ascending
// order: the indices of the packages in installation order, given the
// indices of the packages each depends on. Indices out of range and
// self-edges are ignored.
func InstallOrderIndices(count int, dependenciesOf func(int) []int) []int {
	// Edges point from a depender to its dependees.
	edges := make([][]int, count)
	for i := range count {
		var e []int
		for _, j := range dependenciesOf(i) {
			if j >= 0 && j < count && j != i {
				e = append(e, j)
			}
		}
		slices.Sort(e)
		edges[i] = slices.Compact(e)
	}

	componentOf, componentCount := stronglyConnectedComponents(edges)

	// Nodes are sorted, so iterating them in order yields each component's
	// members sorted, and the first member seen is the component's smallest.
	members := make([][]int, componentCount)
	rank := make([]int, componentCount)
	for i := range count {
		c := componentOf[i]
		if len(members[c]) == 0 {
			rank[c] = i
		}
		members[c] = append(members[c], i)
	}

	// Kahn's algorithm on the condensation, always emitting the ready
	// component with the smallest member first.
	pending := make([]int, componentCount)
	dependers := make([][]int, componentCount)
	seen := map[[2]int]bool{}
	for i := range count {
		for _, j := range edges[i] {
			from, to := componentOf[i], componentOf[j]
			if from != to && !seen[[2]int{from, to}] {
				seen[[2]int{from, to}] = true
				pending[from]++
				dependers[to] = append(dependers[to], from)
			}
		}
	}
	ready := &rankHeap{rank: rank}
	for c := range componentCount {
		if pending[c] == 0 {
			heap.Push(ready, c)
		}
	}
	order := make([]int, 0, count)
	for ready.Len() > 0 {
		c := heap.Pop(ready).(int)
		order = append(order, members[c]...)
		for _, d := range dependers[c] {
			pending[d]--
			if pending[d] == 0 {
				heap.Push(ready, d)
			}
		}
	}
	return order
}

// rankHeap is a priority queue of components by rank (unique per
// component).
type rankHeap struct {
	items []int
	rank  []int
}

func (h *rankHeap) Len() int           { return len(h.items) }
func (h *rankHeap) Less(i, j int) bool { return h.rank[h.items[i]] < h.rank[h.items[j]] }
func (h *rankHeap) Swap(i, j int)      { h.items[i], h.items[j] = h.items[j], h.items[i] }
func (h *rankHeap) Push(x any)         { h.items = append(h.items, x.(int)) }
func (h *rankHeap) Pop() any {
	x := h.items[len(h.items)-1]
	h.items = h.items[:len(h.items)-1]
	return x
}

// stronglyConnectedComponents is Tarjan's algorithm, iterative. Returns the
// component of every node and the number of components.
func stronglyConnectedComponents(edges [][]int) ([]int, int) {
	n := len(edges)
	index := make([]int, n)
	lowLink := make([]int, n)
	component := make([]int, n)
	onStack := make([]bool, n)
	for i := range index {
		index[i] = -1
	}
	var stack []int
	type frame struct{ node, edge int }
	var work []frame
	nextIndex, count := 0, 0

	for root := range n {
		if index[root] != -1 {
			continue
		}
		work = append(work, frame{root, 0})
		for len(work) > 0 {
			f := work[len(work)-1]
			work = work[:len(work)-1]
			v := f.node
			if f.edge == 0 {
				index[v], lowLink[v] = nextIndex, nextIndex
				nextIndex++
				stack = append(stack, v)
				onStack[v] = true
			} else {
				child := edges[v][f.edge-1]
				lowLink[v] = min(lowLink[v], lowLink[child])
			}

			descended := false
			for e := f.edge; e < len(edges[v]); e++ {
				w := edges[v][e]
				if index[w] == -1 {
					work = append(work, frame{v, e + 1}, frame{w, 0})
					descended = true
					break
				}
				if onStack[w] {
					lowLink[v] = min(lowLink[v], index[w])
				}
			}
			if descended || lowLink[v] != index[v] {
				continue
			}
			for {
				member := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[member] = false
				component[member] = count
				if member == v {
					break
				}
			}
			count++
		}
	}
	return component, count
}
