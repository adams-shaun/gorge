package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A land-type setter (CR 305.7): an override static with SetLandSub
// (RemoveLandTypes$ True with AddType$ basic land types: Spreading Seas,
// Evil Presence). gorge (chars/types.go applyTypeEffect, abilityloss.go,
// mana_activation.go's CR 305.6 block) reads it as:
//
//   - layer 4: every land subtype is removed (landSub, the land-type word
//     vocabulary below), then the override's AddSub is added;
//   - the land loses every ability its rules text generates, as part of that
//     layer-4 effect: the loss has no layer-6 timestamp (gorge lossStamp 0),
//     so an ability granted to the land survives it. The override carries
//     LoseAbilities, and the loss readers (lossAt, loss6, l6Walk,
//     loseAllOnset) take no timestamp from it;
//   - the land has the intrinsic "{T}: Add [c]" of each basic land type it
//     now has (blgMask keeps them under this loss, not under a layer-6 one):
//     manaPCGen's view is the intrinsic abilities alone, its own mana
//     abilities gone (setPC).

// landTypeWords is gorge's land-subtype vocabulary (CR 205.3i: the basic
// land types and effects.chooseNonbasicLandTypes), the words the setter
// strips.
var landTypeWords = map[string]bool{
	"Plains": true, "Island": true, "Swamp": true, "Mountain": true, "Forest": true,
	"Cave": true, "Desert": true, "Gate": true, "Lair": true, "Locus": true, "Mine": true, "Omenpath": true,
	"Planet": true, "Power-Plant": true, "Sphere": true, "Tower": true, "Town": true, "Urza's": true,
}

// DebugLandSetOwnMana is the mutation switch of TestLandTypeSetterLockstep:
// a land under a land-type setter keeps its own mana abilities beside the
// intrinsic ones (the plain AddSub reading).
var DebugLandSetOwnMana bool

// prepLandSet sets g.landSet when some override is a land-type setter, the
// matchup's land subtypes, and one intrinsic-only view per land card and
// colour mask (32 per card): the card's facts with its own mana abilities
// replaced by the intrinsic ones. manaAb names no ability of the card
// (len(Abilities)), so manaAbView and the guarded direct readers see none.
func prepLandSet(t *kop.Table, p *prep, g *genPrep) {
	for i := range t.Overrides {
		g.landSet = g.landSet || t.Overrides[i].SetLandSub
	}
	if !g.landSet {
		return
	}
	for i, n := range t.Subtypes {
		if i < kop.MaxSubtypes && landTypeWords[n] {
			g.landSub = g.landSub.With(i)
		}
	}
	g.setIdx = make([]int16, len(t.Cards))
	for i := range t.Cards {
		g.setIdx[i] = -1
		if t.Cards[i].Types&kop.TLand == 0 || len(t.Cards[i].Abilities) > 127 {
			continue
		}
		g.setIdx[i] = int16(len(g.setPC) / 32)
		base := bareMana(p.cards[i])
		base.manaAb = int8(len(t.Cards[i].Abilities))
		for m := 0; m < 32; m++ {
			v := base
			if m != 0 {
				v.manaCol = kop.ColorMask(m)
				v.manaN, v.manaTap = 1, true
				v.manaMulti = bits.OnesCount8(uint8(m)) > 1
				v.blgFirst = uint8(bits.TrailingZeros8(uint8(m)) + 1)
			} else {
				v.manaAb = -1
			}
			g.setPC = append(g.setPC, v)
		}
	}
}

// bareMana is pc without any mana ability facts.
func bareMana(pc pcard) pcard {
	pc.manaAb, pc.manaCol, pc.manaN = -1, 0, 0
	pc.manaChosen, pc.manaFixedChosen, pc.manaDyn, pc.manaOne = false, false, false, false
	pc.manaTap, pc.manaSac, pc.manaSacOnly = false, false, false
	pc.castSrc = 0
	pc.manaMulti, pc.manaIdent, pc.manaRefl = false, false, false
	pc.blgFirst, pc.rankCol, pc.restrU = 0, 0, 0
	pc.manaXv = [kop.NumManaKinds]uint8{}
	pc.wideN, pc.wideCol, pc.wideChosen, pc.restrFloat = 0, 0, false, false
	pc.manaTyped = false
	pc.painCol, pc.painN, pc.painOnly, pc.painLife = 0, 0, false, false
	pc.grantSac, pc.ctrMana = false, false
	return pc
}

// landSetCard reports whether card c carries a land-type setter static.
func landSetCard(k *prep, c CardID) bool {
	cd := &k.t.Cards[c]
	for i := range cd.Abilities {
		if a := &cd.Abilities[i]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatOverride && a.Static.Override > 0 &&
			k.t.Overrides[a.Static.Override-1].SetLandSub {
			return true
		}
	}
	return false
}

// landSetLost reports whether the permanent at sl has lost its abilities to
// land-type setters alone: genLost holds, and the loss carries no layer-6
// timestamp (lossAt; a face-down permanent's CR 708.2 blank is no such loss).
func (s *State) landSetLost(k *prep, sl Slot) bool {
	if !k.gen.landSet || !s.genLost(k, sl) || s.faceDown(k, sl) {
		return false
	}
	ts, lost := s.lossAt(k, sl)
	return lost && ts == 0
}

// landSetView is manaPCGen's view of slot sl (card c) under a land-type
// setter: the intrinsic abilities of its basic land types alone. A card
// with no view (not printed as a land) stalls StallLandGrant.
//
//go:noinline
func (s *State) landSetView(k *prep, sl Slot, c CardID) *pcard {
	m := s.blgMask(k, sl)
	if DebugLandSetOwnMana && k.cards[c].manaAb >= 0 {
		m |= k.cards[c].manaCol & 0x1f
	}
	g := k.gen
	if g.setIdx[c] < 0 {
		if m != 0 {
			s.capFull(StallLandGrant)
		}
		return &noManaPC
	}
	return &g.setPC[int(g.setIdx[c])*32+int(m&0x1f)]
}

// landSetAlts lists the payment planner's alternatives of unit u when its
// source lost its own abilities to a land-type setter: the intrinsic ability
// of each colour of the unit's mask, in W U B R G order (gorge appends them
// after the listed abilities, and none is listed). It reports false for any
// other source.
func (s *State) landSetAlts(k *prep, pk *planner, u int, out *[planLimitAlts]lsAlt) (int, bool) {
	if !k.gen.landSet || !s.landSetLost(k, pk.units[u]) {
		return 0, false
	}
	n := 0
	for left := pk.types[pk.utyp[u]].mask; left != 0 && n < len(out); left &= left - 1 {
		out[n] = lsAlt{uint8(bits.TrailingZeros8(left)), -1}
		n++
	}
	return n, true
}
