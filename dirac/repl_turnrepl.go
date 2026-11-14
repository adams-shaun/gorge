package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// ExtTurnRepl (kop/turnrepl.go): two until-end-of-turn replacements an Effect
// makes over a player's side of the board. Neither costs State bytes: the
// exile one is a scope record in the cold exMark table, the counter one a
// region of the cold buffer (coldCtrP) that exists only for a table
// with such a program.

// DebugTurnReplOff is the mutation switch: both replacements are set but
// ignored, so a lockstep that reaches either diverges.
var DebugTurnReplOff bool

// TurnReplStats counts the lockstep tests' coverage: opposing creatures sent
// to exile by the scope record, placements the creature counter-plus-one made
// larger, CR 616.1 order asks that offered a scope record as a candidate,
// spells an uncounter record protected (Unc), targetings a player's hexproof
// record refused (HexP) and permanents asked about while marked with Veil's
// grant (HexO).
var TurnReplStats struct{ Exiled, Plus, Asked, Unc, HexP, HexO atomic.Int64 }

// StallTurnRepl is a second scope record or creature counter record past what
// the cold table holds: the exile-instead table is full.
const StallTurnRepl = 159

// coldHexUB is the mechHexUB grant's timestamp per battlefield slot (0:
// unmarked), for a table with TurnReplHexUB.
var coldHexUB = newCold[uint16]("hexUB", func(g *genPrep) int {
	if g.anyHexUB {
		return MaxPerm
	}
	return 0
}).view(func(c *coldArrays) *[]uint16 { return &c.hexUB }).perSlot()

// coldCtrP is the creature counter-plus-one record (0 or 1 of them), for a
// table with a Prairie Dog replacement.
var coldCtrP = newCold[plusCreRec]("ctrP", func(g *genPrep) int {
	if g.anyPlusCre {
		return 1
	}
	return 0
})

func init() { stallNames[StallTurnRepl] = "turnrepl" }

// slotExScope is the exMarkRec.sl of a scope record: "a creature an opponent
// of pl controls would die, exile it instead". No battlefield slot is one.
const slotExScope Slot = NoSlot - 2

// slotExUncAny and slotExUncCre are the exMarkRec.sl of an uncounter scope
// record (TurnReplUncounter, TurnReplUncounterCreature): "a spell (a creature
// spell) its controller pl controls can't be countered this turn". pl is the
// effect's controller. No battlefield slot is one.
const (
	slotExUncAny Slot = NoSlot - 3
	slotExUncCre Slot = NoSlot - 4
	slotExPlHex  Slot = NoSlot - 5 // a player hexproof record (TurnReplPlayerHex, TurnReplHexUB): pl, col and ts
)

// prepTurnRepl sets what the table's ExtTurnRepl programs need: the counter
// kind reads genPrep.anyRepl's rbPlusCtrEOT gates beside Boimler's, and the cold
// buffer region; the uncounter kinds arm uncounterable's scope scan.
func prepTurnRepl(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op != kop.OpExt || in.A.K != kop.VConst || kop.ExtOp(in.A.N) != kop.ExtTurnRepl {
					continue
				}
				switch in.C {
				case kop.TurnReplPlusCreatureCtr:
					g.anyRepl |= rbPlusCtrEOT
					g.anyPlusCre = true
				case kop.TurnReplUncounter, kop.TurnReplUncounterCreature:
					g.anyUncScope, g.anyTurnScope = true, true
				case kop.TurnReplPlayerHex, kop.TurnReplHexUB:
					g.anyPlHexRec, g.anyTurnScope = true, true
					g.anyRepl |= rbPlHex // playerHexproof's gate
					g.anyHexUB = g.anyHexUB || in.C == kop.TurnReplHexUB
				}
			}
		}
	}
}

