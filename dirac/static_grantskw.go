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
