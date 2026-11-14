package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Delve (CR 702.66a), printed or granted: a printed K:Delve (CardDef.Delve)
// or Teval, Arbiter of Virtue's "Spells you cast have delve" (a StatStackKw
// Delve grant). The cast is offered on its potential credit, min(generic,
// graveyard size) (gorge delveCredit, castable); the announcement (gorge
// delveAsk) runs after convoke and X and before the pip asks, one graveyard
// card per decision until the shortfall (generic less what the floating pool
// can pay after the coloured pips) is met or Done closes it; the picked
// cards move to exile once the mana is paid (gorge payCast "delved"). The
// picks live in the build's picks / pickKs arrays (kind pkDelve), so a cast
// needs no State field; a 25th pick stalls.

// pkDelve marks a delve pick (build.pickKs). It shares no bit with a kop.Cost
// pickable flag, pkForage, pkWaterbend or pickSubCtr.
const pkDelve pickKind = 0x1000

// StallDelve: more delve picks than the cast build holds, or more graveyard
// options than a decision holds (kw_delve.go).
const StallDelve = 73

func init() { stallNames[StallDelve] = "delve" }

// hasCastDelve is gorge hasCastDelve for the printed or granted keyword: a
// permanent on the battlefield is never a spell being cast.
func (s *State) hasCastDelve(k *prep, p uint8, c CardID, o Obj, fromHand bool) bool {
	if k.gen == nil || !k.gen.anyDelve || s.objs[o].zone == kop.ZoneBattlefield {
		return false
	}
	if k.t.Cards[c].Delve {
		return true
	}
	return s.grantDelve(k, p, c, o, fromHand)
}

// contribPays is tapPays or delve: the shapes gorge's payment
// planner declines as "shape:contribution" (payment_plan.go), so the cast is
// offered only against the floating pool. gorge declines a spell carrying
// delve whatever its graveyard holds.
func (s *State) contribPays(k *prep, p uint8, c CardID, o Obj, fromHand bool) bool {
	return s.tapPays(k, p, c, o, fromHand) || s.hasCastDelve(k, p, c, o, fromHand)
}

// delvePaysEntry reports a spell on the stack cast with delve:
// gorge's planner declines it ("shape:contribution"), so it pays through the
// manual CR 601.2g window like a convoke cast (manaStep).
func (s *State) delvePaysEntry(k *prep, e *stackEntry) bool {
	return k.gen != nil && k.gen.anyDelve && s.hasCastDelve(k, e.ctrl, e.card, e.obj, s.castFromHandBy(e, e.ctrl))
}

// gateDelveCredit is a pool-only tail gate's outer delve credit: the generic
// credit unless the cost is a twobrid one, whose faces the gate's avail
// leaf-credits instead (tbLeafCredited; applying both would price the plain
// generic twice).
func (s *State) gateDelveCredit(k *prep, p uint8, c CardID, o Obj, fromHand bool, cost kop.ManaCost) kop.ManaCost {
	if cost.Hyb[0] == kop.HybTwobrid {
		return cost
	}
	return s.delveOfferCredit(k, p, c, o, fromHand, cost)
}

// delveOfferCredit is cost less the graveyard cards the cast could delve
// away: min(generic, graveyard size) (gorge delveCredit).
func (s *State) delveOfferCredit(k *prep, p uint8, c CardID, o Obj, fromHand bool, cost kop.ManaCost) kop.ManaCost {
	if k.gen == nil || !k.gen.anyDelve || !s.hasCastDelve(k, p, c, o, fromHand) {
		return cost
	}
	cost.Generic -= min(cost.Generic, s.pl[SeatIx(p)].graveN)
	return cost
}

// tbDelveCredit is the graveyard credit a granted-delve cast prices its
// twobrid faces against at an offer gate (gorge FeasibleAny's delve
// argument): the payer's graveyard size, 0 without the grant (hasCastDelve;
// the printed keyword is refused by kgen).
func (s *State) tbDelveCredit(k *prep, p uint8, c CardID, o Obj, fromHand bool) int {
	if !s.hasCastDelve(k, p, c, o, fromHand) {
		return 0
	}
	return int(s.pl[SeatIx(p)].graveN)
}

// tbLeafCredited reports whether a base cost's delve credit already priced
// its twobrid faces (the leaf checks took min(generic, a.delve) per face,
// mana_twobrid.go), so the caller's outer min(generic, graveyard) credit
// (delveOfferCredit) must not apply a second time.
func (a *avail) tbLeafCredited(base *kop.ManaCost) bool {
	return a.delve != 0 && base.Hyb[0] == kop.HybTwobrid
}

// delveTbDeferred reports a twobrid spell whose delve ask waits on the pip
// announcements (gorge delveAsk's CR 601.2b defer, cast_subcounter.go): the
// folded cost still has an unresolved twobrid pip, so the resolved generic
// -- the ask's shortfall -- is not known yet.
func (s *State) delveTbDeferred(k *prep) bool {
	if k.gen == nil || !k.gen.anyDelve {
		return false
	}
	return s.buildIsTb(k) && s.buildCost(k).Hyb[0] == kop.HybTwobrid
}

