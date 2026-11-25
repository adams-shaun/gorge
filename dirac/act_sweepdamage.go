package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugSweepFromSelf deals ExtSweepFrom's damage from the resolving spell
// instead of the target creature (mutation testing only: the lockstep must
// catch it).
var DebugSweepFromSelf bool

// DebugSweepNoPlHits drops ExtSweepFrom's C&2 player arm (mutation testing
// only: the captured-sweep lockstep's proof that some game reaches the player
// arm — a player's life then differs).
var DebugSweepNoPlHits bool

// DebugSweepStats, when non-nil, counts ExtSweepFrom's capture-sweep runs
// (the sweep-from-captured lockstep's reachability assertion; the test-only
// callers set it, the kernel never reads it).
type DebugSweepCounters struct {
	Runs   int // capture sweeps entered
	Gone   int // with the source already gone (the NoSlot arm)
	PlHits int // player hits the batch dealt
}

var DebugSweepStats *DebugSweepCounters

// ExtSweepFrom's C flag bits (an instruction's flags, seat-ok).
const (
	sweepNotSrc = 1 // the source's own slot excluded (the ValidCards$ NotDefinedTargeted suffix, a capture's !IsRemembered)
	sweepPlHits = 2 // each living player takes a hit after the creatures (a capture sweep's ValidPlayers$ Player)
)

// sweepFrom is ExtSweepFrom (gorge effDamageAll with a DamageSource$): the
// object in Obj deals the amount (read once, before any hit) to every
// creature matching F in gorge's sweep order (seat 0's battlefield in entry
// order, then seat 1's; not Obj's own slot with C&1), one damage batch whose
// hits all name that object (dealHits: its deathtouch, infect and wither per
// hit, one lifelink gain). With C&2 each living player takes a hit after the
// creatures, the same batch (a capturing emblem's DamageSource$ Remembered
// sweep; gorge effDamageAll's ValidPlayers$ arm). A source that is gone
// leaves the resolving spell the source, as gorge's newDamageRider keeps its
// own source when the spec names no object.
func (s *State) sweepFrom(k *prep, in *kop.Instr) {
	src := s.objRef(k, in.Obj)
	n := s.value(k, in.B)
	if n <= 0 {
		return
	}
	if dbg := DebugSweepStats; dbg != nil && in.Obj == kop.RefTriggerSource {
		dbg.Runs++
		if src == NoSlot {
			dbg.Gone++
		}
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
				s.damageTo(k, TCode(s.perms[sl].obj), n)
				if s.g.nc.n != 0 {
					// an event parked on its ask (Dralnu's sacrifice): gorge asks it
					// between the sweep's events, and this loop has no resume
					s.capFull(StallReplPlus)
					return
				}
			}
		}
		if in.C&sweepPlHits != 0 && !DebugSweepNoPlHits { // seat-ok: a test-only mutation (DebugSweepNoPlHits)
			s.sweepPlayersTo(k, n)
		}
		return
	}
	// hs holds every creature hit (at most MaxPerm: nearCapacity stalls an
	// entry path at occupancy MaxPerm-4, but the bound is not this function's
	// to trust) plus one player hit per seat, so it is sized for the sum.
	var hs [MaxPerm + MaxSeats]dmgHit
	m, pl0 := 0, 0
	for i := 0; i < cnt; i++ {
		if sl := ord[i]; s.isCreature(k, sl) && (in.C&sweepNotSrc == 0 || sl != src) { // seat-ok: an instruction flag bit
			hs[m] = dmgHit{src: src, dst: sl, n: int16(min(n, 1<<14))}
			m++
		}
	}
	if in.C&sweepPlHits != 0 && !DebugSweepNoPlHits { // seat-ok: a test-only mutation (DebugSweepNoPlHits)
		pl0 = m
		for p := uint8(0); p < s.nSeats(); p++ {
			if s.out(p) {
				continue
			}
			hs[m] = dmgHit{src: src, n: int16(min(n, 1<<14)), pl: p + 1}
			m++
		}
	}
	s.dealHits(k, hs[:m])
	if dbg := DebugSweepStats; dbg != nil && in.Obj == kop.RefTriggerSource {
		dbg.PlHits += m - pl0 // the player hits are the tail of the batch
	}
}

// sweepPlayersTo is the gone-source sweep's player arm: each living player
// takes n from the resolving effect, seat order (gorge effDamageAll's
// ValidPlayers$ arm through the same rider).
func (s *State) sweepPlayersTo(k *prep, n int) {
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		s.damageTo(k, PlayerCode(p), n)
	}
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
	if g.anyRedir {
		c += s.redirLive() // a redirect on some hit's recipient is one more replacement (repl_dmgredirect.go)
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
