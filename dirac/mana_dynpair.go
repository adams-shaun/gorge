package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A plain {T}: Add {G} beside a {T}: Add {G} for each permanent matching a
// filter (Itlimoc, Cradle of the Sun): gorge's census lists the source as one
// unit with both alternatives, the evaluated one at its count when planning
// (payment_plan.go's evaluated-amount layer), and the planner takes the
// alternative that ranks best per payment. The kernel models it as the gated
// pair of Whisperer of the Wilds (mana_gate.go) whose larger alternative is
// the evaluated count: varPC holds one view per count, manaPCGen picks the
// view by evaluating the second ability's amount (dynManaN, so the
// StallDynMana ceiling holds), and the planner reaches restrPairUnit through
// the same arm. The window answers ability 0 (the plain {G}). kgen lowers
// only a count tapping cannot change (dynamicLargerSameColour).

// prepManaDynPair builds card c's per-count views when it is that pair
// (prepareGen, once per card).
//
//go:noinline
func prepManaDynPair(g *genPrep, c *kop.CardDef, id CardID, pc *pcard) {
	if g.has(id, gfManaVar) || !dynPairShape(c, pc) {
		return
	}
	a := &c.Abilities[dynPairAbility(c)]
	g.setFlag(id, gfManaVar|gfManaDynPair)
	g.smallFor(id).manaVar = int16(len(g.varPC))
	for n := 0; n <= maxDynMana; n++ {
		v := *pc
		if n >= 2 {
			v.wideN, v.wideCol = uint8(n), a.ManaColors&kop.ColAny
		}
		g.varPC = append(g.varPC, v)
	}
	g.anyGatePair = true
}

// dynPairAbility is the index of c's evaluated mana ability (the first
// dynamic one), -1 when it has none.
func dynPairAbility(c *kop.CardDef) int {
	for j := range c.Abilities {
		if a := &c.Abilities[j]; a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 && a.AFlags&kop.AfManaDynamic != 0 {
			return j
		}
	}
	return -1
}

// dynPairShape reports whether c has exactly two mana abilities, a plain {T}
// one of one fixed colour (pc, the card's own view) and, after it, a plain
// {T} evaluated one of that colour.
func dynPairShape(c *kop.CardDef, pc *pcard) bool {
	n := 0
	for j := range c.Abilities {
		if a := &c.Abilities[j]; a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 {
			n++
		}
	}
	dj := dynPairAbility(c)
	if n != 2 || dj < 0 || pc.manaAb < 0 || int(pc.manaAb) >= dj || pc.manaN != 1 || !pc.manaTap || pc.manaChosen || pc.manaMulti || pc.painCol != 0 {
		return false
	}
	a0, a := &c.Abilities[pc.manaAb], &c.Abilities[dj]
	return a0.AFlags == 0 && a0.ManaRestrict == 0 && a0.Cost.Flags == kop.CostTap && a.AFlags&^kop.AfManaDynamic == 0 &&
		a.ManaRestrict == 0 && a.Cost.Flags == kop.CostTap && a.Cost.Mana.MV() == 0 && !a.ManaEach && !a0.ManaEach &&
		a.ManaColors == a0.ManaColors && bits.OnesCount8(uint8(a.ManaColors&0x3f)) == 1
}

// dynPairView is manaPCGen's arm for such a card: the view for the second
// ability's count right now, the plain view for a count of 0 or 1 (the
// evaluated alternative is not larger).
//
//go:noinline
func (s *State) dynPairView(k *prep, sl Slot, c CardID, vi int16) *pcard {
	cd := &k.t.Cards[c]
	n := s.dynManaN(k, &cd.Abilities[dynPairAbility(cd)], sl)
	return &k.gen.varPC[int(vi)+min(max(n, 0), maxDynMana)]
}
