package dirac

// scriptPostNode reports whether the prefix search revisits the node at depth
// once its children are exhausted. A sorcery-speed play (CR 307.1, 602.5d)
// whose prefix queued triggers is offered only after they resolve, and gorge's
// scriptSearch.visit tries every deeper activation before the pass that lets
// them resolve: when the deepest prefix's resolution ends the game (eight
// Manabarbs triggers at 8 life, Grafted Butcher's {3}{B} from the graveyard),
// the search falls back to the shallower prefix whose resolution survives, so
// the play stays offered. The leaf (depth == n) is judged before its own
// children exist, so only the nodes above it are revisited.
//
// gorge's search is capped at potentialScriptNodes clones and explores every
// subset of the n gaps, not just prefixes of the order: reaching the post-order
// pass of the prefix that leaves m gaps untapped first visits the 2^m - 1
// deeper subsets, each a chain of taps and the passes resolving their triggers.
// Past the cap gorge finds no script and offers nothing (a Room's other door
// under ten Manabarbs triggers at 5 life, depth 4 of 10:
// TestMBCSpikedCorridorReprosLockstep). The bound below prices that walk at its
// worst, n+1 clones per subset; it is a count of gorge's clones, not the
// kernel's.
func (s *State) scriptPostNode(k *prep, a Action, depth, n int) bool {
	if depth <= 0 || depth >= n || n-depth > 8 || !s.playNeedsEmptyStack(k, a) {
		return false
	}
	return depth+((1<<(n-depth))-1)*(n+1) < potentialScriptNodes
}
