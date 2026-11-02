package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Beam me up: "you may cast this card from your graveyard for [cost] if you also
// return a creature you control to its owner's hand. Then exile this spell"
// (gorge rules/beam_me_up.go; the CR at hand predates the keyword). The cast is
// the flashback graveyard cast (mode bit 2): sfFlashback gives the exile on
// leaving the stack and the cast-from-graveyard answers, CardDef.AltCost its mana
// and CardDef.FbCost the return pick. A StatCantBeBeamedUp static bars a
// permanent from the pick.

// offerBeamCasts offers the graveyard casts of p's beam me up cards (gorge
// legal_walk_alt.go's beam walk).
func (s *State) offerBeamCasts(k *prep, d *Decision, p uint8, sorc bool, av *avail) {
	pl := &s.pl[p]
	have := false
	for i := 0; i < int(pl.graveN); i++ {
		o := pl.grave[i]
		c := s.objs[o].card
		cd := &k.t.Cards[c]
		if !cd.BeamMeUp || !k.cards[c].instant && !sorc && !s.castFlash(k, p, c) {
			continue
		}
		if k.gen.anyCantCast && s.castForbidden(k, p, c, o) {
			continue
		}
		if !have {
			s.availFor(k, p, av)
			have = true
		}
		fc := s.adjustCost(k, p, c, o, cd.AltCost)
		if !s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &fc, av) || !s.spellTargetsFeasible(k, c, p) ||
			!s.pickCostPayable(k, p, &cd.FbCost, o) || s.castHeldOut(o) {
			continue
		}
		if s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) != 0 && s.g.availX&availWalk == 0 && !s.FeatPoolPays(p, o, 2) {
			continue // gorge's planner prices no beam me up mode: only the floating pool pays it
		}
		d.add(MakeAction(AkCast, uint8(o), 0, 2))
	}
}

// beamBarred reports whether a StatCantBeBeamedUp static bars the permanent at
// sl from paying a beam me up cast.
func (s *State) beamBarred(k *prep, sl Slot) bool {
	for m := s.statM; m.any(); m = m.rest() {
		src := m.first()
		if s.ovrLoses(k, src) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(src)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCantBeBeamedUp && s.matchPermGen(k, a.Static.Filter, sl, s.perms[src].ctrl, src, layerFull) {
				return true
			}
		}
	}
	return false
}

// FeatBeamMeUpCard reports whether o's card is a beam me up card (the oracle
// canonicalises its mode-2 cast as gorge's "beam_me_up" option).
func (s *State) FeatBeamMeUpCard(o Obj) bool {
	return s.k().t.Cards[s.objs[o].card].BeamMeUp
}
