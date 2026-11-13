package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugSweepFromSelf deals ExtSweepFrom's damage from the resolving spell
// instead of the target creature (mutation testing only: the lockstep must
// catch it).
var DebugSweepFromSelf bool

// sweepFrom is ExtSweepFrom (gorge effDamageAll with
// DamageSource$ ParentTarget): the creature in Obj deals the amount (read
// once, before any hit) to every creature matching F in gorge's sweep order
// (seat 0's battlefield in entry order, then seat 1's; not Obj's own slot
// with C&1), one damage batch whose hits all name that creature (dealHits:
// its deathtouch, infect and wither per hit, one lifelink gain). A source
// that is gone leaves the resolving spell the source, as gorge's
// newDamageRider keeps its own source when the spec names no object.
func (s *State) sweepFrom(k *prep, in *kop.Instr) {
	src := s.objRef(k, in.Obj)
	n := s.value(k, in.B)
	if n <= 0 {
		return
	}
	var ord [MaxPerm]Slot
	cnt := s.sweepHits(k, in.F, &ord)
	if k.gen != nil && k.gen.anyRepl&(rbDblDmg|rbPrevNC|rbPrevDmgTo|rbPrevCol) != 0 && s.sweepReplCompete(k, src) {
		s.capFull(StallReplPlus) // two damage replacements may match one hit: gorge asks their order (CR 616.1); not modelled
		return
	}
	if DebugSweepFromSelf {
		src = NoSlot
	}
	if src == NoSlot {
		for i := 0; i < cnt; i++ {
			if sl := ord[i]; s.isCreature(k, sl) {
				s.damageTo(k, uint8(s.perms[sl].obj), n)
			}
		}
		return
	}
	var hs [MaxPerm]dmgHit
	m := 0
	for i := 0; i < cnt; i++ {
		if sl := ord[i]; s.isCreature(k, sl) && (in.C&1 == 0 || sl != src) { // seat-ok: an instruction flag bit
			hs[m] = dmgHit{src: src, dst: sl, n: int16(min(n, 1<<14))}
			m++
		}
	}
	s.dealHits(k, hs[:m])
}

// sweepReplCompete reports whether two or more damage replacements (a
// Twinflame-shaped doubler, a Crystal-Barricade-shaped prevention, a
// recipient's own printed K:Prevent "all damage" line, an Ext PreventColor
// record for the source's colours) are in play at once, so that some hit of
// the sweep could pose gorge's CR 616.1 order ask. It is a conservative
// superset of parkNC's per-hit matching: a stall never hides a divergence.
func (s *State) sweepReplCompete(k *prep, src Slot) bool {
	g := k.gen
	c := 0
	for m := s.statM; m.any(); m = m.rest() {
		b := m.first()
		if g.replBits[s.cardAt(b)]&(rbDblDmg|rbPrevNC|rbPrevDmgTo) != 0 && !s.genLost(k, b) {
			c++
		}
	}
	if g.anyDmgMark {
		c += s.dmgMarkTotal() // a mark on some hit's recipient is one more replacement (repl_dmgmark.go)
	}
	if g.anyRepl&rbPrevCol != 0 && src != NoSlot {
		col := s.srcColors(k, src, s.cardAt(src))
		for i := range s.g.prevCol {
			r := &s.g.prevCol[i]
			if r.sc&prevNextSc != 0 && r.col == 0 {
				continue // a spent shield matches nothing
			}
			if r.col != 0 && r.col&col != 0 && r.sc&prevNextSc == 0 || r.sc != 0 {
				c++
			}
		}
	}
	return c >= 2
}
