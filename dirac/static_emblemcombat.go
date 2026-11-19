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

// anyEmb reports whether any seat holds an emblem (every seat: a third or
// fourth seat's emblem gates the fast paths as a first seat's does).
func (s *State) anyEmb() bool {
	n := uint8(0)
	for p := range s.g.nemb {
		n |= s.g.nemb[p]
	}
	return n != 0
}

// idemEmblemHeld reports whether player p already holds the emblem emb
// (Table.Emblems index+1) and emblemIdem reads it as made once: a second
// copy restricts the same creatures for the same turn, so one record stands
// for both (no projection reads the count) and the emblem pool does not fill
// (Peace in Our Time, six of them each saying yes to the same player's
// beginning of combat).
//
//go:noinline
func (s *State) idemEmblemHeld(k *prep, p, emb uint8) bool {
	e := &k.t.Emblems[emb-1]
	if !emblemIdem(e) {
		return false
	}
	for i := 0; i < int(s.g.nemb[p]); i++ {
		if s.embIdx(p, i) == emb {
			return true
		}
	}
	return false
}

// DebugMustAttackOneOpp is the lockstep mutation of an Effect's MustAttack
// over "creatures your opponents control" (Bident of Thassa): it binds only
// the next live seat's creatures, the one opponent of a two-player game, so
// TestFourSeatMustAttackLockstep must diverge. Never set in production.
var DebugMustAttackOneOpp bool
