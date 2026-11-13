package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Cost objects chosen at payment: forage (CR 701.61a, "exile three
// cards from your graveyard or sacrifice a Food", kop.CostForage) and
// ExileFromGrave<N/Spec> (kop.CostExileGrave). gorge poses both from
// costObjectAsks (cast_begin.go, CR 601.2h) after the mana window: the
// forage either-or first (cast_asks.go forageAsk: KChoose forage_exile, then
// one forage_food per Food, asked even with one option), the Food recorded
// as a sacrifice, the exile arm appending an Exile<3/Card> part that exAsk
// asks after the sacrifice and discard asks (one KChoose of N cards there,
// one pick per card here; a CARDNAME singleton is paid unasked). The chosen
// objects are paid with the rest of the cost (cast_commit.go: discards, then
// the exiles, then the sacrifices). Every path is guarded by prep.costObj.

const costObjFlags = kop.CostForage | kop.CostExileGrave | kop.CostReturn | kop.CostBlight

// pickForage is build.pickK while the forage either-or is posed (it has
// pkExile's bit, so the answer routes to answerCostObj).
const pickForage = pkForage | pkExile

// costObjPayable reports whether p can pay cost c's forage / exile parts:
// src is the ability's source (a CARDNAME exile), exclude a card being cast
// (never its own exile fodder).
func (s *State) costObjPayable(k *prep, p uint8, c *kop.Cost, src, exclude Obj) bool {
	if c.Flags&kop.CostForage != 0 && s.pl[p].graveN < 3 && s.countSac(k, p, c.Filter, NoSlot) == 0 {
		return false // gorge castparts.go: three graveyard cards or a Food
	}
	if c.Flags&kop.CostExileGrave != 0 && s.exileCands(k, p, c, src, exclude, nil) < int(c.N) {
		return false
	}
	if c.Flags&kop.CostPayEnergy != 0 && !s.energyPayable(p, c) {
		return false
	}
	if c.Flags&kop.CostReturn != 0 && s.returnCands(k, p, c.Filter, src, nil) < int(c.N) {
		return false
	}
	if c.Flags&kop.CostBlight != 0 {
		sl := NoSlot
		if src != NoObj && s.objs[src].zone == kop.ZoneBattlefield {
			sl = s.objs[src].slot
		}
		if s.countSac(k, p, c.Filter, sl) < 1 {
			return false
		}
	}
	return true
}

// graveCands counts (and, with d, offers) p's graveyard cards matching f
// not already picked: FSelf names src only, FOther excludes it.
func (s *State) graveCands(k *prep, p uint8, f kop.FilterID, src, exclude Obj, d *Decision) int {
	var fl kop.FilterFlags
	if f != 0 {
		fl = k.t.Filters[f].Flags
	}
	pl := &s.pl[p]
	n := 0
	for i := 0; i < int(pl.graveN); i++ {
		o := pl.grave[i]
		if o == exclude || fl&kop.FSelf != 0 && o != src || fl&kop.FOther != 0 && o == src || s.picked(o) {
			continue
		}
		if f != 0 && !s.matchCardGen(k, f, s.objs[o].card, s.objs[o].owner(), p, o) {
			continue
		}
		n++
		if d != nil {
			d.add(MakeAction(AkChoose, TCode(o), 0, 0))
		}
	}
	return n
}

// forageFood: the forage was paid with a Food (its sacrifice pick).
func (s *State) forageFood() bool {
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] == pkSac {
			return true
		}
	}
	return false
}

func (s *State) exilePicks() int {
	n := 0
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] == pkExile {
			n++
		}
	}
	return n
}

