package dirac

// AddsNoCounter$ True on a restricted any-colour mana ability (Cavern of
// Souls, Delighted Halfling; CR 106.6): "Spend this mana only to cast ... and
// that spell can't be countered". gorge floats the unit in a restriction
// batch tagged AddsNoCounter; a payment that carves it for a spell sets
// state.FlagNoCounter on the spell's pay-time CastInfo
// (pay.emitRestrictedManaSpend -> cast_commit.go), and a Counter on it stops
// in effects.counterStopped before any replacement competition is asked.
//
// The kernel floats the unit in a batch of its own kind (genPrep.restrV bit
// restrVarNoCtr, set by genPrep.restrVarOf from kop.Ability.ManaNoCounter),
// so it never merges with another source's batch of the same restriction.
// rbSettle, the one place a payment's spend is attributed to batches, stamps
// the spell being paid for (spellNoCounter on its stack entry's remVer, as
// the Carnelian Orb's haste rider does, mana_spentrider.go); uncounterable
// and uncGate read the stamp ahead of the replacement count, as gorge's flag
// is read ahead of CounterAllowed. A copy keeps no stamp (CR 707.10: a copy is
// not cast; copy_gen zeroes remVer but the first-cast bit).

// restrVarNoCtr is the restrV bit of a batch kind whose mana carries
// AddsNoCounter$ (restrVarOf, beside the activated-half index 0..restrVars).
const restrVarNoCtr uint8 = 0x80

// spellNoCounter is the spell entry's remVer bit 7 (bits 0-3: the gained
// keywords, first cast, mana haste; 4-6: the exile marks).
const spellNoCounter objWord = 1 << 7

// NoCounterHeld counts, for the tests (DebugUncounterStats set), the Counters
// the stamp stopped.
var NoCounterHeld int

// DebugNoCounterOff drops the stamp (mutation test only).
var DebugNoCounterOff bool

// noCounterSpent stamps the spell being cast when a payment spent mana of
// restricted batch kind kind and that kind is an AddsNoCounter$ source's.
//
//go:noinline
func (s *State) noCounterSpent(p, kind uint8) {
	g := s.k().gen
	if g == nil || kind == 0 || g.restrV[(kind-1)%restrKinds]&restrVarNoCtr == 0 || DebugNoCounterOff || !s.buildLive() {
		return
	}
	b := s.bld()
	if b.bfl&bflPushed == 0 || s.sp == 0 || b.e.kind != seSpell {
		return
	}
	if e := &s.stack[s.sp-1]; e.kind == seSpell && e.obj == b.e.obj {
		e.remVer |= spellNoCounter
		b.e.remVer |= spellNoCounter // a later syncBuild keeps it
	}
}

// noCounterStamped: the spell of stack entry e was paid with AddsNoCounter$
// mana (gorge state.FlagNoCounter).
func noCounterStamped(e *stackEntry) bool {
	if e.kind == seSpell && e.remVer&spellNoCounter != 0 {
		if DebugUncounterStats {
			NoCounterHeld++
		}
		return true
	}
	return false
}
