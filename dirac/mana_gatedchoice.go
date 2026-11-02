package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// a gated two-colour choice beside {T}: Add {C} (the MSH
// "basic-land" cycle: Dark Fortress, Gathering Place, Gleaming Bastion,
// Hidden Lair, Training Compound), Mirrex's union generalised. gorge's
// census lists one unit, the {C} alternative then one per colour of the
// Combo ability; the kernel's gate-on view is the union C|c1|c2 (manaMulti),
// the gate-off view {C} alone (prepManaGate).

// DebugGatedChoice is the TestGatedchoiceMutation switch: the union drops
// the choice colours (the gate-on view equals the gate-off one).
var DebugGatedChoice bool

// gatedChoiceUnion reports whether the gated ability gj of c is a choice
// among two or more colours beside a {C}-only first ability (off), and the
// gate-on union view's colours. Prep-time only (once per card).
//
//go:noinline
func gatedChoiceUnion(c *kop.CardDef, gj int, off pcard) (kop.ColorMask, bool) {
	a := &c.Abilities[gj]
	if off.manaCol != kop.ColorMask(1)<<kop.C || a.ManaColors&^kop.ColAny != 0 || bits.OnesCount8(uint8(a.ManaColors)) < 2 {
		return 0, false
	}
	if a.AFlags&^kop.AfManaChoice != 0 && a.ManaColors != kop.ColAny {
		return 0, false
	}
	if DebugGatedChoice {
		return off.manaCol, true
	}
	return off.manaCol | a.ManaColors, true
}

// gatedChoiceFlat reports whether a gate-on view's colours are a {C}-plus-
// colours choice unit (Mirrex, the MSH lands) rather than a Verge's two
// colours: gorge flattens the unit's abilities into one colour ask, {C} then
// each colour, and the oracle's window answer takes the lowest colour, not
// ability 0 (a Verge never produces {C} beside another colour, kgen refuses).
func gatedChoiceFlat(on kop.ColorMask) bool { return on&(kop.ColorMask(1)<<kop.C) != 0 }
