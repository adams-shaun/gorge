package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A "one mana of each colour among <spec>" ability whose cost is a bare {T}
// (kop.AfManaEachColor: Bloom Tender, Faeburrow Elder). gorge's planner
// census prices it as one fixed production (rules paymentPlanEachColourAlt,
// CR 106.1a, 605.1a): a mana of each colour among the matching permanents no
// planned step can remove (paymentPlanReflectedObjectRemovable), so the
// production a step records is the one its resolution names. That is an
// all-at-once source's shape (mana_eachcolour.go): varPC holds one view per
// colour set (32), view m's unit is m's lowest colour and pcard.manaXv the
// rest; view 0, no colour, is no unit and stays a census gap the prefix
// search activates (scriptGaps). manaPCGen picks the live view
// (eachAmongView).
//
// gorge's ward / unless window (windowManaUnits) and its PotentialMana walk
// do not read the planner layer: there the source stays a window gap
// (windowEachGaps, windowPick) and an open production (legal.go).

// StallEachAmongRem: a coloured permanent with a sacrifice- or return-cost
// mana ability is among an each-colour source's matching permanents and
// brings a colour the others do not. gorge's plan prices the production
// without it, its resolution adds it; the kernel's tap adds the view.
const StallEachAmongRem = 142

func init() { stallNames[StallEachAmongRem] = "eachamongrem" }

// debugEachAmongMutate is the test-only mutation switch (0 none; 1 the
// planner view is always view 0: the source is never priced).
// TestGaptapsLockstepMutation requires it to diverge.
var debugEachAmongMutate uint8

// SetDebugEachAmongMutate sets the mutation switch; tests only.
func SetDebugEachAmongMutate(m uint8) { debugEachAmongMutate = m }

// eachAmongPriced: each-colour ability a is one gorge's planner census
// prices: a bare {T}, no spend restriction, one mana of each colour.
func eachAmongPriced(a *kop.Ability) bool {
	return a.AFlags&kop.AfManaEachColor != 0 && a.AFlags&kop.AfLatent == 0 && a.ManaRestrict == 0 && a.ManaN <= 1 &&
		a.Cost.Flags == kop.CostTap && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0
}

// prepEachAmong builds card c's 32 each-colour views (prepareGen, once per
// card) when its each-colour ability is priced and is its only mana ability.
//
//go:noinline
func prepEachAmong(g *genPrep, c *kop.CardDef, id CardID, pc *pcard) {
	j := g.eachCol(id)
	if j < 0 || g.has(id, gfManaVar) || pc.manaAb >= 0 || !eachAmongPriced(&c.Abilities[j]) {
		return
	}
	g.setFlag(id, gfManaVar|gfEachAmong)
	g.smallFor(id).manaVar = int16(len(g.varPC))
	for m := 0; m < 32; m++ {
		v := *pc
		v.manaAb, v.manaN, v.manaTap = int8(j), 1, true
		v.manaSac, v.manaSacOnly, v.manaDyn, v.manaOne, v.manaMulti, v.manaChosen = false, false, false, false, false, false
		v.manaCol, v.manaXv = 0, [kop.NumManaKinds]uint8{}
		if m == 0 {
			v.manaAb = -1
		} else {
			first := bits.TrailingZeros8(uint8(m))
			v.manaCol = kop.ColorMask(1) << first
			for col := first + 1; col < 5; col++ {
				if m&(1<<col) != 0 {
					v.manaXv[col] = 1
				}
			}
		}
		g.varPC = append(g.varPC, v)
	}
	g.anyEachAmong, g.anyTapExtra = true, true
}

// eachAmongView is manaPCGen's arm for a priced each-colour source: the view
// of the colours among its matching permanents no planned step removes.
//
//go:noinline
func (s *State) eachAmongView(k *prep, sl Slot, vi int16) *pcard {
	full, kept := s.eachColorMasks(k, s.perms[sl].ctrl, sl)
	if full != kept {
		s.capFull(StallEachAmongRem)
	}
	if debugEachAmongMutate == 1 {
		kept = 0
	}
	return &k.gen.varPC[int(vi)+int(kept)]
}

// eachColorMasks is the colours sl's each-colour ability reads (full, the
// resolution's: effects eachColorAmongValid) and those of the matching
// permanents no planned step can remove (kept, the planner's).
func (s *State) eachColorMasks(k *prep, p uint8, sl Slot) (full, kept kop.ColorMask) {
	a := &k.t.Cards[s.cardAt(sl)].Abilities[k.gen.eachCol(s.cardAt(sl))]
	for b := s.ctrlM[p&1]; b.any(); b = b.rest() {
		o := b.first()
		if !s.matchPerm(k, a.ManaAmong, o, p, sl) {
			continue
		}
		col := s.colorsAt(k, o)
		full |= col
		if !s.manaRemovable(k, o) {
			kept |= col
		}
	}
	return full & kop.ColAny, kept & kop.ColAny
}

// manaRemovable: the permanent at sl has a mana ability whose cost
// sacrifices or returns a permanent, so a planned step can remove it (gorge
// paymentPlanReflectedObjectRemovable, read wider: the kernel stalls where the
// two readings differ).
func (s *State) manaRemovable(k *prep, sl Slot) bool {
	cd := &k.t.Cards[s.cardAt(sl)]
	for j := range cd.Abilities {
		a := &cd.Abilities[j]
		if a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 && a.Cost.Flags&(kop.CostSacSelf|kop.CostSac|kop.CostReturnSelf|kop.CostReturn) != 0 {
			return true
		}
	}
	return false
}

// windowEachGaps: p's activatable each-colour sources, priced or not: the
// ward / unless window's gaps (gorge's windowManaUnits keeps no planner unit
// for them).
func (s *State) windowEachGaps(k *prep, p uint8) slotMask {
	if !k.gen.anyEachCol {
		return slotMask{}
	}
	return s.gapMask(k, p, gfEachCol)
}
