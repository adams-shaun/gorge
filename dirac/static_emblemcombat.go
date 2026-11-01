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
