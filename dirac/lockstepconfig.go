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
		Commanders:          noCommanders(),
		GorgeAltAdd:         true,
		GorgeUnpriced:       true,
		GorgeMustAttack:     true,
		GorgeAutopaySurface: surface == SurfaceAutopay,
		GorgeLowering:       surface != SurfaceAutopay,
	}
}

// xFolded is cost with an announced x folded in, as the window reads it.
func xFolded(cost kop.ManaCost, x int) kop.ManaCost {
	return cost.AtX(x)
}

// xWindowPays: an announced-X activation pays through gorge's manual window
// (xWindowPay) unless gorge armed its planner's witness at the least X and the
// announced X re-plans (rules armAbilityAutoPay at planAbilityPayment's X
// floor, then rearmAutoPay; CR 602.2b), when the planner pays it instead.
func (s *State) xWindowPays(k *prep, e *stackEntry, a *kop.Ability, sl Slot, cost *kop.ManaCost, saved *[len(build{}.picks)]uint8) bool {
	if cost.X == 0 || s.cfg&cfgGorgeSurface == 0 {
		return false
	}
	if k.gen != nil && k.gen.anyWaterbend && waterbendIsX(a) || hybridX(a) {
		return true // gorge's planner keeps declining a Waterbend<X> or hybrid {X} cost (planAbilityPayment)
	}
	floor := xFolded(s.abPlannedCost(k, a, sl, e.ctrl), costXMin(&a.Cost))
	return !s.abilityPlanSettles(k, e, floor, floor, *cost, saved)
}

// xWindowPay pays an announced X through gorge's manual CR 601.2g window, which its
// planner leaves such a cost to (pay.PlanCostDetail "cost:x"): the first useful
// source, repeated until the pool pays. It reports false when the sources run out;
// what the window tapped stays tapped and its mana floats (CR 733.1). The window
// has no auto-fill option (the oracle's seat never sends Intent.Announce, so
// manaWindowAsk poses the ordinary window), so gorge's answerer
// (payexec.WindowAnswer) takes the first listed source -- a planner-deferred one
// (Twitching Doll, a depletion land) in its battlefield place, the unit
// availXWindow counted for the announcement.
func (s *State) xWindowPay(k *prep, e *stackEntry, cost *kop.ManaCost) bool {
	c := xFolded(*cost, int(e.x))
	anyColor := s.bld().bfl&bflAnyColor != 0
	ok, held := s.windowTapWalk(k, e.ctrl, &c, &c, !anyColor, anyColor, false, false, true, false, true)
	for col := range held {
		s.pl[e.ctrl].pool[col] += held[col]
	}
	return ok
}

// hybridX: a's mana cost carries {X} and a two-colour hybrid pip, which
// gorge's planner declines (planAbilityPayment; Fiend Artisan's {X}{B/G}).
func hybridX(a *kop.Ability) bool {
	return a.Cost.Mana.X != 0 && a.Cost.Mana.Hyb[0] != 0 && a.Cost.Mana.Hyb[0] != kop.HybTwobrid
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
	var ab *kop.Ability
	var n uint8
	if mask != 0 {
		d := s.payDescNow(NoObj)
		var v *pcard
		v, _, ab = s.restrView(k, p, sl, &k.cards[s.cardAt(sl)], &d)
		n = v.manaN
	} else if ab, n = s.restrDynBesideGrant(k, sl); ab != nil {
		mask = uint8(ab.ManaColors) & 0x1f // the payment may not spend it, but gorge's ask lists it first (mana_grantrestrictedown.go)
	}
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
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.markTapped(k, sl)
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
		s.placeFromK(k, o, kop.ZoneGraveyard, false, kop.ZoneLibrary, false, false)
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
	return s.affordsUngrantedFor(k, p, &cost, NoSlot, payDesc{class: pdSpell, card: c, src: NoSlot, og: s.restrOriginOfObj(o)}) &&
		!s.dhWithholdsCast(k, p, o, &cost) // the Diamonds alone fund the hand cast: gorge's planner prices no plan (census_incomplete)
}

