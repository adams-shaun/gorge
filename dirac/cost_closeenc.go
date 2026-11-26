package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// kop.Cost.CloseEnc: Close Encounter's own additional cost, `S:Mode$ RaiseCost
// | ValidCard$ Card.Self | Activator$ You | Type$ Spell | Cost$ ChooseCard<1/
// Creature.YouCtrl+inZoneBattlefield;Creature.YouOwn+inZoneExile+warped>` —
// "as an additional cost to cast this spell, choose a creature you control or
// a warped creature card you own in exile" (CR 118.3, 601.2b). The choice
// moves and marks nothing: it rides the cast's paid "Revealed" list (gorge
// castPayChoosecost), which the spell body's RevealedCards$ value reads
// through RefChosenEnc (the spell entry's trigObj, value_chosenenc.go). gorge
// offers the cast only with a candidate (pay.NonManaCastable's RevealOrChoose
// gate, spellCostsFeasible here), asks only over several
// (revealCostOrChooseAsk, option kind "choosecost") and takes a lone one
// unasked; a cast that lost its last candidate aborts in the ask (CR 601.2h,
// 733.1). Cold and out of line: table data and the build's pickK / donePicks.

// pkCloseEnc marks the recorded choose-cost pick (CostLoyalty's bit is no
// pick kind on a spell's cost, as CostTap's is none under pkBehold). It
// carries pkExile's bit so the answer routes to answerCostObj. Done-ness is
// recorded on the CostLoyalty bit alone (pkCloseEncDone): pkExile's bit is
// set by other picks' takes (act_forage.go and friends), so the composite
// would read a forage's exile pick as this choice already made.
const (
	pkCloseEnc     = pickKind(kop.CostLoyalty) | pkExile
	pkCloseEncDone = pickKind(kop.CostLoyalty)
)

// ChCostChosenEnc is gorge's "choosecost" KChoose of the choose-cost pick
// (source = the object chosen).
const ChCostChosenEnc ChoiceKind = 217

// warpExiledCard reports whether the card o sits in exile because its own
// warp cast exiled it (gorge warpCostExiled: the most recent exile of o was
// the delayed exile of its own warp cast). The permission warpRecastGrant
// armed when the exile resolved is the kernel's provenance: it names the
// object and the incarnation it exiled, and cleanupGen drops a stale one (the
// card was cast, moved or is a new object, CR 400.7), so a live record on a
// card in exile is exactly that exile. gorge requires no later turn here
// (warpRecastRecord's recast offer does): "a later turn is NOT required to
// choose it".
func (s *State) warpExiledCard(o Obj) bool {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneExile {
		return false
	}
	for i := 0; i < int(s.g.nmay); i++ {
		r := *s.mayAt(i)
		if r.obj == o && r.ver == ob.ver && r.until == kop.DurWarp {
			return true
		}
	}
	return false
}

// chooseEncCands counts (and, with d != nil, lists as AkChoose{source: the
// object}) the choose-cost candidates for p: the creatures p controls on the
// battlefield in battlefield order, then the warped creature cards p owns in
// exile in exile order (gorge pay.RevealOrChooseCandidates' CloseEncounter
// branch: the battlefield is indexed by controller, exile by owner).
func (s *State) chooseEncCands(k *prep, p uint8, d *Decision) int {
	n := 0
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		if s.typesGen(k, sl)&kop.TCreature == 0 {
			continue
		}
		n++
		if d != nil {
			d.add(MakeAction(AkChoose, TCode(s.perms[sl].obj), 0, 0))
		}
	}
	pl := &s.pl[p]
	for i := 0; i < int(pl.exileN); i++ {
		o := pl.exile[i]
		ob := &s.objs[o]
		if ob.owner() != p || k.cards[ob.card].types&kop.TCreature == 0 || !s.warpExiledCard(o) {
			continue
		}
		n++
		if d != nil {
			d.add(MakeAction(AkChoose, TCode(o), 0, 0))
		}
	}
	return n
}

// chooseEncOptions poses the choose-cost pick of the spell being built
// (bsCosts, after the reveal part's ask — gorge costObjectAsks: forage,
// reveal, reveal-or-choose, behold, blight): false when none is owed, the
// choice was made, or the lone candidate was taken unasked.
func (s *State) chooseEncOptions(k *prep, d *Decision) bool {
	e := &s.bld().e
	if e.kind != seSpell || s.bld().donePicks&pkCloseEncDone != 0 || s.bld().altAdd != 0 {
		return false
	}
	pc := s.buildPickCost(k)
	if pc == nil || !pc.CloseEnc {
		return false
	}
	d.N = 0
	n := s.chooseEncCands(k, e.ctrl, d)
	if n == 0 {
		// gorge offers the cast only with a candidate (spellCostsFeasible);
		// one that vanished since aborts the cast here, as its abortCast does
		// (CR 601.2h, 733.1)
		s.reverseCast(k)
		return true
	}
	if n == 1 {
		// gorge's auto-election: exactly N candidates are taken unasked
		// (revealCostOrChooseAsk's battlefield-only arm)
		s.chooseEncTake(k, Obj(d.Options[0].Source()))
		d.N = 0
		return false
	}
	s.bld().pickK, s.bld().pickLeft = pkCloseEnc, 1
	d.Kind, d.Sub, d.Player = DkChoose, ChCostChosenEnc, e.ctrl
	return true
}

// chooseEncTake records the chosen object: the RefChosenEnc reader's answer
// rides the spell entry's trigObj (value_chosenenc.go), gorge's paid list.
func (s *State) chooseEncTake(k *prep, o Obj) {
	e := &s.bld().e
	if e.kind == seSpell && k.gen != nil && k.gen.anyEncRef {
		e.trigObj = o // RefChosenEnc (value_chosenenc.go)
	}
	s.bld().donePicks |= pkCloseEncDone
	s.bld().pickK, s.bld().pickLeft = 0, 0
}

// answerChosenEnc consumes the choose-cost answer.
func (s *State) answerChosenEnc(a Action) {
	s.chooseEncTake(s.k(), Obj(a.Source()))
}

// chooseEncLowers is gorgeLowersRider's arm for a spell whose own additional
// cost is the choose pick. gorge's cast planner declines every RaiseCost
// non-mana extra (paymentPlanCastShapeDetailAltAdd: PlanCostDetail of the
// composed extra is "shape:additional_cost"), so the potential planner prices
// the composition only on its census, the printed sources and the pool, with
// no prefix search for a granted source beside it — the same reading as the
// half-sacrifice arm (cost_sachalf.go) — and its offer gate confirms the
// whole cost first (potential_plan.go abilityPlanUnreservedPart: the plan is
// only a witness after nonManaCastable holds), the candidate existence here.
func (s *State) chooseEncLowers(k *prep, p uint8, c CardID, o Obj, mayPlay, anyColor bool) bool {
	if s.chooseEncCands(k, p, nil) == 0 {
		return false // gorge NonManaCastable's RevealOrChoose gate (pay/castparts.go)
	}
	cost := s.featCost(k, p, c, o, k.cards[c].cost)
	cost = s.mayRiderCost(cost, anyColor)
	return s.ungrantedPaysNoDH(k, p, c, o, &cost) ||
		mayPlay && s.mayPlayPrefixPays(k, p, c, o, anyColor)
}
