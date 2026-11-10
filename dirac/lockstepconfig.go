package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Surface names the option surface a lockstep game offers.
type Surface uint8

const (
	// SurfaceFull offers a play gorge's planner never prices by lowering a
	// potential play, as the oracle reaches it in gorge.
	SurfaceFull Surface = iota
	// SurfaceAutopay offers such a play only when the floating pool pays it.
	SurfaceAutopay
)

// LockstepConfig is the configuration the lockstep verifies against gorge on
// surface: the one on which the kernel and gorge offer and play the same game.
// The caller adds what lockstep leaves to it: NoElide, Mulligans, RecordChance,
// ArrangeRestOrder and AutoDivide, and a Commander game's Commander and
// Commanders; the seats carry no commander (-1, since 0 indexes a seat's first
// deck card) until it sets them.
func LockstepConfig(surface Surface) Config {
	return Config{
		Commanders:          [MaxSeats]int16{-1, -1},
		GorgeAltAdd:         true,
		GorgeUnpriced:       true,
		GorgeMustAttack:     true,
		GorgeAutopaySurface: surface == SurfaceAutopay,
		GorgeLowering:       surface != SurfaceAutopay,
	}
}

// xFolded is cost with an announced x in its generic part, as the window reads it.
func xFolded(cost kop.ManaCost, x int) kop.ManaCost {
	cost.Generic += uint8(cost.XGeneric(x))
	cost.X, cost.XRed = 0, 0
	return cost
}

// xWindowPay pays an announced X through gorge's manual CR 601.2g window, which its
// planner leaves such a cost to (pay.PlanCostDetail "cost:x"): the first useful
// source, repeated until the pool pays. It reports false when the sources run out;
// what the window tapped stays tapped and its mana floats (CR 733.1).
func (s *State) xWindowPay(k *prep, e *stackEntry, cost *kop.ManaCost) bool {
	c := xFolded(*cost, int(e.x))
	anyColor := s.bld().bfl&bflAnyColor != 0
	ok, held := s.windowTapRider(k, e.ctrl, &c, !anyColor, anyColor)
	for col := range held {
		s.pl[e.ctrl].pool[col] += held[col]
	}
	return ok
}

// windowRestrDynMask is the colours of the restricted evaluated-amount ability at sl
// (Helga's), which the census leaves out and the window offers when the payment may
// spend it; 0 otherwise.
func (s *State) windowRestrDynMask(k *prep, p uint8, sl Slot) uint8 {
	c := s.cardAt(sl)
	if k.gen.flags[c]&gfRestrSrc == 0 || k.cards[c].manaAb >= 0 {
		return 0
	}
	d := s.payDescNow(NoObj)
	v, kind, ab := s.restrView(k, p, sl, &k.cards[c], &d)
	if v == nil || kind != rkDyn {
		return 0
	}
	return uint8(ab.ManaColors) & 0x1f
}

// windowRestrDyn taps the source at sl when it is such an ability: its amount floats
// restricted beside the pool, in the colour cost still owes, else the lowest.
func (s *State) windowRestrDyn(k *prep, p uint8, cost *kop.ManaCost, sl Slot) bool {
	mask := s.windowRestrDynMask(k, p, sl)
	if mask == 0 {
		return false
	}
	col := bits.TrailingZeros8(mask)
	for c := 0; c < 5; c++ {
		if mask&(1<<c) != 0 && cost.Pips[c] > s.pl[p].pool[c] {
			col = c
			break
		}
	}
	d := s.payDescNow(NoObj)
	v, _, ab := s.restrView(k, p, sl, &k.cards[s.cardAt(sl)], &d)
	n := v.manaN
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	if !s.floatRestrAb(k, p, sl, ab, col, n) {
		s.capFull(StallRestrSrc)
	}
	return true
}