// availXWindow is what CR 601.2g's window can make for an X announcement: availFor,
// with a source's best granted mana ability in place of its own (a land under New
// Horizons makes two of one colour). availFor keeps the own ability the planner uses.
func (s *State) availXWindow(k *prep, p uint8, a *avail) {
	// a granted ability that spends only on some payments (Mm'menon's blue for
	// a spell cast from elsewhere than the hand) adds nothing to an ask the
	// building entry cannot pay with it
	a.xd, a.xdOn = s.payDescNow(NoObj), true
	a.xPlain = s.cfg&cfgGorgeSurface == 0
	// gorge's xAsk takes an X its planner refuses when the cast window pays it
	// (castWindowPays), and that window taps a painful source whatever the
	// life (the damage is lethal only afterwards, CR 704.5a): Cephalid
	// Coliseum at 1 life still buys Mask of the Jadecrafter's X one more.
	x := s.g.availX
	if k.painMana && !a.xPlain {
		s.g.availX |= availWinCol
	}
	// The window's census counts a dynamic {T}, sacrifice source's evaluated
	// amount (Lotus Blossom's petals: gorge cast_window.go
	// castWindowProbeUnits' free dynamic-amount arm) that the planner census
	// leaves out (availForMode's dynSac gate): the window can activate it, so
	// the ask counts the unit. The payment plan itself never prices the
	// source for a cast (plan's castPay gate), so a larger X is paid through
	// the manual window, as gorge's castWindowPays reads it.
	a.dynSac = true
	s.availFor(k, p, a)
	a.dynSac = false
	s.g.availX = x
	a.xdOn, a.xPlain = false, false
	// A deferred mana source (Pyromancer's Goggles' TriggersWhenSpent$ {R}) is
	// absent from the planner census but CR 601.2g's window can tap it, and
	// gorge's xAsk reads the window (mana_deferred.go; availScript adds it when
	// the caller asked for the gap mana).
	if k.gen != nil && k.gen.anyDeferred && s.g.availX&3 == 0 {
		for b := s.gapMask(k, p, gfDeferred); b.any(); b = b.rest() {
			if s.pickCommittedSrc(k, b.first()) {
				continue // a convoke/waterbend pick taps it (excludePickedSources)
			}
			a.addUnits(s.deferMask(k, b.first()), 1)
		}
	}
	// A Combo Any source (Axebane Guardian) is a census gap too, and the window
	// taps it for its amount of any-colour mana (mana_comboany.go).
	if k.gen != nil && k.gen.anyComboAny && s.g.availX&3 == 0 {
		s.availCA(k, p, a)
	}
	// A life-only source (Treasonous Ogre's "Pay 3 life: Add {R}") is a census
	// gap too, and the window activates it (mana_lifegap.go).
	if k.gen != nil && k.gen.uses.has(useLifeGap) && s.g.availX&3 == 0 {
		s.availLifeGap(k, p, a)
	}
	// A counter-rider or Taps-deferred source (Twitching Doll's "{T}: Add one
	// mana of any color. Put a nest counter on this", a Taps watcher's Defer
	// source) is deferred by the planner the same way, and CR 601.2g's window
	// still taps it (mana_rider.go, mana_tapsdefer.go): gorge's xAsk reads the
	// window, so the ask counts this unit too.
	//
	// Only a source availFor actually skipped is added: planDeferred is the
	// same predicate deferSkip keys the plain walk on. Grove of the
	// Burnwillows is a plain first ability beside a later rider, so it is not
	// planner-deferred -- availFor already counts its {C}, and adding the
	// rider's {R}/{G} too made one land tap twice (Treasure Vault's {X}{X}
	// offered X = 4 with seven sources). The walk modes count the source
	// themselves (deferSkip is off there), so they add nothing here either.
	if s.g.availX&(3|availWalk|availHyp) == 0 {
		for b := s.scriptTapsDefer(k, p); b.any(); b = b.rest() {
			if s.pickCommittedSrc(k, b.first()) {
				continue // a convoke/waterbend pick taps it (excludePickedSources)
			}
			if k.gen.anyContGrant && s.g.availX&availPlain == 0 && !s.g.noGrant &&
				s.grantIdx(k, b.first()) >= 0 && s.planDeferred(k, p, b.first()) && !s.blgIntrinsicUnit(k, b.first()) {
				// availFor's dual deferGap arm already counted this source through
				// its granted ability (availGrant, mana.go: the census counts the
				// grant only when noGrant's prefix search is not pricing the ask,
				// hence the two extra terms): the permanent taps once for the
				// window (CR 605.1a), and gorge's census lists one unit for it.
				// Without them, The Rollercrusher Ride's X ask offered one too
				// many (choose:x(-,-,10) beside gorge's 9). When the grant is
				// priced out, availFor skips the source uncounted and this walk
				// is its modelled gap, so it keeps the unit.
				continue
			}
			if ra, ok := s.riderAbilityOf(k, b.first()); ok {
				if ra.ManaRider == kop.ManaRiderDeplete {
					// A depletion land's activation costs a counter, so gorge's window
					// census (windowManaUnits: a free cost only; castWindowProbeUnits
					// withholds SubCounter) never counts it in an X announcement.
					continue
				}
				if !s.planDeferred(k, p, b.first()) {
					// a plain-first source (Thalakos Lowlands) is counted at its plain
					// tap; the window can tap its rider for the colours instead
					if alt, ok := s.riderFaceAlt(k, p, b.first(), ra); ok {
						a.addRiderFace(uint8(s.srcMaskAt(s.manaPCGen(k, b.first()), b.first())), alt)
					}
					continue
				}
				s.addTapsDeferUnits(k, b.first(), a) // its own production, an all-at-once pair included (mana_rider.go)
				continue
			}
			if s.askColDeferred(k, b.first()) {
				s.askColUnits(k, b.first(), a) // mana_askcolour.go
				continue
			}
			if s.restrTapsGapShape(k, b.first()) {
				// A restricted Taps-deferred gap (a Powerstone token under Relic
				// Bind) floats its mana restricted, so it funds the X ask only
				// when its restriction admits the payment being built (CR 106.6:
				// a Powerstone's {C} is never spent on a nonartifact spell).
				ab := &k.t.Cards[s.cardAt(b.first())].Abilities[k.gen.restrRecOf(s.cardAt(b.first())).ab]
				if !s.restrSrcAdmits(k, p, b.first(), ab, &a.xd) {
					continue
				}
			}
			pc := s.manaPCGen(k, b.first())
			if s.splitDeferred(k, b.first()) {
				pc = s.splitWindowPC(k, b.first()) // gorge's window census reads a Condition-split root alone (mana_condsplit.go)
			}
			a.addUnits(s.srcMaskAt(pc, b.first()), srcYield(pc))
			// gorge's windowTapExtras folds the tap's reflected extra (Mana
			// Flare's one mana of the type the land produced) into every priced
			// alternative, a deferred source's own (paymentPlanTapExtraMana):
			// the same fixed carrier vector the census adds to a counted
			// source's tap (availForMode's srcExtra units, mana.go). A shape
			// srcExtra cannot price (a choice, painful or all-at-once
			// production) carries no vector, as gorge's window census gives a
			// multi-symbol or Any alternative none, so the ask reads the unit
			// without it and no stall the plain census never posed.
			if k.gen != nil && k.gen.anyTapExtra {
				if xv, hit := s.srcExtraHit(k, b.first(), pc); hit && xv != ([kop.NumManaKinds]uint8{}) &&
					s.tapExtraPlain(pc) && pc.painCol == 0 {
					for c := range xv {
						if xv[c] > 0 {
							a.addUnits(1<<c, xv[c])
						}
					}
				}
			}
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

// xFoldCost is the cost the X ask prices at x, the announced tap-paid picks
// folded on: the stat-reduction composition at x (gorge paymentManaX's
// mods), then a Waterbend<X> pick's or an announced convoke/harmonize
// pick's one pip or one generic (CR 601.2b, 702.51a; gorge xAsk prices
// paymentManaX, whose fold is applyConvoke). absorb picks the ask's primary
// arm, where a pick the cost at x cannot absorb is rejected (ok=false, gorge
// convokeAbsorbs); the fallback arm applies the picks as applyConvoke does
// when they reduce nothing, which is the same cost wherever the primary arm
// absorbs, so both arms price through this one helper.
func (s *State) xFoldCost(k *prep, base kop.ManaCost, x int, wbX, absorb bool) (kop.ManaCost, bool) {
	c := base
	if k.gen != nil && s.statM.any() {
		c = s.xHypoCost(k, x) // the stat-reduction alternative: the cost at this X
	}
	if wbX {
		return s.waterbendXCost(c, x)
	}
	if absorb {
		return s.convokeXCost(k, c, x)
	}
	return s.convokeApplyPicksTo(k, xFolded(c, x)), true
}

// addXOptions offers X = 0..mx for the entry being built, less any value the target
// ask that follows could not answer, or no hand card could pay the discard at
// (cost_discardxbound.go): X is announced before the targets (CR 601.2b-c),
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
		// a protection quality reads the spell's own mana value, X in it: the
		// announced X can leave every candidate protected (protSpellXMV)
		bound = bound || e.kind == seSpell && k.cards[e.card].cost.X != 0 && protXBoundTarget(k, a)
	}
	picks := s.convokePickCount() > 0
	wbX := s.waterbendXBuilding(k) && s.waterbendPickCount() > 0 // a Waterbend<X> pick pays one generic of X (act_waterbend.go)
	if wbX {
		hi = min(maxXAsk, max(hi, mx+s.waterbendPickCount()))
	}
	// A cast with delve may exile a graveyard card for each generic mana of
	// the total cost (CR 702.66a), the announced X included: gorge's xAsk
	// credits the graveyard in its bound, so the ask's ceiling rises by that
	// credit and xReducedPays prices each value through the same credit.
	if dl := s.xDelveCredit(k, e); dl > 0 {
		hi = min(maxXAsk, max(hi, mx+dl))
	}
	base := s.buildCost(k)
	anyColor := s.bld().bfl&bflAnyColor != 0
	// pays: X = x is payable. An announced convoke pick pays one generic of
	// the X total, so the pool owes the post-pick cost, and a pick the total
	// cannot absorb is rejected (gorge xAsk, convokeAbsorbs with xOpen false).
	// absorb false is gorge xAsk's fallback: when no X is legal (every payable X
	// leaves an announced pick a no-op, or no X answers the targets) the old
	// offer, every payable X with the picks applied as applyConvoke folds them,
	// stands, and the payment settles the cast (CR 733.2).
	absorb := true
	pays := func(x int) bool {
		if !picks && !wbX {
			return x <= mx || s.xReducedPays(k, av, x) || s.xDelveWithinMx(k, e, x, av, anyColor)
		}
		c, ok := s.xFoldCost(k, base, x, wbX, absorb)
		if !ok {
			return false
		}
		if x <= mx {
			return true // picks only lower the cost mx already priced
		}
		if anyColor {
			return av.affordsAnyColor(&c, 0)
		}
		// A filter's net is a paid census gap (mana_filterfirst.go), so the
		// plain census refuses a folded cost the window still pays: gorge's
		// xAsk prices every X through castWindowPays, whose search funds a
		// filter's fee from mana made earlier in the window (CR 601.2b with
		// CR 601.2g, 605.3a-b). faceReachable is that walk, the one the
		// window's strand check prices the credited cost through too
		// (windowStranded), as xDelveWithinMx prices the delve credit through
		// the same filters.
		return s.faceReachable(k, e.ctrl, av, &c, 0)
	}
	n0 := d.N
	fl := s.xFloor(k) // the announced X's floor (cost_sacx.go)
	dxb := s.xDiscardBound(k)
	if bound || dxb {
		for x := fl; x <= hi; x++ {
			if (!bound || s.xAnswerable(k, a, e, x)) && (!dxb || s.xDiscardPays(k, e, x)) && pays(x) {
				d.add(MakeAction(AkChoose, 0, 0, uint8(x)))
			}
		}
	}
	convokeFallback := picks && !wbX
	if d.N == n0 {
		absorb = !(convokeFallback && (bound || dxb))
		for x := fl; x <= hi; x++ {
			if pays(x) {
				d.add(MakeAction(AkChoose, 0, 0, uint8(x)))
			}
		}
	}
	if d.N == n0 && convokeFallback && absorb {
		absorb = false
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
	// the spell build is already pushed (bsPush precedes bsX), so CR 115.5
	// makes it an illegal target for itself: gorge's xAsk sweep excludes the
	// cast card (targetsAnnounceableAt), the target ask's own self pass. Its
	// mana value still reads X = 0 here, and a cmcEQX target (Spell Blast)
	// would match the spell itself at X = its pip total.
	if e.kind == seSpell {
		s.g.tgtSelf = e.obj + 1
	}
	// CR 601.2b: X is announced after the modes, so a modal spell's X is judged
	// against the modes already chosen (gorge chosenModesTargetableAt), not any
	// mode: Confront the Past's Return mode needs a planeswalker card of mana
	// value X or less, whatever the Remove mode could do.
	modes := uint8(0xff)
	if a.NModes > 0 && e.kind == seSpell {
		if chosen := a.ModeSet(e.mode) & (1<<a.NModes - 1); chosen != 0 {
			modes = chosen
		}
	}
	ok := s.abilityTargetsFeasible(k, a, e.ctrl, src, modes)
	if e.kind == seSpell {
		s.g.tgtSelf = 0
	}
	s.g.probe = probe{}
	return ok
}