// turnRepl is ExtTurnRepl.
func (s *State) turnRepl(k *prep, in *kop.Instr) {
	switch in.C {
	case kop.TurnReplExileOpp:
		for i := range s.cold.exMark {
			if r := &s.cold.exMark[i]; !r.live {
				*r = exMarkRec{sl: slotExScope, src: s.res.e.obj, live: true, pl: s.res.e.ctrl}
				return
			}
		}
		s.capFull(StallTurnRepl)
	case kop.TurnReplUncounter, kop.TurnReplUncounterCreature:
		// each activation is its own replacement (CR 616.1): two live
		// records matching one spell stall (uncounterable), as gorge's
		// order ask over identical stoppers does
		sl := slotExUncAny
		if in.C == kop.TurnReplUncounterCreature {
			sl = slotExUncCre
		}
		for i := range s.cold.exMark {
			if r := &s.cold.exMark[i]; !r.live {
				*r = exMarkRec{sl: sl, src: s.res.e.obj, live: true, pl: s.res.e.ctrl}
				return
			}
		}
		s.capFull(StallTurnRepl)
	case kop.TurnReplPlayerHex, kop.TurnReplHexUB:
		// each resolution is its own grant: a record of its own (the permanents
		// the second Veil marks are the ones its controller controls now)
		rec := exMarkRec{sl: slotExPlHex, src: s.res.e.obj, live: true, pl: s.res.e.ctrl, ts: s.tick()}
		if in.C == kop.TurnReplHexUB {
			rec.col = kop.ColU | kop.ColB
		}
		for i := range s.cold.exMark {
			if r := &s.cold.exMark[i]; !r.live {
				*r = rec
				if in.C == kop.TurnReplHexUB {
					for m := s.ctrlM[rec.pl]; m.any(); m = m.rest() {
						sl := m.first()
						s.mSlSet(k, mechHexUB, sl)
						// the slot keeps this record's timestamp: the grant is
						// the object's, so a later control change neither takes
						// it nor lets a later Veil that did not mark the slot
						// refresh it (hexUBColours)
						if int(sl) < len(s.cold.hexUB) {
							s.cold.hexUB[sl] = rec.ts
						}
					}
				}
				return
			}
		}
		s.capFull(StallTurnRepl)
	case kop.TurnReplPlusCreatureCtr:
		// each activation is its own replacement effect, so the count
		// stacks; two sources make the order ask stall (ctrGather)
		c := s.plusCreCold()
		if p := s.res.e.ctrl; c != nil && c.n[p] < 255 {
			if c.n[p] == 0 {
				c.src[p] = s.res.e.obj
			} else if c.src[p] != s.res.e.obj {
				c.src[p] = NoObj
			}
			c.n[p]++
		}
	}
}

// exScopeHit reports whether scope record r makes creature sl's move to the
// graveyard an exile: its controller is an opponent of the effect's.
func (s *State) exScopeHit(k *prep, r *exMarkRec, sl Slot) bool {
	return r.live && r.sl == slotExScope && r.pl != s.perms[sl].ctrl && s.isCreature(k, sl)
}

// exScopeN is how many live scope records make creature sl's move to the
// graveyard an exile. Each is its own CR 616.1 replacement: dieZone counts
// them with the marks and the leave replacements, and a competition is asked
// (graveMoveCompetes, exileOrderOptions) like two marks on one creature.
func (s *State) exScopeN(k *prep, sl Slot) int {
	if DebugTurnReplOff || len(s.cold.exMark) == 0 {
		return 0
	}
	n := 0
	for i := range s.cold.exMark {
		if s.exScopeHit(k, &s.cold.exMark[i], sl) {
			n++
		}
	}
	return n
}

// plusCreRec is the creature counter-plus-one record: per player the count of
// activations, then per player the source of them (NoObj: more than one).
type plusCreRec struct {
	n   [MaxSeats]uint8
	src [MaxSeats]Obj
}

// plusCreCold is the creature counter-plus-one record, nil when the table has
// none.
func (s *State) plusCreCold() *plusCreRec {
	if c := coldCtrP.of(s); c != nil {
		return &c[0]
	}
	return nil
}

