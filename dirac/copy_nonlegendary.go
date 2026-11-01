package dirac

// A copy exception's NonLegendary$ (CR 707.9b): "except
// it isn't legendary". gorge folds it into the copy's copiable values
// (copypermanent.go removeLegendary), so the copy's type line loses Legendary
// in layer 1 and a copy of the copy inherits it (copyExcAdd pushes the
// template's records first). The two type readers drop the supertype right
// after the added types (chars_gen.go typesGen, static_layerwalk.go walkTypes);
// the legend rule (turn.go) reads those derived types, never the printed ones.

// DebugCopyNonLegKeep is the mutation: the copies stay legendary.
var DebugCopyNonLegKeep bool

// copyExcNoLegendary reports whether some copy exception of slot sl removes
// Legendary.
//
//go:noinline
func (s *State) copyExcNoLegendary(k *prep, sl Slot) bool {
	if DebugCopyNonLegKeep {
		return false
	}
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == sl && r.kind == fxCopyExc && k.t.Overrides[r.r.idx-1].RemoveLegendary {
			return true
		}
	}
	return false
}
