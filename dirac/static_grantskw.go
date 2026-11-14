package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// grantsKw reports whether some program of t grants keyword bit kw
// (OpGrantKeyword: C the low 16 bits, B.N the high).
func grantsKw(t *kop.Table, kw kop.KwMask) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpGrantKeyword && (kop.KwMask(uint16(in.C))|kop.KwMask(uint16(in.B.N))<<16)&kw != 0 {
					return true
				}
			}
		}
	}
	return false
}

// tableGrantsKw reports whether some part of t can grant keyword bit kw:
// an OpGrantKeyword, a Continuous static (a card's or an emblem's: Hunter's
// Ambush's Effect grant) or an override's AddKw.
func tableGrantsKw(t *kop.Table, kw kop.KwMask) bool {
	if grantsKw(t, kw) {
		return true
	}
	isGrant := func(a *kop.Ability) bool {
		return a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.Kw&kw != 0
	}
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if isGrant(&t.Cards[i].Abilities[j]) {
				return true
			}
		}
	}
	for i := range t.Emblems {
		for j := range t.Emblems[i].Abilities {
			if isGrant(&t.Emblems[i].Abilities[j]) {
				return true
			}
		}
	}
	for i := range t.Overrides {
		if t.Overrides[i].AddKw&kw != 0 {
			return true
		}
	}
	return false
}

// overridesRemoveKw reports whether some override of t removes keywords
// (an OpOverride record's RemoveKw: gorge effDebuff's layer-6 removal):
// the layer-6 walk orders it (prepL6).
func overridesRemoveKw(t *kop.Table) bool {
	for i := range t.Overrides {
		if t.Overrides[i].RemoveKw != 0 {
			return true
		}
	}
	return false
}
