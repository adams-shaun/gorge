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
const StallZeroAltShared = 189

func init() { stallNames[StallZeroAltShared] = "zeroaltshared" }

// The target byte of the {0} alternative's cast action: it tells the
// alternative from the plain free cast of the same card (a free permission's,
// castMayPlay|castFree with target 0), whose mode it shares. The apply side
// spends the static's once-per-turn mark only for a marked cast. The free
// marker is for an alternative beside a free plain cast, which keeps the
// plain cast's own permission spend (free); the other beside a priced one.
const (
	zeroAltTgt     = 0xfe
	zeroAltFreeTgt = 0xfd
)

// FeatIsZeroAltTgt reports whether target byte t marks a {0} alternative cast
// (the canonical form follows: FeatMayZeroAltCast).
func FeatIsZeroAltTgt(t uint8) bool { return t == zeroAltTgt || t == zeroAltFreeTgt }

// zeroAltStatic reports whether MayPlayFrom static st is a non-granting
// alternative-cost permission (kop.StMayZeroAlt; Dragon's Smile's {0}, As
// Foretold, Demon of Fate's Design's pay-life, Nissa's pay-energy, Conspiracy
// Unraveler's collect-evidence). The Zone
// byte is the reach (mayPlayFromZone), Weftwalking's plain free shape keeps
// StFreeOnly alone.
func zeroAltStatic(st *kop.Static) bool {
	return st.StFlags&kop.StMayZeroAlt != 0
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
// is the alternative of a non-granting static (gorge's "mayplay" option at
// AltCostIndex 1 beside the plain cast another permission makes; the
// canonical form follows). The static's own zone byte is the reach, so the
// zone pre-gate is gone: Dragon's Smile is exile-only, As Foretold's full
// list reaches the hand and the graveyard.
func (s *State) FeatMayZeroAltCast(p uint8, o Obj) bool {
	k := s.k()
	if k.gen == nil || !k.gen.anyZeroAlt {
		return false
	}
	n, _ := s.zeroAlts(k, p, o)
	return n > 0
}

// offerZeroAlt offers the {0} alternative of o beside the plain may-play
// cast in mode that a permission makes (plain: gorge's mayPlayPlainGrant
// holds for it, so its walk lists the alternative). A free plain cast, a
// permission priced its own way, or a second static stalls
// (StallZeroAltShared). Beside a free plain cast the alternative's action
// carries zeroAltFreeTgt, for the two share a mode. The static's own zone
// byte is the reach (mayPlayStaticsOf's mayPlayFromZone gate), so the exile
// pre-gate is gone: As Foretold's full list reaches the graveyard beside a
// permission there too. Options already offered are not repeated (two
// records for one card).
//
//go:noinline
func (s *State) offerZeroAlt(k *prep, d *Decision, p uint8, o Obj, mode uint8, plain, sorc bool, av *avail) {
	n, st := s.zeroAlts(k, p, o)
	if n == 0 {
		return
	}
	if n > 1 || !plain {
		s.capFull(StallZeroAltShared)
		return
	}
	if mode&castFree != 0 && st.a.Static.StFlags&(kop.StMayPayLife|kop.StMayPayEnergy|kop.StMayPayEvidence) != 0 {
		// a priced alternative (Demon's pay-life, Nissa's pay-energy,
		// Conspiracy Unraveler's evidence) beside a free plain cast would
		// share its mode: the build cannot tell the two apart and the cost
		// would go unpaid (zeroAltFreeTgt waives no cost). gorge lists the
		// alternative at its own AltCostIndex either way, so the game stalls
		// loudly instead.
		s.capFull(StallZeroAltShared)
		return
	}
	if st.a.Static.StFlags&kop.StMayPayEvidence != 0 && s.graveTotalMV(k, p) < int(st.a.Static.A) {
		// the graveyard's total mana value does not reach the evidence
		// (gorge's walk prices the alternative's cost and withholds it, CR
		// 701.59a); an unpayable alternative is never offered
		return
	}
	tgt := TCode(zeroAltTgt)
	if mode&castFree != 0 {
		tgt = zeroAltFreeTgt
	}
	act := MakeAction(AkCast, TCode(o), tgt, castMayPlay|castFree)
	for i := 0; i < int(d.N); i++ {
		if d.Options[i] == act {
			return
		}
	}
	n0 := int(d.N)
	s.offerPlay(k, d, p, o, castMayPlay|castFree, sorc, false, false, false, false, av)
	plainAct := MakeAction(AkCast, TCode(o), 0, castMayPlay|castFree)
	for i := n0; i < int(d.N); i++ {
		if d.Options[i] == plainAct {
			d.Options[i] = act // the cast just offered is the alternative
		}
	}
}

// offerMayPlayHandAlt offers the non-granting alternative cost of hand card o
// beside its plain hand cast (gorge legal_walk_hand.go's alternativeCosts
// loop: mayPlayAltCosts entries are "cast" options at AltCostIndex, listed
// whether or not the plain price is payable, so it rides before the plain
// affordability gates like offerAltHand's own alternative). The alternative's
// action carries zeroAltTgt: the apply spends the static's once-per-turn turn
// and pays its cost (payLifeMV, the energy count) over the plain cast below
// (castFlagsGenTgt). A second alternative static stalls (gorge lists each at
// its own AltCostIndex; the kernel's canon cannot name the second one), and
// so does a free non-granting static (gorge lists a free entry first, whose
// cast the kernel canonizes as the AltCostIndex 1 cast) or the card's own
// alternative cost (gorge's own statics take the earlier indexes,
// cast_altfree.go): the game stalls loudly rather than pair two casts on one
// canon code.
//
//go:noinline
func (s *State) offerMayPlayHandAlt(k *prep, d *Decision, p uint8, o Obj, c CardID, sorc bool, av *avail) {
	if k.gen == nil || !k.gen.anyZeroAlt {
		return
	}
	n, st := s.zeroAlts(k, p, o)
	if n == 0 {
		return
	}
	if n > 1 || k.cards[c].altCost {
		s.capFull(StallZeroAltShared)
		return
	}
	fl := st.a.Static.StFlags
	if fl&kop.StMayPayLife != 0 && !s.payLifeOfferable(k, p, o) {
		return // the life covers the mana value beside the spell's own (CR 119.4)
	}
	if fl&kop.StMayPayEnergy != 0 && int(s.g.energy[SeatIx(p)]) < int(st.a.Static.A) {
		return // the energy the alternative owes (act_energy.go energyPayable)
	}
	if fl&kop.StMayPayEvidence != 0 && s.graveTotalMV(k, p) < int(st.a.Static.A) {
		return // the evidence the alternative owes (pay.EvidencePayable)
	}
	if rest, _ := s.mayPlayParts(k, p, o); rest.free {
		// a free non-granting or granting static also admits the card: gorge
		// lists its free entry before this alternative and the kernel
		// canonizes the free cast as the AltCostIndex 1 cast (canon
		// translate.go's hand arm)
		s.capFull(StallZeroAltShared)
		return
	}
	act := MakeAction(AkCast, TCode(o), zeroAltTgt, castMayPlay|castFree)
	for i := 0; i < int(d.N); i++ {
		if d.Options[i] == act {
			return
		}
	}
	n0 := int(d.N)
	s.offerPlay(k, d, p, o, castMayPlay|castFree, sorc, false, false, false, false, av)
	plainAct := MakeAction(AkCast, TCode(o), 0, castMayPlay|castFree)
	for i := n0; i < int(d.N); i++ {
		if d.Options[i] == plainAct {
			d.Options[i] = act // the cast just offered is the alternative
		}
	}
}

// spendZeroAlt marks the once-per-turn turn of the non-granting
// alternative-cost static a free may-play cast of o by p went through (gorge
// stamps the static's limit key on the cast at the alternative); false when
// none admits o, so the free cast is some other permission's. The static
// rides back so the apply pays its cost (payLifeMV, the energy count).
//
//go:noinline
func (s *State) spendZeroAlt(k *prep, o Obj, p uint8) (bool, statSrc) {
	n, st := s.zeroAlts(k, p, o)
	if n == 0 {
		return false, st
	}
	if st.a.Static.StFlags&kop.StOncePerTurn != 0 {
		u, ab, host := s.staticUseKey(k, st)
		s.markUsedBy(u, ab, host)
		s.bld().bfl |= bflUsedMark
	}
	return true, st
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
