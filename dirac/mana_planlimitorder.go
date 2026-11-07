package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// unitAltsChoice emits unit u's alternatives when the source has a
// choice-shaped mana ability (Produced$ Any, a Combo, a gated Chosen:
// AfManaChoice) and the board grants a basic land type. gorge's census
// (payment_plan.go paymentPlanManaUnitsOnlyCompute) keeps only deterministic
// production in its shared walk, so a printed fixed single-colour production
// and the intrinsic ability of the card's own basic land type precede the
// granted types' intrinsic abilities, and the choice-shaped ability is
// appended to the unit after them, once. It reports false when every ability
// is fixed, leaving unitAlts' single-walk order.
//
// A source whose alternatives outnumber the array (two any-colour abilities
// the payment may spend beside five granted types) stalls the game through
// capFull(StallPlanAlts) with the list cut at the array, never a write past it.
func (s *State) unitAltsChoice(k *prep, pk *planner, u int, out *[planLimitAlts]lsAlt, granted bool) (int, bool) {
	if !granted {
		return 0, false // no granted type on the board
	}
	bm := uint8(s.blgMask(k, pk.units[u]))
	if bm == 0 {
		return 0, false // no granted type to follow
	}
	cd := &k.t.Cards[s.cardAt(pk.units[u])]
	choice := false
	var printed, choiceCols kop.ColorMask
	for j := range cd.Abilities {
		a := &cd.Abilities[j]
		if a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 {
			continue
		}
		if bits.OnesCount8(uint8(a.ManaColors)) == 1 && a.AFlags&kop.AfManaChoice == 0 {
			printed |= a.ManaColors
			continue
		}
		if !a.ManaEach {
			choice = true
			choiceCols |= a.ManaColors
		}
	}
	for c, i := range k.gen.blgSub {
		if i != subOutside && cd.Subtypes.Has(int(i)) {
			printed |= 1 << c // the card's own basic land type: its ability is listed above
		}
	}
	mask := uint8(pk.types[pk.utyp[u]].mask)
	left := mask
	more := bm &^ uint8(printed) & mask
	if !choice || more == 0 {
		return 0, false // no choice shape, or no granted type for it to follow
	}
	n := 0
	// the printed fixed productions and the card's own basic land type's
	// intrinsic ability first (the shared walk's face list), then the granted
	// types' intrinsic abilities
	for j := range cd.Abilities {
		a := &cd.Abilities[j]
		if a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 || a.ManaEach ||
			a.AFlags&kop.AfManaChoice != 0 || bits.OnesCount8(uint8(a.ManaColors)) != 1 {
			continue
		}
		for m := uint8(a.ManaColors) & left; m != 0; m &= m - 1 {
			c := uint8(bits.TrailingZeros8(m))
			ab := int8(j)
			if c < 5 && k.gen.blgSub[c] != subOutside && cd.Subtypes.Has(int(k.gen.blgSub[c])) {
				ab = -1
			}
			out[n] = lsAlt{c, ab}
			n++
			left &^= 1 << c
		}
	}
	// a colour only a choice-shaped ability makes has no intrinsic ability:
	// gorge lists the granted types' intrinsic abilities alone (White Lotus
	// Hideout under an Island grant lists {U}, not WUBRG)
	for left = left&^uint8(choiceCols) | more; left != 0; left &= left - 1 {
		out[n] = lsAlt{uint8(bits.TrailingZeros8(left)), -1}
		n++
	}
	// the choice-shaped abilities after them, each colour its own alternative
	// even when an intrinsic ability already makes that colour: the appended
	// alternative belongs to the printed ability (its step sorts by it,
	// pay.CompareSteps), whatever the card's own basic land type makes
	for j := range cd.Abilities {
		a := &cd.Abilities[j]
		if a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 || a.ManaEach ||
			a.AFlags&kop.AfManaChoice == 0 && bits.OnesCount8(uint8(a.ManaColors)) == 1 ||
			!planChoiceAbListed(a, pk.rkind[u]) {
			continue
		}
		for m := uint8(a.ManaColors); m != 0; m &= m - 1 {
			if n == planLimitAlts {
				s.capFull(StallPlanAlts) // past the array: never a silent cut
				return n, true
			}
			out[n] = lsAlt{uint8(bits.TrailingZeros8(m)), int8(j)}
			n++
		}
	}
	return n, true
}

// StallPlanAlts: a source of the node-limited plan has more alternatives than
// planLimitAlts holds (unitAltsChoice).
const StallPlanAlts = 144

func init() { stallNames[StallPlanAlts] = "planalts" }

// planChoiceAbListed reports whether gorge's census expands choice-shaped
// mana ability a into the unit's alternatives (pay.appendUnitAlternatives):
// a normal-tier ability must have a tap-only cost, so a filter's mana fee
// ({1},{T}: White Lotus Hideout's third ability) is no alternative, and a
// RestrictValid$ one counts only when the payment admits it (restrictedRefused
// withholds it), which the unit's restricted kind records (rk 0: refused).
func planChoiceAbListed(a *kop.Ability, rk uint8) bool {
	if a.Cost.Mana.MV() != 0 || a.Cost.Mana.X != 0 {
		return false
	}
	return !restrSrcAb(a) || rk != 0
}
