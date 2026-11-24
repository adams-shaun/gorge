package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// genState.prevX holds one rider byte per prevCol record: what the record's
// controller gets from the damage the record prevents (gorge's
// PreventionEffect$ True replacement with a body, damageReplacementPreventsWithRider).
// Layout of the byte:
//
//	bits 0-2  pxRider: the rider kind (pxRidGain, pxRidTok, pxRidCtr,
//	             pxRidDelayCtr, pxRidDmg); 0 is a bare prevention
//	bit  3    pxEOC:     the record ends with the combat (Duration$
//	             UntilEndOfCombat: Suppressor Skyguard); repl_prevscope.go's
//	             prevEndCombat drops it on the way out of the end-of-combat step
//	bit  4    pxFrom:    the shield is keyed on the damage source
//	bit  5    pxPlayers: the shield matches a player recipient only
//	bits 6-7  the rider's player (pxSeatShift)
const (
	pxRider       = 7
	pxRidGain     = 1 // the player gains life equal to the damage prevented
	pxRidTok      = 2 // a token rider: the record's body mints tokens
	pxRidCtr      = 3 // the prevented amount as +1/+1 counters on the rider's target
	pxRidDelayCtr = 4 // the prevented amount accumulated for the end step's P0P1 counters
	pxRidDmg      = 5 // the prevented amount dealt as damage to the rider's target
	pxEOC         = 1 << 3
	pxFrom        = 1 << 4
	pxPlayers     = 1 << 5

	pxSeatShift = 6
)

// the rider's player lives in two bits
const _ = uint(8 - MaxSeats)

// prevRidRec is the per-prevCol-record rider state the hot struct has no
// room for: the rider's stored target code (a ParentTarget shield's
// retribution target) and the delayed-counter accumulation, plus the delayed
// rider's IsPresent$ Creature.IsRemembered gate (Scars of the Veteran). The
// gate is a flag of its own so the target code keeps naming the real target,
// or NoTarget for the shield's own recipient. A cold region keyed by the
// record index; clear at the turn change with the records.
type prevRidRec struct {
	t    TCode
	n    uint8
	gate bool
}

var coldPrevRid = newCold[prevRidRec]("prevrid", func(g *genPrep) int { return prevColN })

// prevRidAt is record i's rider state, nil when the table has no region.
func (s *State) prevRidAt(i int) *prevRidRec {
	r := coldPrevRid.of(s)
	if i < 0 || i >= len(r) {
		return nil
	}
	return &r[i]
}

// StallPrevRider is a rider prevention (gain the prevented life) on an event
// beside anything the kernel does not model: damage that can't be prevented
// (gorge runs the rider and keeps the damage), a parked or paused ask, another
// prevention record, a live PreventNext shield or another damage replacement
// (gorge asks the CR 616.1 order, and a body is never a bare prevention).
const StallPrevRider = 193

func init() { stallNames[StallPrevRider] = "prevrider" }

// DebugNoPrevRider (the rider lockstep mutation): the rider is skipped, and
// the prevention still applies.
var DebugNoPrevRider bool

// DebugNoPrevEndCombat (the lockstep mutation): an end-of-combat scoped
// record is not dropped on the way out of the end-of-combat step.
var DebugNoPrevEndCombat bool

// prevEndCombat drops every scoped record whose rider byte carries pxEOC
// (kop.PrevScopeEndCombat, Duration$ UntilEndOfCombat): gorge's
// expireUntilEndOfCombat runs on the transition that leaves StepEndCombat
// (CR 511.2/511.3; turn.go advance), so a second combat phase in the turn
// starts without it. It frees the slot, it does not spend it, so the next
// combat's record can reuse it.
func (s *State) prevEndCombat() {
	if DebugNoPrevEndCombat {
		return
	}
	for i := range s.g.prevCol {
		if s.g.prevX[i]&pxEOC != 0 {
			s.g.prevCol[i] = prevRec{}
			s.g.prevX[i] = 0
		}
	}
}

// prevXAny reports whether any record carries a rider.
func (s *State) prevXAny() bool {
	for _, x := range s.g.prevX {
		if x&pxRider != 0 {
			return true
		}
	}
	return false
}

// prevRiderMatch reports whether a rider record matches the damage event: a
// scoped record's gain rider (prevRecHit) or a source-keyed PreventNext
// shield's (kop.PrevNextFrom, Awe Strike, matched by the source at sl;
// pnxMatches, which no recipient-keyed record reaches here).
func (s *State) prevRiderMatch(k *prep, sl Slot, c CardID, t prevTgt, combat bool) bool {
	for i := range s.g.prevCol {
		if s.g.prevX[i]&(pxFrom|pxRidGain) == pxFrom|pxRidGain {
			if s.pnxMatches(i, TCode(NoTarget), sl, combat) {
				return true
			}
			continue
		}
		if s.g.prevCol[i].sc&prevNextSc == 0 && s.g.prevX[i]&pxRider != 0 && s.prevRecHit(k, i, sl, c, t, combat) {
			return true
		}
	}
	return false
}