// settleFollowed gives the triggers that read o live, those on its entry,
// its last known information as it leaves (CR 608.2h, 603.10).
//
//go:noinline
func (s *State) settleFollowed(k *prep, o Obj) {
	var li uint8
	for i := uint8(0); i < s.npend; i++ {
		if p := &s.pend[i]; p.trigObj == o && followsLive(k, p.card, p.ab) {
			if li == 0 {
				li = s.snapLKI(o)
			}
			p.li = li
		}
	}
	for i := 0; i < min(int(s.sp), MaxStack); i++ {
		if e := &s.stack[i]; e.trigObj == o && e.kind == seTrigger && followsLive(k, e.card, e.ab) {
			if li == 0 {
				li = s.snapLKI(o)
			}
			e.li = li
		}
	}
	if s.res.active && s.res.e.trigObj == o && s.res.e.kind == seTrigger && followsLive(k, s.res.e.card, s.res.e.ab) {
		if li == 0 {
			li = s.snapLKI(o)
		}
		s.res.e.li = li
	}
}

// followsLive: ability ab of card c triggers on an object that stays where it is.
func followsLive(k *prep, c CardID, ab uint8) bool {
	abs := k.t.Cards[c].Abilities
	if int(ab) >= len(abs) || abs[ab].Kind != kop.AbTriggered {
		return false
	}
	ev := abs[ab].Trig.Event
	return ev == kop.TrigETB
}

// millShuffling mills p's top n cards on a table where one may be shuffled back
// instead of milled (Progenitus): the n are fixed before any of them moves, so a
// shuffle does not change which cards are milled (CR 701.17a).
//
//go:noinline
func (s *State) millShuffling(k *prep, p uint8, n int, c int) {
	pl := &s.pl[p]
	var top [MaxObj]Obj
	m := copy(top[:min(n, len(top))], pl.lib[pl.libLo:pl.libHi])
	for _, o := range top[:m] {
		pl.libRemove(o)
		s.touch(o)
		s.objs[o].zone = kop.ZoneNone
		s.placeFromK(k, o, kop.ZoneGraveyard, false, kop.ZoneLibrary, false)
		s.millMark(o, c)
	}
}

// lastKnownController answers who controls the permanent r names, which an effect
// instructs to act: its controller on the battlefield, else who last controlled it
// (CR 701.44c, 701.50b): the trigger's snapshot of it, else the effect's controller.
func (s *State) lastKnownController(k *prep, r kop.Ref) uint8 {
	if sl := s.objRef(k, r); sl != NoSlot {
		return s.perms[sl].ctrl
	}
	e := &s.res.e
	if o := s.objRefObj(r); o != NoObj && o == e.trigObj && e.li != 0 {
		return s.lk(e.li).ctrl
	}
	return e.ctrl
}

// plannerPaysCast reports whether gorge's planner has a witness for o's printed cost:
// its census has no granted source and no source with a fee (Barrels of Blasting
// Jelly's {1}), and it does have a restricted source this cast may spend.
func (s *State) plannerPaysCast(k *prep, p uint8, c CardID, o Obj) bool {
	cost := s.featCost(k, p, c, o, k.cards[c].cost)
	if s.g.mayPlayAny {
		cost = anyType(cost) // a may-play under the any-type rider (rules potentialModeCastPlan)
	}
	return s.affordsUngrantedFor(k, p, &cost, NoSlot, payDesc{class: pdSpell, card: c, src: NoSlot, og: s.restrOriginOfObj(o)})
}

// availXWindow is what CR 601.2g's window can make for an X announcement: availFor,
// with a source's best granted mana ability in place of its own (a land under New
// Horizons makes two of one colour). availFor keeps the own ability the planner uses.
func (s *State) availXWindow(k *prep, p uint8, a *avail) {
	// a granted ability that spends only on some payments (Mm'menon's blue for
	// a spell cast from elsewhere than the hand) adds nothing to an ask the
	// building entry cannot pay with it
	a.xd, a.xdOn = s.payDescNow(NoObj), true
	s.availFor(k, p, a)
	a.xdOn = false
	// A deferred mana source (Pyromancer's Goggles' TriggersWhenSpent$ {R}) is
	// absent from the planner census but CR 601.2g's window can tap it, and
	// gorge's xAsk reads the window (mana_deferred.go; availScript adds it when
	// the caller asked for the gap mana).
	if k.gen != nil && k.gen.anyDeferred && s.g.availX&3 == 0 {
		for b := s.gapMask(k, p, gfDeferred); b.any(); b = b.rest() {
			a.addUnits(1<<kop.R, 1)
		}
	}
	if k.gen == nil || k.gen.anyContGrant || len(k.gen.grantPC) == 0 {
		return
	}
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		sl := m.first()
		if !s.grantMay(k, sl) || !s.canTapForMana(k, sl) || !s.ownManaUsable(k, sl) {
			continue
		}
		var gi [maxGrants]int16
		ng := s.grantIdxs(k, sl, &gi)
		if ng == 0 {
			continue
		}
		own := s.manaPCGen(k, sl)
		mask, n := s.srcMaskAt(own, sl), own.manaN
		a.remove(mask, n)
		for i := 0; i < ng; i++ {
			g := s.grantView(k.gen, gi[i], sl)
			mask |= s.srcMaskAt(g, sl)
			n = max(n, g.manaN)
		}
		a.addUnits(mask, n)
	}
}