// costObjOptions poses the next forage / exile pick of the entry being
// built (bsCosts, before the sacrifice and discard picks); false when none
// is owed.
func (s *State) costObjOptions(k *prep, d *Decision) bool {
	e := &s.bld().e
	if e.kind == seTrigger {
		return false
	}
	if s.beholdOptions(k, d) {
		return true // a spell's behold part (act_behold.go)
	}
	pc := s.buildPickCost(k)
	if pc == nil || pc.Flags&costObjFlags == 0 && !pc.PutLibSelf {
		return false
	}
	exclude := NoObj
	if e.kind == seSpell {
		exclude = e.obj
	}
	if pc.PutLibSelf {
		// PutCardToLibFromGrave<1/-1/CARDNAME> (gorge putToLibAsk): the
		// source card in its owner's graveyard is the only candidate, and
		// gorge still asks, so the kernel poses the one-option pick.
		if s.bld().donePicks&pkPutLib != 0 {
			return false
		}
		d.N = 0
		if ob := &s.objs[e.obj]; ob.zone == kop.ZoneGraveyard {
			d.add(MakeAction(AkChoose, TCode(e.obj), 0, 0))
		}
		if d.N == 0 {
			panic("kernel: put-to-library cost no longer payable") // unreachable after the offer gate
		}
		s.bld().pickK = pkPutLib
		d.Kind, d.Sub, d.Player = DkChoose, ChCostPutToLib, e.ctrl
		return true
	}
	if pc.Flags&kop.CostForage != 0 && s.bld().donePicks&pkForage == 0 {
		d.N = 0
		if s.pl[e.ctrl].graveN >= 3 {
			d.add(MakeAction(AkChoose, 0, 0, 0)) // forage_exile
		}
		src := s.srcSlot()
		for m := s.ctrlM[e.ctrl]; m.any(); m = m.rest() {
			sl := m.first()
			if s.sacMatch(k, pc.Filter, sl, e.ctrl, src) {
				d.add(MakeAction(AkChoose, TCode(s.perms[sl].obj), 0, 1)) // forage_food
			}
		}
		if d.N == 0 {
			panic("kernel: forage no longer payable") // gorge aborts the cast; unreachable after the offer gate
		}
		s.bld().pickK = pickForage
		d.Kind, d.Sub, d.Player = DkChoose, ChCostForage, e.ctrl
		return true
	}
	if pc.Flags&kop.CostBlight != 0 && s.bld().donePicks&pkBlight == 0 {
		d.N = 0
		src := s.srcSlot()
		for m := s.ctrlM[e.ctrl]; m.any(); m = m.rest() {
			sl := m.first()
			if s.matchPerm(k, pc.Filter, sl, e.ctrl, src) {
				d.add(MakeAction(AkChoose, TCode(s.perms[sl].obj), 0, 0))
			}
		}
		if d.N == 0 {
			panic("kernel: blight cost no longer payable") // unreachable after the offer gate
		}
		if d.N == 1 {
			s.bld().pickK = pkBlight
			s.recordPick(Obj(d.Options[0].Source()))
			s.bld().pickK = 0
			s.bld().donePicks |= pkBlight
			return s.costObjOptions(k, d)
		}
		s.bld().pickK, s.bld().pickLeft = pkBlight, 1
		d.Kind, d.Sub, d.Player = DkChoose, ChCostBlight, e.ctrl
		return true
	}
	if pc.Flags&(kop.CostForage|kop.CostExileGrave) == 0 {
		return s.returnPickOptions(k, d, pc) // a Return<N/Spec> part (act_energy.go)
	}
	n, f := int(pc.N), pc.Filter
	if exileXCost(pc) {
		n = int(e.x) // ExileFromGrave<X/Spec>: the announced X (cost_exilex.go)
	}
	if pc.ExileGSelf {
		// ExileFromGrave<1/CARDNAME> first (gorge exAsk's unasked singleton),
		// then the N other cards: the source is the first exile pick.
		if !s.picked(e.obj) {
			s.bld().pickK, s.bld().pickLeft = pkExile, 1
			s.recordPick(e.obj)
			s.bld().pickK = 0
		}
		n++
	}
	if pc.Flags&kop.CostForage != 0 {
		if s.forageFood() {
			return false
		}
		n, f = 3, 0
	}
	for {
		left := n - s.exilePicks()
		if left <= 0 {
			return false
		}
		d.N = 0
		if pc.Flags&kop.CostExileCtrlOrGrave != 0 || pc.ExileHand {
			s.exileCands(k, e.ctrl, pc, e.obj, exclude, d) // battlefield, then graveyard, or the hand (cost_exilecands.go)
		} else {
			s.graveCands(k, e.ctrl, f, e.obj, exclude, d)
		}
		s.bld().pickK, s.bld().pickLeft = pkExile, uint8(left)
		if f != 0 && k.t.Filters[f].Flags&kop.FSelf != 0 && n == 1 && d.N == 1 {
			// a CARDNAME singleton: no choice (gorge exAsk)
			s.recordPick(Obj(d.Options[0].Source()))
			s.bld().pickK = 0
			continue
		}
		if int(d.N) < left {
			panic("kernel: exile cost no longer payable")
		}
		d.Kind, d.Sub, d.Player = DkChoose, ChCostExile, e.ctrl
		return true
	}
}