// plusCreN is the number of Prairie Dog activations whose replacement puts one
// more of kind counters on sl, and their source (NoObj when two differ).
func (s *State) plusCreN(k *prep, sl Slot, kind int) (uint8, Obj) {
	if DebugTurnReplOff || kind != int(kop.CtrP1P1) {
		return 0, NoObj
	}
	c := s.plusCreCold()
	if c == nil {
		return 0, NoObj
	}
	p := s.perms[sl].ctrl
	if c.n[p] == 0 || !s.isCreature(k, sl) {
		return 0, NoObj
	}
	return c.n[p], c.src[p]
}

// turnReplEnd ends the creature record (the cleanup and the new turn's reset
// both call it) and, with scope, the scope records (the cleanup).
func (s *State) turnReplEnd(scope bool) {
	if c := s.plusCreCold(); c != nil {
		*c = plusCreRec{}
	}
	if scope {
		for i := range s.cold.exMark {
			if r := &s.cold.exMark[i]; r.sl == slotExScope || r.sl == slotExUncAny || r.sl == slotExUncCre || r.sl == slotExPlHex {
				r.live = false
			}
		}
		s.mSlZero(s.k(), mechHexUB)
		clear(s.cold.hexUB)
	}
}

// uncounterScope counts the live uncounter scope records that stop a Counter
// on the spell of stack entry e: its controller's "spells you control can't
// be countered this turn", a creature one only for a creature spell.
func (s *State) uncounterScope(k *prep, e *stackEntry) (n int) {
	if DebugTurnReplOff {
		return 0
	}
	for i := range s.cold.exMark {
		r := &s.cold.exMark[i]
		if !r.live || r.pl != e.ctrl {
			continue
		}
		if r.sl == slotExUncAny || r.sl == slotExUncCre && k.cards[e.card].types&kop.TCreature != 0 {
			n++
		}
	}
	if n > 0 {
		TurnReplStats.Unc.Add(1)
	}
	return n
}

// turnPlayerHex reports whether a live player hexproof record of player p
// stops the spell or ability the current targeting is for: a plain one
// (TurnReplPlayerHex) any, a coloured one (TurnReplHexUB) a source of one of
// its colours (CR 702.11d). The callers have checked that p is no
// controller of the source.
//
//go:noinline
func (s *State) turnPlayerHex(k *prep, p uint8) bool {
	if DebugTurnReplOff {
		return false
	}
	known, c := false, kop.ColorMask(0)
	for i := range s.cold.exMark {
		r := &s.cold.exMark[i]
		if !r.live || r.sl != slotExPlHex || r.pl != p {
			continue
		}
		if r.col == 0 {
			TurnReplStats.HexP.Add(1)
			return true
		}
		if !known {
			known = true
			_, _, c, _, _ = s.tgtSource(k)
		}
		if r.col&c != 0 {
			TurnReplStats.HexP.Add(1)
			return true
		}
	}
	return false
}

// hexUBColours is the colours permanent sl has hexproof from through the
// mechHexUB plane: blue and black while a Veil marked it and that grant no
// later loss of abilities has taken (CR 613.7: the newest of the grants and the
// loss wins). The plane's per-slot timestamp is the marking record's: the grant
// is the object's, so a control change neither takes it nor lets a later Veil
// that did not mark this slot (the permanent was not its controller's at that
// resolution) refresh it. Cold: targetableGen calls it behind genPrep.anyHexUB.
//
//go:noinline
func (s *State) hexUBColours(k *prep, sl Slot) kop.ColorMask {
	if DebugTurnReplOff || !s.mSl(k, mechHexUB, sl) || int(sl) >= len(s.cold.hexUB) {
		return 0
	}
	ts := s.cold.hexUB[sl]
	if ts == 0 {
		return 0
	}
	if lts, lost := s.lossAt(k, sl); lost && lts > ts {
		return 0
	}
	TurnReplStats.HexO.Add(1)
	return kop.ColU | kop.ColB
}
