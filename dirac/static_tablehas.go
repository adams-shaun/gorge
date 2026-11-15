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

// tableHasOvGate reports whether some card of t carries a static with an
// OvGate bit: a static an Animate staticAbilities$ grant lowers as its own
// gated body (kgen animateGrantedStatic, animateGrantStatics; the kernel
// reads it on the grant record's holder, static_ovgate.go grantedStaticFlag).
func tableHasOvGate(t *Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.OvGate != 0 {
				return true
			}
		}
	}
	return false
}
