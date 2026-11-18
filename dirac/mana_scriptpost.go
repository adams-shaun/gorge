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
func (s *State) scriptPostNode(k *prep, a Action, depth, n int) bool {
	return depth > 0 && depth < n && s.playNeedsEmptyStack(k, a)
}
