package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// tableHasStatic reports whether some card of t has a static of kind sk.
func tableHasStatic(t *Table, sk kop.StaticKind) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == sk {
				return true
			}
		}
	}
	for i := range t.Emblems {
		for j := range t.Emblems[i].Abilities {
			if a := &t.Emblems[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == sk {
				return true
			}
		}
	}
	return false
}
