package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A static SpellCast rider's timed Animate of the triggering spell
// (Thundermane Dragon's "If you cast a creature spell this way, it gains
// haste until end of turn"). gorge registers the timed grant on the spell,
// then carries it onto the permanent the spell becomes (CR 400.7a; see
// engine/effects/combatfx.go ExileOnMovedStackCarry and
// engine/rules/layers_effect.go spellCarrySweep). The kernel has no
// battlefield slot for a spell, so the trigger's OpOverride with
// B = kop.OvSpellCarry stamps the triggering spell entry instead
// (stackEntry.ov, the record index + 1) and resolve.go applies that record to
// the permanent as it enters (spellCarryEnter), with the until-end-of-turn
// flag the lowering proved.
//
// The record's source card rides the spell entry's etbX pair: a spell that
// also carries more than the pair of enters-with counter effects would fill
// etbX too, so that combination stalls (StallTables) instead of mixing the
// two. Only the AddKw-only Animate shape lowers to this arm
// (dirac/kgen/lower_statictrigger_spellcast.go).

// SpellCarryStats counts the lockstep test's coverage: rider records stamped
// on a spell and records applied as the permanent entered.
var SpellCarryStats struct {
	Stamped atomic.Int64
	Entered atomic.Int64
}

// DebugSpellCarryOff is TestDemoThundermaneMutation's switch: the stamped
// rider is not applied as the permanent enters, so the lockstep must diverge.
var DebugSpellCarryOff bool

// spellCarryOverride stamps the triggering spell's stack entry with override
// index in.C + 1 and the resolving trigger source's card (for the record's
// source). It reports whether a spell entry was stamped; a trigger object
// that is not a spell on the stack falls through to the ordinary OpOverride
// path.
//
//go:noinline
func (s *State) spellCarryOverride(k *prep, in *kop.Instr) bool {
	if in.C < 0 || int(in.C) >= len(k.t.Overrides) {
		s.capFull(StallTables)
		return true
	}
	if in.C >= 255 {
		// stackEntry.ov is one byte (index + 1): a table past 254 records
		// cannot name this one on a spell entry.
		s.capFull(StallTables)
		return true
	}
	o := s.res.e.trigObj
	if o == NoObj {
		return false
	}
	src := NoCard
	if s.res.e.obj != NoObj {
		src = s.objs[s.res.e.obj].card
	}
	for i := int(s.sp) - 1; i >= 0; i-- {
		e := &s.stack[i]
		if e.kind != seSpell || e.obj != o {
			continue
		}
		if e.ov != 0 {
			if e.ov == uint8(in.C)+1 {
				return true
			}
			s.capFull(StallTables) // two carried records on one spell
			return true
		}
		if e.etbX != [2]uint8{} {
			s.capFull(StallTables) // extra enters-with counter effects already own etbX
			return true
		}
		e.ov = uint8(in.C) + 1
		e.etbX[0], e.etbX[1] = uint8(src), uint8(src>>8)
		SpellCarryStats.Stamped.Add(1)
		return true
	}
	return false
}

// spellCarryEnter applies the rider record stamped on the resolving spell e
// to the permanent at sl as it enters.
func (s *State) spellCarryEnter(k *prep, sl Slot, e *stackEntry) {
	if sl == NoSlot || e.ov == 0 || DebugSpellCarryOff {
		return
	}
	src := CardID(e.etbX[0]) | CardID(e.etbX[1])<<8
	s.applyOverrideFl(k, sl, int(e.ov)-1, src, ovEOT)
	SpellCarryStats.Entered.Add(1)
}
