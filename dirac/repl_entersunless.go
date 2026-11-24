package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// replSkip is OpSkipUnless inside an as-enters replacement program
// runReplProgAs walks the program with its own index,
// so a node's Condition gate (gorge effects/registry.go Resolve: a failed
// gate skips that node only, the chain continues) returns how many following
// instructions to skip. The count excludes the entering object like the
// whole-replacement condition does (the caller cleared it from occ).
func (s *State) replSkip(k *prep, sl Slot, in *kop.Instr) int {
	if in.Op == kop.OpBranch {
		return s.replBranchSkip(sl, in) // face_siegemode.go
	}
	if s.condEval(k, kop.CondID(in.C), s.ctxEntry(&s.res.e)) {
		return 0
	}
	return int(in.A.N)
}