// addXOptions offers X = 0..mx for the entry being built, less any value the target
// ask that follows could not answer: X is announced before the targets (CR 601.2b-c),
// and a filter bounded by X (Rydia's Saga card with mana value X) has its candidates
// only at some values. When no value has one, the whole range stands and the target
// ask reverses the cast (gorge xAsk).
func (s *State) addXOptions(k *prep, d *Decision, e *stackEntry, mx, hi int, av *avail) bool {
	a := s.buildAbility(k)
	bound := false
	if k.gen != nil && a != nil {
		for i := 0; i < int(a.NTargets); i++ {
			bound = bound || xBoundTarget(k.t, &a.Targets[i]) || xCountTarget(&a.Targets[i])
		}
	}
	picks := s.convokePickCount() > 0
	base := s.buildCost(k)
	anyColor := s.bld().bfl&bflAnyColor != 0
	// pays: X = x is payable. An announced convoke pick pays one generic of
	// the X total, so the pool owes the post-pick cost, and a pick the total
	// cannot absorb is rejected (gorge xAsk, convokeAbsorbs with xOpen false).
	pays := func(x int) bool {
		if !picks {
			return x <= mx || s.xReducedPays(k, av, x)
		}
		c := base
		if k.gen != nil && s.statM.any() {
			c = s.xHypoCost(k, x) // the stat-reduction alternative: the cost at this X
		}
		c, ok := s.convokeXCost(k, c, x)
		if !ok {
			return false
		}
		if x <= mx {
			return true // picks only lower the cost mx already priced
		}
		if anyColor {
			return av.affordsAnyColor(&c, 0)
		}
		return av.affords(&c, pipMaskOf(&c), 0)
	}
	n0 := d.N
	fl := s.xFloor(k) // the announced X's floor (cost_sacx.go)
	if bound {
		for x := fl; x <= hi; x++ {
			if s.xAnswerable(k, a, e, x) && pays(x) {
				d.add(MakeAction(AkChoose, 0, 0, uint8(x)))
			}
		}
	}
	if d.N == n0 {
		for x := fl; x <= hi; x++ {
			if pays(x) {
				d.add(MakeAction(AkChoose, 0, 0, uint8(x)))
			}
		}
	}
	if d.N == n0 && fl > 0 {
		// gorge xAsk: a floored announcement with no payable value aborts the
		// cast without posing a decision (CR 601.2b, 733.1; sacShort's exit)
		d.N = 0
		s.reverseCast(k)
		return false
	}
	if d.N == n0 {
		// gorge xAsk: no payable X leaves the old offer standing, X = 0, and
		// the mana window reverses the cast (CR 601.2b, 733.1); a Play's copy
		// is begun whatever the pool holds
		d.add(MakeAction(AkChoose, 0, 0, 0))
	}
	return true
}

// xAnswerable reports whether ability a's mandatory targets have candidates once X is
// x, for the entry being built.
func (s *State) xAnswerable(k *prep, a *kop.Ability, e *stackEntry, x int) bool {
	src := NoSlot
	s.g.probe = probe{kind: probeSpell, src: NoSlot, card: e.card, ctrl: e.ctrl, x: uint8(x + 1)}
	if e.kind == seAbility {
		src = s.srcSlot()
		s.g.probe = probe{kind: probeAbility, src: src, ctrl: e.ctrl, x: uint8(x + 1)}
	}
	ok := s.abilityTargetsFeasible(k, a, e.ctrl, src, 0xff)
	s.g.probe = probe{}
	return ok
}