// answerCostObj applies a forage / exile pick.
func (s *State) answerCostObj(a Action) {
	if s.bld().pickK == pkBehold {
		s.answerBeholdPick(a)
		return
	}
	if s.bld().pickK == pickForage {
		s.bld().donePicks |= pkForage
		s.bld().pickK = 0
		if a.Mode() == 1 {
			s.bld().pickK = pkSac
			s.recordPick(Obj(a.Source()))
			s.bld().pickK = 0
		}
		return
	}
	if s.bld().pickK == pkBlight {
		s.bld().donePicks |= pkBlight
		s.recordPick(Obj(a.Source()))
		s.bld().pickK = 0
		return
	}
	if s.bld().pickK == pkPutLib {
		s.bld().donePicks |= pkPutLib
		s.recordPick(Obj(a.Source()))
		s.bld().pickK = 0
		return
	}
	if s.bld().pickK == pkReturn && sneakEntry(&s.bld().e) {
		s.sneakCapture(Obj(a.Source())) // its defender, while it still attacks
	}
	if s.bld().pickK == pkReturn && s.k().gen != nil && s.k().gen.anyNinjutsu && ninjutsuEntry(s.k(), &s.bld().e) {
		s.ninjutsuCapture(Obj(a.Source())) // CR 702.49b (kw_ninjutsu.go)
	}
	s.recordPick(Obj(a.Source()))
	s.bld().pickK = 0 // costObjOptions poses the next one
}

// payBlightPicks puts the -1/-1 counters on each blight pick (CR 701.68;
// gorge cast_commit.go emitBlightCostCounters, before the tap).
func (s *State) payBlightPicks(k *prep) {
	pc := s.buildPickCost(k)
	if pc == nil || pc.Flags&kop.CostBlight == 0 {
		return
	}
	n := int(pc.N)
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] != pkBlight {
			continue
		}
		if ob := &s.objs[s.bld().picks[i]]; ob.zone == kop.ZoneBattlefield {
			s.blightSlot(ob.slot, n)
		}
	}
}

// payExilePicks exiles the chosen graveyard cards (after the discards,
// before the sacrifices: gorge payCast).
func (s *State) payExilePicks() {
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] != pkExile {
			continue
		}
		if o := s.bld().picks[i]; s.objs[o].zone == kop.ZoneGraveyard || s.objs[o].zone == kop.ZoneHand {
			from := s.detach(o) // a hand card is an ExileFromHand cost's (cost_exilecands.go)
			s.placeFrom(o, kop.ZoneExile, false, from, false)
			s.exwCostExilePick(o)
		} else if s.objs[o].zone == kop.ZoneBattlefield {
			s.leave(s.k(), s.objs[o].slot, kop.ZoneExile, false) // an ExileCtrlOrGrave permanent (cost_exilecands.go)
			s.exwCostExilePick(o)
		}
	}
	if e := &s.bld().e; e.kind == seSpell && s.k().t.Cards[e.card].BeholdExile {
		s.payBeholdExilePicks(s.k()) // a BeholdExile cost (act_beholdexile.go)
	}
}

