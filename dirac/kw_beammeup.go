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
	if t := a.Target(); t == castBeamGrantTgt || t == 0 && k.t.Cards[s.objs[a.Source()].card].BeamMeUp {
		s.g.altCast = true
	}
}

// beamGrantor is p's permanent making a StatBeamGrant grant now and how many do.
func (s *State) beamGrantor(k *prep, p uint8) (sl Slot, n int) {
	if s.active != p || s.step != kop.StepMain1 && s.step != kop.StepMain2 {
		return NoSlot, 0
	}
	sl = NoSlot
	for m := s.statM.and(s.ctrlM[p&1]); m.any(); m = m.rest() {
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
// pool pays one.
func (s *State) beamPoolPays(k *prep, p uint8, o Obj, mana kop.ManaCost) bool {
	a := s.poolAvailPlus(p, nil)
	c := s.objs[o].card
	if s.pl[p&1].poolR != 0 {
		s.restrAvail(k, p&1, c, s.restrOriginOfObj(o), &a)
	}
	cost := s.featCost(k, p&1, c, o, mana)
	return a.affords(&cost, pipMaskOf(&cost), 0)
}

// offerBeamCasts offers the graveyard casts of p's beam me up cards (gorge
// legal_walk_alt.go's beam walk).
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
		granted := grantor != NoSlot && cd.Types&(kop.TInstant|kop.TSorcery) != 0 && cd.Cost != (kop.ManaCost{})
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
		mana, pick, tgt := cd.AltCost, &cd.FbCost, uint8(castBeamTgt)
		if !cd.BeamMeUp {
			mana, pick, tgt = cd.Cost, &k.t.Cards[s.cardAt(grantor)].FbCost, castBeamGrantTgt
			s.g.beamPick = true
		}
		fc := s.adjustCost(k, p, c, o, mana)
		ok := s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &fc, av) && s.spellTargetsFeasible(k, c, p) &&
			(cd.BeamMeUp || s.spellCostsFeasible(k, c, p, o)) && s.pickCostPayable(k, p, pick, o) && !s.castHeldOut(o)
		s.g.beamPick = false
		if !ok || s.cfg&(cfgGorgeAutopay|cfgGorgeLowering) != 0 && s.g.availX&availWalk == 0 && !s.beamPoolPays(k, p, o, mana) {
			continue
		}
		d.add(MakeAction(AkCast, uint8(o), tgt, 2))
	}
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

// StallBeamTwice: a printed beam me up card beside a beam me up grant. gorge lists
// a cast per distinct cost, and a canonical cast of the mode names one.
const StallBeamTwice = 73

func init() { stallNames[StallBeamTwice] = "beamtwice" }

// tableBeamTwice reports a printed beam me up card beside a beam me up grant.
func tableBeamTwice(t *kop.Table) bool {
	beam := false
	for i := range t.Cards {
		beam = beam || t.Cards[i].BeamMeUp
	}
	return beam && tableHasStatic(t, kop.StatBeamGrant)
}
