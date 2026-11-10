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
	j    uint8   // the next extra target index plus one; 0 = no loop parked
	slot uint8   // the target slot being walked
	t, v uint8   // the slot's own target and version, restored when the loop ends
	bad  uint8   // res.bad before the loop
	hold holdRec // a return's entry frame, kept open across the ask (on: it is ours to close)
}

// DebugCopyEachNoPark is the mutation switch: the loop runs on past a copy
// token's as-enters ask, every target's copies made before it is answered.
// Tests only.
var DebugCopyEachNoPark bool

// tokSrcM is the permanents p controls whose CreateToken replacements a token
// being created reads. The copies of one "for each of them, create a token"
// are created together (gorge effCopyPermanent sizes every destination's copy
// through the replacements before the first one enters, CR 111.1, 614.1), so
// a Holo-Engineer copy that entered earlier in the loop rewrites none of the
// later ones: the loop fixes the permanents when it begins (tokFrz).
func (s *State) tokSrcM(p uint8) slotMask {
	m := s.statM.and(s.ctrlM[p])
	if s.g.tokFrzOn {
		m = m.and(s.g.tokFrz)
	}
	return m
}

// eachParks reports whether op in, run for one object of the loop, left an
// ask the loop must wait for: a copy's as-enters choice, or the host an Aura
// copy enchants (CR 303.4f; Doppelgang copying a targeted Curious Inquiry).
func (s *State) eachParks(in *kop.Instr) bool {
	return !DebugCopyEachNoPark && in.Op == kop.OpCopyAsTokenEx && (s.res.ask == raGenTokChoice || s.res.ask == raGenEnchant)
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
		n0 := s.g.nask
		if ok := s.exec(k, in); !ok && (s.eachParks(in) || s.eachReturnParked(in)) || ok && s.eachReturnParks(k, in, n0) {
			w.j = uint8(j + 2)
			s.g.each = w
			s.g.inLoop = false
			return true
		}
	}
	if w.hold.on {
		s.holdClose(k, w.hold) // the targets still enter together (CR 603.6a)
	}
	e.tgt[i], e.tver[i], s.res.bad = w.t, w.v, w.bad
	s.g.inLoop = false
	s.g.tokFrzOn = false
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
	if returnToBattlefield(in) && w.hold.on {
		// the frame stayed open across the ask; eachRun closes it
	} else if returnToBattlefield(in) {
		defer s.holdClose(k, s.holdEntries()) // as execEachTarget: the targets enter together
	} else {
		defer s.holdClose(k, s.holdOpen(k))
	}
	s.g.inLoop = true
	return s.eachRun(k, in, w.slot, int(w.j-1), w)
}
