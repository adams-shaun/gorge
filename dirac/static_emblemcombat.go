package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Cold, out of line.

// prepEmbComb reports whether some emblem of a generated table carries a
// combat static (CantBlock, CantBlockBy, CantAttack or MustAttack: kgen's
// Effect statics, which gorge effEffect registers as continuous effects of
// the Effect's controller). The combat checks that skip on an empty statM
// consult emblems only then; the generated walks (eachStatic) already read
// emblem statics.
//
//go:noinline
func prepEmbComb(t *kop.Table, p *prep) bool {
	if p.gen == nil {
		return false
	}
	for i := range t.Emblems {
		for j := range t.Emblems[i].Abilities {
			a := &t.Emblems[i].Abilities[j]
			if a.Kind == kop.AbStatic {
				switch a.Static.Kind {
				case kop.StatCantBlock, kop.StatCantBlockBy, kop.StatCantAttack, kop.StatMustAttack:
					return true
				}
			}
		}
	}
	return false
}

// anyEmb reports whether either player has an emblem.
//
//go:noinline
func (s *State) anyEmb() bool { return s.g.nemb[0]+s.g.nemb[1] != 0 }

// idemEmblemHeld reports whether player p already holds the end-of-turn
// emblem emb (Table.Emblems index+1) and every ability of it is a CantAttack
// static: a second copy restricts the same creatures for the same turn, so
// one record stands for both (no projection reads the count) and the emblem
// pool does not fill (Peace in Our Time, six of them each saying yes to the
// same player's beginning of combat).
//
//go:noinline
func (s *State) idemEmblemHeld(k *prep, p, emb uint8) bool {
	e := &k.t.Emblems[emb-1]
	if !e.UntilEOT || e.UntilNextTurn || e.UntilEndOfNextTurn || e.HostBound() {
		return false
	}
	for j := range e.Abilities {
		if a := &e.Abilities[j]; a.Kind != kop.AbStatic || a.Static.Kind != kop.StatCantAttack {
			return false
		}
	}
	for i := 0; i < int(s.g.nemb[p]); i++ {
		if s.embIdx(p, i) == emb {
			return true
		}
	}
	return false
}
