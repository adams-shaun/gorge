package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A gated land under a granted basic type (CR 305.6, 605.3b): Tainted Field's
// "{T}: Add {C}" and "{T}: Add {W} or {B}. Activate only if you control a Swamp",
// Spire of Industry's "{T}, Pay 1 life: Add one mana of any color. Activate only
// if you control an artifact". The kernel's source view is one of two gate views
// (mana_gate.go varPC: gate off, gate on); the granted intrinsic {c} is one more
// free mana beside the printed {C} (and the gated colours), so each gate view is
// merged with the granted colour set as a plain land's is (prepLandGrant): one
// unit, the printed ability's colour the window's answer (blgFirst), the painful
// gated ability of Spire the planner's last-resort pair (painUnit).
//
// prep-only: the merged views ride genPrep, so State grows no byte.

// DebugLandGrantGateOff is the mutation: no gated land carries the merged
// views, so the refusal is back.
var DebugLandGrantGateOff bool

// DebugLandGrantGateView is the mutation: a gated land's merged views keep the
// printed colours alone, the intrinsic colour dropped.
var DebugLandGrantGateView bool

// gateLandShape: card i is a land of two or more mana abilities, one gated
// (prepManaGate), whose gate views both keep the first ability's unit and whose
// abilities are each one mana of one colour or a colour choice, the gated one
// possibly painful (Spire of Industry's life cost).
func gateLandShape(t *kop.Table, g *genPrep, i int, pc *pcard) bool {
	c := &t.Cards[i]
	if DebugLandGrantGateOff || c.Types&kop.TLand == 0 || g.flags[i]&gfManaGate == 0 || g.flags[i]&(gfFiltFee|gfManaDynPair|gfManaRefl) != 0 {
		return false
	}
	vi := g.manaVar(CardID(i))
	if vi < 0 || pc.manaAb < 0 || pc.manaChosen || pc.manaDyn || pc.ctrMana || pc.manaSac || pc.manaSacOnly || !pc.manaTap {
		return false
	}
	for st := 0; st < 2; st++ {
		if v := &g.varPC[int(vi)+st]; v.manaAb < 0 || !v.manaTap || v.wideN != 0 || v.painOnly {
			return false
		}
	}
	n := 0
	for j := range c.Abilities {
		a := &c.Abilities[j]
		if a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 {
			continue
		}
		n++
		if a.ManaN > 1 || a.ManaRestrict != 0 || a.ManaRider != kop.ManaRiderNone || a.ManaColors&0x3f == 0 {
			return false
		}
	}
	return n == 2
}

// prepLandGrantGate builds the merged views of every gated land of the shape
// (prepLandGrant has run): per card 2 gate states x 32 granted-colour sets.
func prepLandGrantGate(t *kop.Table, p *prep, g *genPrep) {
	if !g.blgOn {
		return
	}
	g.gateIdx = make([]int16, len(t.Cards))
	for i := range t.Cards {
		g.gateIdx[i] = -1
		if !gateLandShape(t, g, i, &p.cards[i]) {
			continue
		}
		vi := int(g.manaVar(CardID(i)))
		g.gateIdx[i] = int16(len(g.gatePC) / 64)
		for st := 0; st < 2; st++ {
			base := g.varPC[vi+st]
			ab := &t.Cards[i].Abilities[base.manaAb]
			for m := 0; m < 32; m++ {
				v := base
				if !DebugLandGrantGateView {
					v.manaCol |= kop.ColorMask(m)
				}
				v.manaN, v.manaTap = 1, true
				v.manaMulti = bits.OnesCount8(uint8(v.manaCol&0x3f)) > 1
				first := bits.TrailingZeros8(uint8(ab.ManaColors & 0x3f))
				v.blgFirst = uint8(first + 1)
				g.gatePC = append(g.gatePC, v)
			}
		}
	}
}

// gateCovers: card i has the merged gate views (the refusal's exception).
func gateCovers(g *genPrep, i int) bool { return g.gateIdx != nil && g.gateIdx[i] >= 0 }

// gateGrantView is manaPCGen's view of the gated land at sl (card c, gate arm vi)
// under a granted basic type: the current gate view with the granted colours its
// own abilities do not make merged in. nil when the card has no merged views or
// the grant adds nothing.
func (s *State) gateGrantView(k *prep, sl Slot, c CardID, vi int16) *pcard {
	g := k.gen
	if g.gateIdx == nil || g.gateIdx[c] < 0 {
		return nil
	}
	gv := s.manaGateView(k, sl, c, vi)
	m := s.blgMask(k, sl) &^ ownMadeCols(gv) & 0x1f
	if m == 0 {
		return nil
	}
	st := 0
	if gv == &g.varPC[int(vi)+1] {
		st = 1
	}
	return &g.gatePC[int(g.gateIdx[c])*64+st*32+int(m)]
}