// prevRiderStall stalls StallPrevRider when the event has two or more
// candidates (count) and a rider record matches it: a body is never a bare
// prevention, so gorge asks the order. It reports whether it stalled.
func (s *State) prevRiderStall(k *prep, count int, sl Slot, c CardID, t prevTgt, combat bool) bool {
	if count < 2 || k.gen.anyRepl&rbPrevCol == 0 || !s.prevRiderMatch(k, sl, c, t, combat) {
		return false
	}
	s.capFull(StallPrevRider)
	return true
}

// protBeforeRepl reports whether the recipient (the permanent at sl, a player
// when NoSlot, controlled by rcp) is protected from the source: protection
// prevents the damage before any replacement (CR 702.16e; gorge emit).
func (s *State) protBeforeRepl(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, rcp uint8, sl Slot, combat bool) bool {
	g := k.gen
	return sl != NoSlot && (g.anyRepl&rbProtAll != 0 && s.kw(k, sl)&kop.KwProtectionAll != 0 ||
		g.anyRepl&rbProtCol != 0 && s.protFrom(k, sl, srcSl, srcCard, s.srcColors(k, srcSl, srcCard), combat)) ||
		sl == NoSlot && g.anyRepl&rbPlProt != 0 && s.plProtected(k, srcCtrl, rcp, combat)
}

// pnxOn reports a live PreventNext shield on the recipient code.
func (s *State) pnxOn(code TCode, combat bool) bool {
	for i := range s.g.prevCol {
		if r := &s.g.prevCol[i]; r.sc&prevNextSc != 0 && r.col != 0 && r.ctl == code && (combat || r.sc&prevNextCmb == 0) {
			return true
		}
	}
	return false
}

// pnxFromOn reports a live source-keyed PreventNext shield (kop.PrevNextFrom,
// Awe Strike) on the permanent at srcSl: beside a rider record it is another
// candidate of the event (gorge asks the CR 616.1 order, and a body is never
// a bare prevention).
func (s *State) pnxFromOn(srcSl Slot, combat bool) bool {
	if srcSl == NoSlot {
		return false
	}
	code := TCode(s.perms[srcSl].obj)
	for i := range s.g.prevCol {
		if r := &s.g.prevCol[i]; r.sc&prevNextSc != 0 && r.col != 0 && s.g.prevX[i]&pxFrom != 0 && r.ctl == code && (combat || r.sc&prevNextCmb == 0) {
			return true
		}
	}
	return false
}

// prevRiderApply runs the rider of the prevention record that matches a
// damage event of n (srcCtrl's source, to the recipient code): it gains the
// prevented life for the record's player. The caller prevents the event as for
// a bare record. Protection prevents the damage first and nothing is gained.
// Anything the kernel does not model beside the record stalls StallPrevRider.
func (s *State) prevRiderApply(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, code TCode, rcp uint8, sl Slot, combat bool, n int) {
	if k.gen.anyRepl&rbPrevCol == 0 || !s.prevXAny() {
		return
	}
	t := prevTgt{rcp, sl}
	at, hits := -1, 0
	for i := range s.g.prevCol {
		if s.prevRecHit(k, i, srcSl, srcCard, t, combat) {
			hits++
			if s.g.prevX[i]&pxRider != 0 {
				at = i
			}
		}
	}
	if at < 0 || n <= 0 {
		return
	}
	if s.protBeforeRepl(k, srcCtrl, srcSl, srcCard, rcp, sl, combat) {
		return // prevented before the replacement: no rider
	}
	if s.noPrev(k, combat) || s.g.ro.on || s.parkedAny() || hits > 1 || s.pnxOn(code, combat) || s.pnxFromOn(srcSl, combat) ||
		s.replOthersCompete(k, srcCtrl, srcSl, srcCard, rcp, sl, combat, false) {
		s.capFull(StallPrevRider) // loud: gorge runs the rider where the kernel cannot
		return
	}
	if DebugNoPrevRider {
		return
	}
	if x := s.g.prevX[at]; x&pxRider == pxRidGain {
		s.gainLife(k, x>>pxSeatShift, n)
	}
}

// prevEndStepCounters fires the delayed-counter rider of the Sacred Boon and
// Scars of the Veteran shields at the beginning of the next end step: the
// amount accumulated across the shield's applications goes on its target as
// P0P1 counters (gorge's Effect rider, cards.PreventionRiderDelayedCounterEffect).
// A target no longer on the battlefield takes none.
func (s *State) prevEndStepCounters(k *prep) {
	for i := range s.g.prevCol {
		if s.g.prevX[i]&pxRider != pxRidDelayCtr {
			continue
		}
		r := s.prevRidAt(i)
		if r == nil || r.n == 0 {
			continue
		}
		n := int(r.n)
		r.n = 0
		if r.gate {
			if sl, ok := s.prevRiderSlot(i); !ok || !s.isCreature(k, sl) {
				continue
			}
		}
		if sl, ok := s.prevRiderSlot(i); ok {
			s.putDividedShare(k, sl, kop.CtrP0P1, n)
		}
	}
}
