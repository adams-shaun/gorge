package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// SpellCast "if at least N mana was spent to cast it"
// (CR 601.2h, 603.2): ValidSA$ / ValidSAonCard$ Spell.ManaSpent <op><n|X>.
// Gorge reads both as fire-time match filters on the cast event
// (trigmatch/cast.go spellCastMatches; ValidSAMatches case 2,
// spellValidSAonCardMatches): CompareIntCount over ManaSpentForCast, the
// pool mana spent since the spell's PutOnStack -- the kernel's recorded
// spend on the spell entry (castSpent, written by histCast before the cast
// triggers fire). X (ValidSAonCard$ only) is the cast spell's PRINTED face
// mana value (Face().ManaValue(), not CR 202.3e's on-stack value); copied.
// This ValidSA gate is a fire-time match filter, not an intervening-if:
// gorge rechecks no such filter on resolution, so neither does the kernel. The gate is
// encoded as kop.DmgGate in Trigger.N (read on SpellCast only under
// Tf2NthSpell, which the lowering refuses beside the gate); 0xff as the
// literal is the X sentinel.

// DebugSpentGateUseMV is the mutation switch of the lockstep
// (dirac/oracle/spentvalidsa_test.go): compare the spell's printed mana
// value instead of its spend; Blaze / Crater's Claws with X split the two.
var DebugSpentGateUseMV bool

// DebugSpentGateLog records every gate decision (the oracle probe's counts).
var DebugSpentGateLog bool

// SpentGateRec is one gate decision: the compared spend, the right side,
// the comparator, whether the right side was X (the printed mana value),
// and the result.
type SpentGateRec struct {
	Spent, Rhs int
	Cmp        kop.DmgCmp
	X          bool
	Admitted   bool
}

// SpentGateLog is the decision log DebugSpentGateLog appends to.
var SpentGateLog []SpentGateRec

// spentGateOK is castTrigGen's cold gate for a SpellCast trigger whose
// Trigger.N carries a spend gate.
//
//go:noinline
func (s *State) spentGateOK(k *prep, t *kop.Trigger, e *stackEntry) bool {
	spent := int(s.castSpent(e.obj))
	if DebugSpentGateUseMV {
		spent = k.t.Cards[e.card].Cost.MV()
	}
	rhs := int(t.N & 0xff)
	x := rhs == 0xff
	if x {
		rhs = k.t.Cards[e.card].Cost.MV() // gorge Face().ManaValue(), the printed cost (opus castSpentAmt's DebugOpusMV read)
	}
	c := kop.DmgCmp(t.N >> 8)
	ok := spentCmp(c, spent, rhs)
	if DebugSpentGateLog {
		SpentGateLog = append(SpentGateLog, SpentGateRec{Spent: spent, Rhs: rhs, Cmp: c, X: x, Admitted: ok})
	}
	return ok
}

func spentCmp(c kop.DmgCmp, a, b int) bool {
	switch c {
	case kop.DmgGE:
		return a >= b
	case kop.DmgLE:
		return a <= b
	case kop.DmgEQ:
		return a == b
	case kop.DmgNE:
		return a != b
	case kop.DmgGT:
		return a > b
	case kop.DmgLT:
		return a < b
	}
	return false
}

// tableSpentGate: some SpellCast trigger carries the gate (Trigger.N
// nonzero without Tf2NthSpell, the only other SpellCast reader of N).
func tableSpentGate(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigSpellCast && a.Trig.N != 0 && a.Trig.Flags2&kop.Tf2NthSpell == 0 {
				return true
			}
		}
	}
	return false
}
