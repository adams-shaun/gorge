package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A token copy made for each object of a multi-object target slot ("for each
// of them, create a token that's a copy of that creature") is made one target
// at a time, as gorge's effCopyPermanent mints each destination's copies in
// turn. When one of those copies has an as-enters choice (a copied Nyota
// Uhura's number), the choice is made as that token enters (CR 614.12,
// 707.9): the targets after it get their copies only once it is answered, the
// spell still resolving (CR 608.2c). execEachTarget parks the loop in
// genState.each and the raGenTokChoice answer resumes it here.

// eachSusp is an each-target loop parked on a copy token's as-enters ask.
type eachSusp struct {
	j    uint8 // the next extra target index plus one; 0 = no loop parked
	slot uint8 // the target slot being walked
	t, v uint8 // the slot's own target and version, restored when the loop ends
	bad  uint8 // res.bad before the loop
}

// DebugCopyEachNoPark is the mutation switch: the loop runs on past a copy
// token's as-enters ask, every target's copies made before it is answered.
// Tests only.
var DebugCopyEachNoPark bool

// eachParks reports whether op in, run for one object of the loop, left an
// ask the loop must wait for.
func (s *State) eachParks(in *kop.Instr) bool {
	return !DebugCopyEachNoPark && in.Op == kop.OpCopyAsTokenEx && s.res.ask == raGenTokChoice
}

// eachRun runs in for the extra targets of slot i from index j0 on. It returns
// true when one of them parked the loop on an ask.
func (s *State) eachRun(k *prep, in *kop.Instr, i uint8, j0 int, w eachSusp) bool {
	e := &s.res.e
	for j := j0; j < int(s.xtOf(e).n); j++ {
		if s.xtOf(e).slot[j] != i || !s.xtLegalRes(k, e, j, i) {
			continue
		}
		e.tgt[i], e.tver[i] = s.xtOf(e).code[j], s.xtOf(e).ver[j]
		s.res.bad &^= 1 << i
		if !s.exec(k, in) && s.eachParks(in) {
			w.j = uint8(j + 2)
			s.g.each = w
			s.g.inLoop = false
			return true
		}
	}
	e.tgt[i], e.tver[i], s.res.bad = w.t, w.v, w.bad
	s.g.inLoop = false
	return false
}

// eachResume continues a parked each-target loop once the copy token's ask
// is answered; true when a later target parks it again.
func (s *State) eachResume(k *prep, in *kop.Instr) bool {
	w := s.g.each
	if w.j == 0 {
		return false
	}
	s.g.each = eachSusp{}
	if k.gen.anyBatch {
		defer s.batchClose(s.batchOpen())
	}
	defer s.holdClose(k, s.holdOpen(k))
	s.g.inLoop = true
	return s.eachRun(k, in, w.slot, int(w.j-1), w)
}
