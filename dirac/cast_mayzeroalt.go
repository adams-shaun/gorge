package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A non-granting may-play static whose alternative cost is {0} on a zone
// (Dragon's Smile: "Once during each of your turns, you may pay {0} rather
// than pay the mana cost for a spell you cast from exile"; kgen's FreeOnly
// static on Exile, CR 118.9). It opens no zone: gorge (rules mayplay.go
// mayPlayAltCosts, mayPlayCastAlts) lists it only beside the plain cast
// another permission makes, as the "mayplay" option at AltCostIndex 1; the
// spell's other costs still apply. The cast at the alternative uses the
// static's once-per-turn turn (gorge stamps its limit key) and spends the
// plain permission as the plain cast would. Cold: every site sits behind
// genPrep.anyZeroAlt.

// StallZeroAltShared: the {0} alternative would share the kernel mode
// (castMayPlay|castFree) with another cast gorge lists for the card (a free
// or pay-life permission, a second such static at AltCostIndex 2), or sits
// beside a permission whose own price gorge's walk does not list it beside.
// The game stalls loudly rather than charge the wrong cast.
const StallZeroAltShared = 188

func init() { stallNames[StallZeroAltShared] = "zeroaltshared" }

// zeroAltStatic reports whether MayPlayFrom static st is a {0} alternative:
// FreeOnly on a zone other than Weftwalking's hand spelling of "All".
func zeroAltStatic(st *kop.Static) bool {
	return st.StFlags&kop.StFreeOnly != 0 && st.Zone != kop.ZoneHand
}

// zeroAlts is the number of {0}-alternative statics that let p's cast of o
// skip its mana cost (zone, controller, condition, filter, unused turn), and
// the first of them.
func (s *State) zeroAlts(k *prep, p uint8, o Obj) (n int, first statSrc) {
	s.mayPlayStaticsOf(k, p, o, true, func(st statSrc) bool {
		if n == 0 {
			first = st
		}
		n++
		return true
	})
	return n, first
}

// FeatMayZeroAltCast reports whether a castMayPlay|castFree cast of o by p
// is the {0} alternative of a non-granting static (gorge's "mayplay" option
// at AltCostIndex 1; the canonical form follows).
func (s *State) FeatMayZeroAltCast(p uint8, o Obj) bool {
	k := s.k()
	if k.gen == nil || !k.gen.anyZeroAlt || s.objs[o].zone != kop.ZoneExile {
		return false
	}
	n, _ := s.zeroAlts(k, p, o)
	return n > 0
}

// offerZeroAlt offers the {0} alternative of o beside the plain may-play
// cast in mode that a permission makes (plain: gorge's mayPlayPlainGrant
// holds for it, so its walk lists the alternative). A free plain cast, a
// permission priced its own way, or a second static stalls
// (StallZeroAltShared). Options already offered are not repeated (two
// records for one card).
//
//go:noinline
func (s *State) offerZeroAlt(k *prep, d *Decision, p uint8, o Obj, mode uint8, plain, sorc bool, av *avail) {
	if s.objs[o].zone != kop.ZoneExile {
		return
	}
	n, _ := s.zeroAlts(k, p, o)
	if n == 0 {
		return
	}
	if n > 1 || !plain || mode&castFree != 0 {
		s.capFull(StallZeroAltShared)
		return
	}
	act := MakeAction(AkCast, TCode(o), 0, castMayPlay|castFree)
	for i := 0; i < int(d.N); i++ {
		if d.Options[i] == act {
			return
		}
	}
	s.offerPlay(k, d, p, o, castMayPlay|castFree, sorc, false, false, false, false, av)
}

// spendZeroAlt marks the once-per-turn turn of the {0}-alternative static a
// free may-play cast of o by p went through (gorge stamps the static's limit
// key on the cast at the alternative); false when none admits o, so the
// free cast is some other permission's.
//
//go:noinline
func (s *State) spendZeroAlt(k *prep, o Obj, p uint8) bool {
	if s.objs[o].zone != kop.ZoneExile {
		return false
	}
	n, st := s.zeroAlts(k, p, o)
	if n == 0 {
		return false
	}
	if st.a.Static.StFlags&kop.StOncePerTurn != 0 && st.sl != NoSlot {
		s.markUsed(st.obj, uint8(abIndex(k, s.cardAt(st.sl), st.a)))
		s.bld().bfl |= bflUsedMark
	}
	return true
}

// tableZeroAlt reports whether some card of t prints a {0}-alternative
// static (genPrep.anyZeroAlt).
func tableZeroAlt(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatMayPlayFrom && zeroAltStatic(&a.Static) {
				return true
			}
		}
	}
	return false
}

// mayKindMode is the mode of the plain cast a kind record r offers
// (offerMayKind): castFree for a free record.
func mayKindMode(r mayPlay) uint8 {
	if r.kind&kop.MayPlayFree != 0 {
		return castMayPlay | castFree
	}
	return castMayPlay
}

// mayKindPlain reports whether kind record r is a printed-cost permission
// gorge's walk lists the {0} alternative beside (mayPlayPlainGrant): not an
// adventure grant, a raised cast or a pay-life one, each priced its own way.
func mayKindPlain(r mayPlay) bool {
	return r.kind&(kop.MayPlayAdvKind|mayKindRaise) == 0 && !r.payLife()
}