// graveActOK: graveyard ability a of card o can be activated by p (its
// forage / exile parts payable, an "another target" slot not counting o).
func (s *State) graveActOK(k *prep, a *kop.Ability, p uint8, o Obj) bool {
	if !s.costObjPayable(k, p, &a.Cost, o, NoObj) {
		return false
	}
	for i := 0; i < int(a.NTargets); i++ {
		t := &a.Targets[i]
		if t.UpTo || t.TFlags&kop.TgfAtResolution != 0 || t.Kind != kop.TgtCard || t.Filter == 0 || k.t.Filters[t.Filter].Flags&kop.FOther == 0 {
			continue
		}
		var buf [MaxOptions]TCode
		if s.targetCands(k, t, p, NoSlot, o, &buf) < int(max8(t.Min, 1)) {
			return false
		}
	}
	return true
}

// --- a trigger's own forage cost (CR 701.61a; gorge) ---
//
// gorge trigger_cost_forage.go: the trigger_cost ask offers "pay" when the
// payer has three graveyard cards or controls a Food (foragePayable; else
// decline only, execPay's pfNoPay). On pay, forageElectionAsk poses the
// either-or (pay.ForageOptions: forage_exile first, then one forage_food per
// Food) only when two or more arms are live; a lone Food is sacrificed and a
// lone exile arm settles unasked. forageSettle exiles the first three cards
// of the graveyard in zone order (SettleManaForage) or sacrifices the Food.
// A paid forage fires TrigForage after its moves (trig_modes.go;
// gorge's ForageAction marker).

// foragePayableRes: p can forage (three graveyard cards or a Food matching f).
//
//go:noinline
func (s *State) foragePayableRes(k *prep, p uint8, f kop.FilterID) bool {
	return s.pl[p].graveN >= 3 || s.countSac(k, p, f, NoSlot) > 0
}

// forageRes is the pay answer of a PayForage cost.
//
//go:noinline
func (s *State) forageRes(k *prep) {
	p := s.g.pickP
	f := s.genPickFilter(k)
	grave := s.pl[p].graveN >= 3
	foods := s.countSac(k, p, f, NoSlot)
	switch {
	case foods == 0 && !grave:
		s.payDone(k, false) // the gate's read no longer holds: decline, nothing moved
	case foods == 0:
		s.forageExile3(p)
		s.fireKwAct(k, kop.TrigForage, s.g.pickP, 0)
		s.payDone(k, true)
	case foods == 1 && !grave:
		for m := s.ctrlM[p]; m.any(); m = m.rest() {
			if sl := m.first(); s.sacMatchGen(k, f, sl, p, NoSlot) {
				s.forageSacFood(k, sl)
				break
			}
		}
		s.fireKwAct(k, kop.TrigForage, s.g.pickP, 0)
		s.payDone(k, true)
	default:
		s.res.ask = raGenForage
	}
}

// forageResOptions poses the either-or: mode 0 = exile three graveyard
// cards, mode 1 = sacrifice the Food in source (the cast path's encoding).
//
//go:noinline
func (s *State) forageResOptions(k *prep, d *Decision) {
	p := s.g.pickP
	d.Sub = ChCostForage
	if s.pl[p].graveN >= 3 {
		d.add(MakeAction(AkChoose, 0, 0, 0))
	}
	f := s.genPickFilter(k)
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		if sl := m.first(); s.sacMatchGen(k, f, sl, p, NoSlot) {
			d.add(MakeAction(AkChoose, TCode(s.perms[sl].obj), 0, 1))
		}
	}
}

//go:noinline
func (s *State) answerForageRes(k *prep, a Action) {
	if a.Mode() == 1 {
		s.forageSacFood(k, s.objs[Obj(a.Source())].slot)
	} else {
		s.forageExile3(s.g.pickP)
	}
	s.fireKwAct(k, kop.TrigForage, s.g.pickP, 0)
	s.payDone(k, true)
}

// forageExile3 exiles the first three cards of p's graveyard (gorge
// SettleManaForage walks Zone(ZGraveyard) in order).
func (s *State) forageExile3(p uint8) {
	var os [3]Obj
	pl := &s.pl[p]
	for i := 0; i < 3; i++ {
		os[i] = pl.grave[i]
	}
	for _, o := range os {
		from := s.detach(o)
		s.placeFrom(o, kop.ZoneExile, false, from, false)
	}
}

func (s *State) forageSacFood(k *prep, sl Slot) {
	s.fireSac(k, sl)
	s.leave(k, sl, kop.ZoneGraveyard, false)
}