// delveOfferCreditHand is delveOfferCredit for a spell offered from its hand.
func (s *State) delveOfferCreditHand(k *prep, p uint8, c CardID, o Obj, cost kop.ManaCost) kop.ManaCost {
	return s.delveOfferCredit(k, p, c, o, true, cost)
}

// delvePickCount counts the announced delve picks.
func (s *State) delvePickCount() int {
	n := 0
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] == pkDelve {
			n++
		}
	}
	return n
}

// delveApplyPicks is cost less the announced delve picks, each one generic;
// a pick past the printed generic part reduces the {X} part (XRed). Every
// in-build read of the cost sees the same credit (buildCostResolve).
func (s *State) delveApplyPicks(k *prep, cost kop.ManaCost) kop.ManaCost {
	if k.gen == nil || !k.gen.anyDelve || s.bld().e.kind != seSpell || s.bld().pickN == 0 {
		return cost
	}
	n := s.delvePickCount()
	if n <= int(cost.Generic) {
		cost.Generic -= uint8(n)
		return cost
	}
	ex := n - int(cost.Generic)
	cost.Generic = 0
	cost.XRed = uint8(min(int(cost.XRed)+ex, 255))
	return cost
}

// delveOptions poses the next delve pick: one option per graveyard card not
// yet picked, in zone order, then Done. It is not posed when the cast has no
// delve, no graveyard, or no generic shortfall left (gorge delveAsk).
func (s *State) delveOptions(k *prep, d *Decision) bool {
	e := &s.bld().e
	if k.gen == nil || !k.gen.anyDelve || e.kind != seSpell || !s.hasCastDelve(k, e.ctrl, e.card, e.obj, s.castFromHandBy(e, e.ctrl)) {
		return false
	}
	pl := &s.pl[SeatIx(e.ctrl)]
	if pl.graveN == 0 {
		return false
	}
	if int(s.bld().pickN) >= len(s.bld().picks) {
		s.capFull(StallDelve)
		return false
	}
	cost := s.convokeApplyPicks(k, s.buildCost(k))
	cost = cost.AtX(int(e.x))
	gen := int(cost.Generic)
	if gen <= 0 {
		return false
	}
	// delve covers only generic: reserve the coloured pips out of the pool,
	// and the rest of the pool pays at most its total as generic (gorge
	// delveAsk). A coloured pip the pool cannot pay leaves the whole generic
	// requirement as the shortfall.
	rest := 0
	ok := true
	for i, n := range pl.pool {
		have := int(n)
		if i < len(cost.Pips) {
			if n < cost.Pips[i] {
				ok = false
				break
			}
			have -= int(cost.Pips[i])
		}
		rest += have
	}
	if !ok {
		rest = 0
	}
	if gen-min(gen, rest) <= 0 {
		return false
	}
	d.N = 0
	for i := 0; i < int(pl.graveN); i++ {
		o := pl.grave[i]
		if s.picked(o) {
			continue
		}
		if d.N >= MaxOptions-1 {
			s.capFull(StallDelve)
			break
		}
		d.add(MakeAction(AkChoose, TCode(o), 0, 0))
	}
	if d.N == 0 {
		return false // gorge's multi-pick Max is min(#options, shortfall): every card picked ends the ask
	}
	s.bld().pickK = pkDelve
	d.Kind, d.Sub, d.Player = DkChoose, ChGenDelve, e.ctrl
	d.add(MakeAction(AkDone, 0, 0, 0))
	return true
}

// answerDelvePick records the graveyard card the answer names.
func (s *State) answerDelvePick(a Action) {
	s.bld().pickK = pkDelve
	s.bld().pickLeft = 1
	s.recordPick(Obj(a.Source()))
	s.bld().pickK = 0
}

// endDelveStage closes the announcement.
func (s *State) endDelveStage(k *prep) {
	s.bld().pickK = 0
	if s.bld().e.faceDown() {
		s.toStage(k, bsMana) // a face-down cast skips the pips and targets
		return
	}
	// a twobrid spell's ask ran after its pip announcements (delveTbDeferred);
	// the stage after it is the targets -- back through the pip stage would
	// re-enter the ask (bsPips' tail)
	if s.bld().pipN > 0 && s.buildIsTb(k) {
		s.toStage(k, bsTargets)
		return
	}
	s.nextStage(k)
}

// payDelvePicks exiles the announced cards once the mana is paid (gorge
// payCast: "delved"). Delve is no cost object, so no exiled-with link is made.
// Zone-guarded: a resumed build cannot exile a card twice.
func (s *State) payDelvePicks() {
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] != pkDelve {
			continue
		}
		if o := s.bld().picks[i]; s.objs[o].zone == kop.ZoneGraveyard {
			from := s.detach(o)
			s.placeFrom(o, kop.ZoneExile, false, from, false)
		}
	}
}
