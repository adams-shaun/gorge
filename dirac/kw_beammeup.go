package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Beam me up: "you may cast this card from your graveyard for [cost] if you also
// return a creature you control to its owner's hand. Then exile this spell"
// (gorge rules/beam_me_up.go; the CR at hand predates the keyword). The cast is
// shaped as a flashback graveyard cast (mode bit 2): sfFlashback gives the exile on
// leaving the stack and the cast-from-graveyard answers. The action's target byte
// says it is no flashback, so a card can offer both. A printed cast takes its mana
// from CardDef.AltCost and its return from CardDef.FbCost. A granted one
// (StatBeamGrant) costs the card's mana cost and takes the return from the granter's
// CardDef.FbCost. A StatCantBeBeamedUp static bars a permanent from the pick.

const (
	castBeamTgt      = 1 // a printed beam me up cast, in an AkCast's target byte
	castBeamGrantTgt = 2 // a granted beam me up cast
)

// buildBeamGrant in build.altAdd: a granted beam me up cast.
const buildBeamGrant = 3

// noteBeamCast readies beginCast for AkCast a of mode bit 2. A flashback on a card
// with printed beam me up is the granted instance, at the mana cost.
func (s *State) noteBeamCast(k *prep, a Action) {
	t := a.Target()
	if t == castBeamGrantTgt || t == 0 && k.t.Cards[s.objs[a.Source()].card].BeamMeUp {
		s.g.altCast = true
	}
	s.g.beamGrant = t == castBeamGrantTgt && k.t.Cards[s.objs[a.Source()].card].BeamMeUp
}

// FeatBeamGrantTgt is the target byte of a granted beam me up cast.
const FeatBeamGrantTgt = castBeamGrantTgt

// FeatBeamPrinted reports whether the card of o prints beam me up.
func (s *State) FeatBeamPrinted(o Obj) bool {
	return s.k().t.Cards[s.objs[o].card].BeamMeUp
}

// beamGrantor is p's permanent making a StatBeamGrant grant now and how many do.
func (s *State) beamGrantor(k *prep, p uint8) (sl Slot, n int) {
	if s.active != p || s.step != kop.StepMain1 && s.step != kop.StepMain2 {
		return NoSlot, 0
	}
	sl = NoSlot
	for m := s.statM.and(s.ctrlM[SeatIx(p)]); m.any(); m = m.rest() {
		src := m.first()
		if !s.ovrLoses(k, src) && s.hasBeamGrant(k, src) {
			if n == 0 {
				sl = src
			}
			n++
		}
	}
	return sl, n
}

func (s *State) hasBeamGrant(k *prep, sl Slot) bool {
	cd := &k.t.Cards[s.cardAt(sl)]
	for i := range cd.Abilities {
		if a := &cd.Abilities[i]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatBeamGrant {
			return true
		}
	}
	return false
}

// beamPoolPays: gorge's planner prices no beam me up mode, so only the floating
// pool pays one. The cast's own delve credit prices it like the offer
// (FeatPoolPaysPlus's shape).
func (s *State) beamPoolPays(k *prep, p uint8, o Obj, mana kop.ManaCost) bool {
	a := s.poolAvailPlus(p, nil)
	c := s.objs[o].card
	if s.rbAny(SeatIx(p)) {
		s.restrAvail(k, SeatIx(p), c, s.restrOriginOfObj(o), &a)
	}
	a.delve = s.tbDelveCredit(k, SeatIx(p), c, o, false) // the faces' credit (CR 702.66a)
	cost := s.featCost(k, SeatIx(p), c, o, mana)
	if !a.tbLeafCredited(&cost) {
		cost = s.delveOfferCredit(k, SeatIx(p), c, o, false, cost) // the offer credits the generic too
	}
	return a.affords(&cost, pipMaskOf(&cost), 0)
}

// offerBeamCasts offers the graveyard casts of p's beam me up cards (gorge
// legal_walk_alt.go's beam walk). gorge lists one cast per distinct cost, the
// printed one first (beamMeUpInstances): a card that prints the keyword beside a
// grant gets a second cast at its mana cost unless that is the printed cost, and
// the cast's alt-cost nibble tells the two apart.
func (s *State) offerBeamCasts(k *prep, d *Decision, p uint8, sorc bool, av *avail) {
	pl := &s.pl[p]
	have := false
	grantor := NoSlot
	if k.gen.anyBeamGrant {
		grantor, _ = s.beamGrantor(k, p)
	}
	for i := 0; i < int(pl.graveN); i++ {
		o := pl.grave[i]
		c := s.objs[o].card
		cd := &k.t.Cards[c]
		granted := grantor != NoSlot && cd.Types&(kop.TInstant|kop.TSorcery) != 0 && cd.Cost != (kop.ManaCost{}) &&
			(!cd.BeamMeUp || cd.AltCost != cd.Cost)
		if !cd.BeamMeUp && !granted || !k.cards[c].instant && !sorc && !s.castFlash(k, p, c) {
			continue
		}
		if k.gen.anyCantCast && s.castForbidden(k, p, c, o) {
			continue
		}
		if !have {
			s.availFor(k, p, av)
			have = true
		}
		if cd.BeamMeUp && s.beamCastOffered(k, p, o, cd.AltCost, &cd.FbCost, false, av) {
			d.add(MakeAction(AkCast, uint8(o), castBeamTgt, 2))
		}
		if granted && s.beamCastOffered(k, p, o, cd.Cost, &k.t.Cards[s.cardAt(grantor)].FbCost, true, av) {
			d.add(MakeAction(AkCast, uint8(o), castBeamGrantTgt, 2))
		}
	}
}

// beamCastOffered reports whether p can cast o for one beam me up instance: mana,
// and the return pick (a granted instance's return is barred differently).
func (s *State) beamCastOffered(k *prep, p uint8, o Obj, mana kop.ManaCost, pick *kop.Cost, granted bool, av *avail) bool {
	// the avail is the caller's walk picture: the cast's own delve credit
	// prices its twobrid faces and is restored after (offerPlay's shape)
	saved := av.delve
	defer func() { av.delve = saved }()
	c := s.objs[o].card
	s.g.beamPick = granted
	av.delve = s.tbDelveCredit(k, p, c, o, false) // the faces' credit (CR 702.66a)
	fc := s.adjustCost(k, p, c, o, mana)
	if !av.tbLeafCredited(&fc) {
		fc = s.delveOfferCredit(k, p, c, o, false, fc) // gorge's choke point credits the generic at this walk too
	}
	ok := s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &fc, av) && s.spellTargetsFeasible(k, c, p) &&
		(!granted && k.t.Cards[c].BeamMeUp || s.spellCostsFeasible(k, c, p, o)) && s.pickCostPayable(k, p, pick, o) && s.beamReturnable(k, p, pick, o) && !s.castHeldOut(o)
	s.g.beamPick = false
	return ok && !(s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) != 0 && s.g.availX&availWalk == 0 && !s.beamPoolPays(k, p, o, mana))
}

// beamReturnable reports whether p controls a permanent that pays the cast of o's
// return, as a StatCantBeBeamedUp static leaves it: pickCostPayable counts the
// battlefield without the spell, which the bar reads (beamPick says which kind).
func (s *State) beamReturnable(k *prep, p uint8, pick *kop.Cost, o Obj) bool {
	return pick.Flags&kop.CostReturn == 0 || s.returnCands(k, p, pick.Filter, o, nil) >= int(pick.N)
}

// beamBarred reports whether a StatCantBeBeamedUp static bars the permanent at
// sl from paying a beam me up cast. A static of StBeamGranted bars it only from
// paying for a grant it alone makes (gorge onlyBeamMeUpGrantor).
func (s *State) beamBarred(k *prep, sl Slot, p uint8, granted bool) bool {
	for m := s.statM; m.any(); m = m.rest() {
		src := m.first()
		if s.ovrLoses(k, src) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(src)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatCantBeBeamedUp || !s.matchPermGen(k, a.Static.Filter, sl, s.perms[src].ctrl, src, layerFull) {
				continue
			}
			if a.Static.StFlags&kop.StBeamGranted != 0 {
				if g, n := s.beamGrantor(k, p); !granted || n != 1 || g != src {
					continue
				}
			}
			return true
		}
	}
	return false
}
