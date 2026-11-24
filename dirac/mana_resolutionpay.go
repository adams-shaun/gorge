package dirac

import (
	"fmt"
	"math/bits"
	"os"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// lowerHybridOnly lowers the activation being built (ability ab of the build's
// card) when its mana cost is hybrid pips alone: the activation gorge lowers
// by tapping the planner's witness before activating, however the source
// is activated (a battlefield permanent's ability, a hand or graveyard
// ability such as cycling, a granted one).
func (s *State) lowerHybridOnly(k *prep, ab uint8) {
	if s.cfg&cfgGorgeUnpriced == 0 || ab >= abCascade {
		return
	}
	if c := &k.t.Cards[s.bld().e.card].Abilities[ab].Cost.Mana; c.Hyb[0] != 0 && !c.Phyrexian() && c.X == 0 && c.Generic == 0 && pipMaskOf(c) == 0 {
		s.lowerHybridActivation(k, c)
	}
}

// lowerHybridActivation taps the witness of a hybrid activation the
// oracle lowers (kverify-c4: Puresight Merrow's {W/U}{Q}). gorge offers
// an activation as is through its autopay offer only when the cost has
// generic or coloured mana (rules abilityCostNeedsMana: Silkbind Faerie's
// {1}{W/U} is offered and paid in its mana window, as the kernel pays);
// a hybrid-only cost is offered only when the pool pays, else its
// potential play is lowered: the planner's witness (planHybridCost: the
// first pip resolution the sources afford) is tapped before activating,
// so the mana floats through the pip ask. A pool that already pays it
// taps nothing.
//
//go:noinline
func (s *State) lowerHybridActivation(k *prep, c *kop.ManaCost) {
	p := s.bld().e.ctrl
	if s.poolPays(p, c) {
		return
	}
	// The restricted mana floating from an earlier ability (Eclipsed Realms'
	// {G} for Activated abilities of the chosen type) pays the activation too
	// (CR 106.6; gorge AvailableFor), so the witness taps only what it leaves.
	d := payDesc{class: pdAct, card: s.bld().e.card, src: s.srcSlot()}
	rm := s.restrFloatActMask(k, p, &d)
	if rm != 0 && s.restrFloatPays(p, rm, c) {
		return
	}
	// A lowered hybrid activation's witness is printed-only, like gorge's
	// planHybridCost (dirac/mana.go's noGrant rule, CR 107.4e's mana
	// objects: a granted ability is the grant search, phase 2): the
	// announcement's trial here is the same resolution, so Insidious
	// Roots' any-colour token is not the {G} face the {G/W} prices.
	ng, ax := s.g.noGrant, s.g.availX
	s.g.noGrant, s.g.availX = true, s.g.availX|availPlain
	// The activated source's own {T} cost is paid from it (CR 602.2b,
	// 601.2h), so it is no mana source for the witness: gorge's planner
	// reserves it, and the manual window holds it tapped across the mana
	// step the same way (mana_window.go). Without this the witness taps an
	// Azorius Locket for the {W/U} its own draw ability then needs to tap for.
	sl := s.srcSlot()
	held := sl != NoSlot && k.t.Cards[s.bld().e.card].Abilities[s.bld().e.ab].Cost.Flags&kop.CostTap != 0 && s.perms[sl].flags&pfTapped == 0
	if held {
		s.perms[sl].flags |= pfTapped
	}
	r := s.hybResolveAct(k, p, c, &d, rm, sl)
	s.g.noGrant, s.g.availX = ng, ax
	if rm != 0 {
		r = s.restrShortAdm(p, rm, r)
	}
	// what the witness taps of a restricted source floats under its
	// restriction, as a spell's witness does (lowerWitnessEx): a hybrid pip
	// answered with another colour leaves the witness's mana unspent (the
	// taps stand), and Eclipsed Realms' leftover {R} keeps "Spell.ChosenType,
	// Activated.ChosenType" (CR 106.6; gorge emits the batch at the tap)
	if k.restrSrc {
		pool, untapped := s.pl[p].pool, s.untappedRestrSrcs(k, p)
		var pk planner
		s.tapForPK(k, p, &r, 0, NoObj, false, false, &pk)
		s.witnessManaRestricted(k, p, &pool, untapped, &pk)
	} else {
		s.tapFor(k, p, &r, 0, NoObj)
	}
	if held {
		s.perms[sl].flags &^= pfTapped
	}
}

// hybAbilityUnwitnessed: gorge arms no witness for this activation: the cost
// its arm gate (rules armAbilityAutoPay's abilityCostNeedsMana) reads prices
// no generic or coloured pip, a cost of hybrid pips alone, so no plan is
// armed at the activation's begin whatever the pool and sources fund, and
// the pips' answers never re-plan one. Its mana is the ordinary manual
// CR 601.2g window's, not a plan's.
func (s *State) hybAbilityUnwitnessed(k *prep, e *stackEntry, a *kop.Ability, sl Slot) bool {
	c := s.abPlannedCost(k, a, sl, e.ctrl)
	return c.Hyb[0] != 0 && c.Generic == 0 && pipMaskOf(&c) == 0
}

// hybWindowPay pays a hybrid-only activation through gorge's ordinary manual
// CR 601.2g window (the case arm hybAbilityUnwitnessed approved): the first
// useful source in battlefield order (Creakwood Ghoul's {B/G}{B/G} over a
// {G} pool: Vivid Meadow's plain {T} floats a {W} before Forest's {G} pays).
// The window can run dry when a source's first ability is not the colour owed
// (a granted basic land type: static_landtype.go; a filter's free {C}) or when
// the sources simply run out (four hybrid pips settled to a colour no
// remaining source makes); gorge then aborts the activation and keeps the
// taps, as for the off-plan window. It reports whether the activation
// continues.
func (s *State) hybWindowPay(k *prep, e *stackEntry, cost *kop.ManaCost, saved *[len(build{}.picks)]uint8, hostSl, sl Slot, selfUntapped bool) bool {
	if !s.windowTapHeldOK(k, e.ctrl, cost) && !s.winPaused() {
		s.reverseActivation(k, saved, hostSl, sl, selfUntapped)
		return false
	}
	return true
}

// hybAbilityOffPlan: gorge arms a hybrid activation the pool does not pay
// with its planner's witness (rules armAbilityAutoPay, over the cost
// after the pip asks), and the witness resolves each pip as
// planHybridCost does (the first resolution the pool and sources afford):
// a pip answered with another colour changes the cost, and gorge falls
// back to its manual window (paymentPlanCheck's cost_changed), which the
// oracle answers source by source (kverify-c4: Silkbind Faerie's
// {1}{W/U} paid {1}{U}; a lowered Puresight Merrow whose witness {W}
// floats, paid {U}). resolved is the cost after the pip asks.
//
//go:noinline
func (s *State) hybAbilityOffPlan(k *prep, p uint8, resolved *kop.ManaCost) bool {
	a := &k.t.Cards[s.bld().e.card].Abilities[s.bld().e.ab]
	c0 := s.abPlannedCost(k, a, s.srcSlot(), p) // the planner's priced cost
	if c0.Hyb[0] == 0 || c0.X != 0 || s.poolPays(p, resolved) {
		return false
	}
	return s.hybResolve(k, p, c0, 0) != *resolved
}

// payReplanned pays a resolution-time mana cost the way gorge funds an
// elected "pay" (rules respay.go resPayFund): while the pool does not pay
// it, plan the rest and activate only that plan's first source, then plan
// again -- the plan of the whole cost can leave a different source
// untapped (kverify-c4: Digsite Conservator's {4} over a Swamp and five
// Forests). The manual window's decision carries no owed cost, so an
// any-colour source answers its first colour, W (windowTapNoOwed). The
// planner's witness has printed abilities only: a granted one (Great Divide
// Guide's any-colour mana) is a V1 exclusion (pay.PaymentAbility), so a
// payment only granted sources could fund has no witness and takes the
// manual window.
// When the planner has no witness at all -- a pay
// gorge offered on its potential bound the sources cannot actually fund
// (respay.go:84, potential.go:324) -- gorge funds through that same manual
// window, tapping every useful source, then the charge fails and the flow
// declines. The taps and the floated mana stay; nothing panics. The dry
// probe is that no-witness test (a dry plan reports found without tapping:
// mana.go plan), and the final poolPays check is the same fallback for a
// plan that stops short.
//
//go:noinline
func (s *State) payReplanned(k *prep, p uint8, cost *kop.ManaCost) bool {
	c := *cost
	if c.Hyb[0] != 0 {
		c = s.hybResolve(k, p, c, 0)
	}
	// the floating restricted mana its restriction admits on a non-spell
	// payment funds it first (CR 106.6: gorge's planner pool is AvailableFor's
	// PurposeOther view), so the sources tap only for what it leaves
	need := s.restrShortNonSpell(k, p, c)
	// The plan is gorge's plain census (resPayFund's pay.PlanPaymentCost):
	// no tiered fallback pass. A source a Taps watcher defers (Manabarbs)
	// or a Combo Any source is never one of its units, so a cost only those
	// fund has no witness and takes the manual window (resPayFund's
	// rp.manual), whose taps are its answer's (trigWindowPays); the tiered
	// passes are the potential play's prefix gaps', never a payment's.
	var dry planner
	dry.dry, dry.noGrant, dry.plain = true, true, true
	s.tapPlanNeed(k, p, &need, 0, NoObj, true, false, &dry)
	if dry.needT > 0 && !dry.found {
		return s.trigWindowPays(k, p, &c) // gorge's manual window: tap every useful source, no owed answer
	}
	for i := 0; i < MaxPerm && !s.poolPays(p, &need); i++ {
		pk := planner{noGrant: true, plain: true}
		sl := s.tapForPK(k, p, &need, 0, NoObj, true, false, &pk)
		if sl == NoSlot {
			break
		}
		if s.parkedWaiting() || s.reflectPickPending() {
			return false // a sacrificed source's order ask or a tap's reflected pick: the payment runs again after it
		}
		need = s.restrShortNonSpell(k, p, c) // a tapped source may have floated restricted mana
	}
	if !s.poolPays(p, &need) {
		return s.trigWindowPays(k, p, &c)
	}
	s.payTrigPool(k, p, &c)
	return true
}

// restrShortNonSpell is cost less p's floating restricted mana that a
// non-spell payment may spend (restrPoolNonSpell, CR 106.6).
func (s *State) restrShortNonSpell(k *prep, p uint8, cost kop.ManaCost) kop.ManaCost {
	if m := s.restrPoolNonSpell(k, p); m != 0 {
		return s.restrShortAdm(p, m, cost)
	}
	return cost
}

// payTrigPool charges a trigger's own mana cost the pool pays (restrShortNonSpell
// is covered): the floating restricted mana its restriction admits goes first.
func (s *State) payTrigPool(k *prep, p uint8, c *kop.ManaCost) {
	if m := s.restrPoolNonSpell(k, p); m != 0 {
		s.payRestrSpend(k, p, m, c, 0, NoObj, false)
		return
	}
	s.payFromPool(p, c, 0)
}

// trigWindowPays runs the manual window for a payment the planner has no
// witness for and charges the cost when what the window floated pays it, as
// gorge's resolution pay does after the window (Mardu Devotee turning one
// Swamp's black into the white a {1}{W} cost needs); a window that stops short
// leaves its taps and the mana floating, and a paused one finishes on its
// answer.
//
//go:noinline
func (s *State) trigWindowPays(k *prep, p uint8, c *kop.ManaCost) bool {
	if s.trigWindowTap(k, p, c) {
		return false
	}
	return s.trigWindowCharge(k, p, c)
}

// trigWindowCharge charges the trigger's cost when what its window floated pays
// it: the settle of trigWindowPays, which a tap-another pick's answer runs once
// the window has walked on (applyAskWinTap).
func (s *State) trigWindowCharge(k *prep, p uint8, c *kop.ManaCost) bool {
	if need := s.restrShortNonSpell(k, p, *c); !s.poolPays(p, &need) {
		return false
	}
	s.payTrigPool(k, p, c)
	return true
}

// runPrefixes runs the prefix searches applyPriority does before the play --
// grantFirst's grant search, the script search, the plain grant search beside
// it and the chosen-colour-pair search -- flushing each one's triggers below
// the play. It reports whether a prefix ran and tapped: its taps are gorge's
// search's, made at its asks (before the play), and a cost part it left to the
// planner is paid the same way before the push, not by manaStep's post-push tap
// (payPrefixRest). A prefix that ran without tapping priced the play by
// planning; gorge pays that at payment, after the push, as manaStep does.
func (s *State) runPrefixes(k *prep, a Action, p uint8, grants, grantFirst bool) (ran, tapX bool) {
	pool0 := s.prefixPoolMana(p)
	grantFirstRun := false
	if grantFirst {
		if dbgAct {
			fmt.Fprintf(os.Stderr, "PREF grantPrefix\n")
		}
		s.grantPrefix(k, a, p)
		ran, grantFirstRun = true, true
		if s.npend != 0 {
			s.flushPrefixTriggers(k)
		}
	}
	if s.scriptAny(k) && isScriptPlayKind(a.Kind()) {
		// The tapX search's prefix taps the plan's needed colours before the
		// play (mana_tapanother.go), as gorge's one search does: a gap-funded
		// activation's printed share is floated here, before the target ask,
		// not by the post-push planner.
		tapXBefore := !grantFirstRun && (k.tapMana || k.ctrMana) && s.scriptTapXModelled(k, p).any()
		if dbgAct {
			fmt.Fprintf(os.Stderr, "PREF scriptPrefix tapXBefore=%v\n", tapXBefore)
		}
		s.scriptPrefix(k, a, p) // a play only a scripted activation pays (script_gen.go)
		ran = true
		if tapXBefore && !s.scriptTapXModelled(k, p).any() {
			tapX = true
		}
		if s.npend != 0 {
			s.flushPrefixTriggers(k) // its trigger resolves before the play (seed 640)
		}
	}
	if grants && !grantFirst {
		if dbgAct {
			fmt.Fprintf(os.Stderr, "PREF grantPrefix2\n")
		}
		s.grantPrefix(k, a, p) // a play only a granted mana ability pays (script_gen.go)
		ran = true
		if s.npend != 0 {
			s.flushPrefixTriggers(k)
		}
	}
	if a.Kind() == AkActivate && k.gen != nil && k.gen.rs != nil && k.gen.rs.chosenPair {
		if dbgAct {
			fmt.Fprintf(os.Stderr, "PREF chosenPair\n")
		}
		s.chosenPairPrefix(k, a, p) // a play only a chosen-colour pair pays (mana_chosenprefix.go)
		ran = true
		if s.npend != 0 {
			s.flushPrefixTriggers(k)
		}
	}
	return ran && s.prefixPoolMana(p) > pool0, tapX
}

// prefixPoolMana is p's floating pool total: the prefix section's taps (and
// sacrifices) float their mana here, so a rise over the snapshot taken before
// it marks a prefix search that activated for the play. A prefix that also
// paid a sub-cost from the pool can net to no rise; that play keeps the
// post-push payment, as before.
func (s *State) prefixPoolMana(p uint8) int {
	n := 0
	pl := &s.pl[SeatIx(p)]
	for c := 0; c < kop.NumManaKinds; c++ {
		n += int(pl.pool[c])
	}
	return n
}

// activatePrefixRest is payPrefixRest for an activation applyPriority is about
// to build. A prefix that tapped runs it (pref); so does a board with a
// Taps-deferred source even when no prefix tapped: gorge's planner refuses the
// deferred sources on its first pass, so a play its plain census cannot price
// is proved by the potential play's search, which activates the plan before the
// play (Blisterspit Gremlin's {1} under Manabarbs: the land's trigger sits below
// the ability, not above it). payPrefixRest returns when the plain census
// prices the remainder, so a board with no such gap is unchanged.
func (s *State) activatePrefixRest(k *prep, a Action, p uint8, pref, tapX bool) {
	if pref || k.gen != nil && k.gen.anyTapsDefer {
		s.payPrefixRest(k, a, p, tapX)
	}
}

// payPrefixRest pays, before the activation's build begins, what a prefix
// search left of a plain activation cost to the payment planner. The
// planner's tap fires at manaStep (bsMana), after bsPush has placed the
// ability, so the tapped source's Taps trigger lands above the paid-for
// ability; gorge's search taps at its ask, below it (Charix, the Raging
// Isle's {3} pump under an opponent's Manabarbs). The cost is the offer's
// (abPlannedCost): the build is not yet live, so the pre-build fields the
// build's arms read (pips, X) are not announced yet anyway. Only a cost the
// plain planner settles takes this -- one it cannot price keeps its manaStep
// window arm (sacXWindowPays, equipGapWindowPays, the hybrid arms), and
// restricted floating mana keeps the shortened cost the planner pays.
func (s *State) payPrefixRest(k *prep, a Action, p uint8, force bool) {
	o, ab := Obj(a.Source()), a.Mode()
	ob := &s.objs[o]
	cd := &k.t.Cards[s.cardAt(ob.slot)]
	if isGainedMode(ab) || int(ab) >= len(cd.Abilities) {
		return // a gained ability reads its host card (activateGen), not this build
	}
	abn := &cd.Abilities[ab]
	if abn.Kind == kop.AbMana || abn.Cost.Flags&(kop.CostSac|kop.CostSacSelf|kop.CostTapX|kop.CostTapHost|kop.CostPowerUp|kop.CostWaterbend) != 0 ||
		abn.Cost.Mana.X != 0 || abn.Cost.Mana.XPip != 0 {
		return // the build's X, sacrifice, pip and window arms keep their paths
	}
	sl := NoSlot
	if ob.zone == kop.ZoneBattlefield {
		sl = ob.slot
	}
	cost := s.abPlannedCost(k, abn, sl, p)
	if cost.X != 0 || cost.Hyb[0] != 0 || cost.Tb != 0 || cost.Snow != 0 || cost.Phyrexian() {
		return
	}
	if s.poolPays(p, &cost) {
		return // the prefix settled it
	}
	if s.restrActNow(k, p) != 0 {
		return // admitted restricted mana shortens the cost the planner pays (mana_window.go)
	}
	// The remainder must be one the plain census cannot price but the census
	// with the Taps-deferred sources can (the probe is dry: it taps nothing).
	// That is the shape gorge's prefix search activates for: its planner's
	// first pass refuses the Taps-deferred sources, so the potential play's
	// search proves the play by activating the whole plan (mana_tapsdefer.go).
	// A remainder the plain census prices is gorge's planner's to pay, at
	// payment, after the push -- and a plan no pass prices keeps manaStep's
	// window arm.
	var chk planner
	chk.dry = true
	s.tapNeed(&chk, p, &cost, 0, false)
	if chk.needT == 0 {
		return
	}
	force = force && abn.NTargets > 0 && abn.Cost.Flags&kop.CostTap != 0
	if force {
		// The tapX search's prefix taps the plan's needed-colour sources
		// before the play (mana_tapanother.go), as gorge's one search does:
		// float them here so the target ask sees the same board. The
		// remaining generic is the post-push planner's, as it is in gorge.
		// Only a play whose own cost taps its source floats: the held {T}
		// (CR 602.2b) is the play's own unit, so gorge's pre-play floats
		// cover the whole mana part beside it (Nephalia Smuggler's {3}{U},
		// {T}, activation_tapx_payment_floats_before_target). A plain-cost
		// activation's gap float is its prefix's last word: gorge pays the
		// rest at payment, after the push, and its target ask sees the
		// board only as far as the gap's float went (a Springleaf Drum
		// beside an untapped Swamp floating one of a {2B} ability's two
		// units: no more taps before the target ask).
		held := sl != NoSlot && abn.Cost.Flags&kop.CostTap != 0 && s.perms[sl].flags&pfTapped == 0
		if held {
			s.perms[sl].flags |= pfTapped
		}
		// gorge's search stops at the node where the planner prices the play
		// from the pool and the census: a prefix float that already covers the
		// coloured pips beside a census that pays the rest floats no more
		// (Arc Mage's {2}{R} after Relic of Legends' R).
		priced := chk
		priced.try = true
		s.plan(k, p, &priced, NoObj)
		if priced.found && !plansAnyColour(&priced) {
			if held {
				s.perms[sl].flags &^= pfTapped
			}
			return
		}
		// Leave one unit (the generic remainder) to the post-push planner, as
		// gorge's prefix search does.
		s.payNeededColours(k, p, &cost, max(cost.MV()-1, 0))
		if held {
			s.perms[sl].flags &^= pfTapped
		}
		if s.npend != 0 {
			s.flushPrefixTriggers(k) // the tapped source's trigger below the play, not above it
		}
		return
	}
	if k.gen == nil || (!k.gen.anyTapsDefer || DebugNoTapsDefer) {
		return // no Taps-deferred source can be the remainder: the post-push payment, as before
	}
	saved := chk
	chk.try = true // the first pass: the census without the Taps-deferred sources
	s.plan(k, p, &chk, NoObj)
	if chk.found {
		return // the plain census prices it: gorge's planner pays it post-push
	}
	chk = saved
	chk.tapsIn = true // the second pass: the Taps-deferred sources are units
	s.plan(k, p, &chk, NoObj)
	if !chk.found {
		return
	}
	// the source's own {T} cost holds it tapped across the payment the way
	// manaStep holds it (CR 602.2b), so the planner does not tap it for mana
	held := sl != NoSlot && abn.Cost.Flags&kop.CostTap != 0 && s.perms[sl].flags&pfTapped == 0
	if held {
		s.perms[sl].flags |= pfTapped
	}
	s.tapFor(k, p, &cost, 0, NoObj)
	if held {
		s.perms[sl].flags &^= pfTapped
	}
	if s.npend != 0 {
		s.flushPrefixTriggers(k) // the tapped source's trigger below the play, not above it
	}
}

// payNeededColours taps p's untapped sources of cost's coloured pips until
// target mana is floating or no such source is left, in the planner's source
// order. It is the pre-push half of a tapX-funded activation's payment: gorge's
// prefix search taps the play's needed-colour sources before the play, and the
// generic remainder after it, so the target ask sees the same board
// (mana_tapanother.go's tapXPrefix).
//
// Each iteration demands one NEW unit of a pip colour, so a colour the board
// can fund no more of is a clean miss, not the payment: the demand is probed
// with a dry planner first (it selects the plan without tapping and reports no
// plan as found == false, the two-pass shape payPrefixRest prices its
// remainder with), and a probe with no plan stops the pre-float -- the
// already-floated units stay in the pool and the post-push payment at manaStep
// pays the rest, the pre-fix shape for a play whose prefix never fired
// (TestTapxPrefixFloatSkipsUnfundablePip: Smuggler's {3}{U},{T} with Sentinel's
// unit as the board's only {U}).
func (s *State) payNeededColours(k *prep, p uint8, cost *kop.ManaCost, target int) {
	for {
		if s.prefixPoolMana(p) >= target {
			return
		}
		tappedOne := false
		for c := 0; c < kop.NumManaKinds; c++ {
			if cost.Pips[c] == 0 {
				continue
			}
			var one kop.ManaCost
			one.Pips[c] = s.pl[SeatIx(p)].pool[c] + 1
			var probe planner
			probe.dry = true
			s.tapForPK(k, p, &one, 0, NoObj, false, false, &probe)
			if !probe.found {
				return // no plan funds one more of c: the post-push planner pays the rest
			}
			before := s.prefixPoolMana(p)
			s.tapFor(k, p, &one, 0, NoObj)
			if s.prefixPoolMana(p) > before {
				tappedOne = true
				break
			}
		}
		if !tappedOne {
			return
		}
	}
}

// castRestPay prices the Taps-deferred gaps' payment search's stop: the
// planned cast o whose cost part the gap taps' floats must cover
// (payCastPrefixRest). nil in the offer search (scriptPrefixAny,
// scriptDeadEndAny).
type castRestPay struct {
	o    Obj
	cost kop.ManaCost
}

// castRestPriced reports whether the planner prices the planned cast's
// payment from the pool and census, tapping nothing (gorge's sr.priced node
// check, potential_script.go: the search's nodes stop where the planner pays
// the play from the floating pool). On a gap search's node copy the deferred
// sources are already hidden (hideDeferred), so the census is the printed
// sources' and the pool is the taps' floats. dry: the probe taps nothing;
// try: the census without the Taps-deferred sources (gorge's census never
// lists them); castPay: a cast's payment, so the dynamic {T}, sacrifice
// sources stay out (planCastDynSacSkip); o: the card being cast is out of the
// hand reserve.
func (s *State) castRestPriced(k *prep, p uint8, o Obj, cost *kop.ManaCost) bool {
	var pk planner
	pk.dry, pk.try, pk.castPay = true, true, true
	s.tapNeed(&pk, p, cost, 0, false)
	if pk.needT == 0 {
		return true
	}
	s.plan(k, p, &pk, o)
	return pk.found
}

// castRestPricedModes are the AkCast mode bits plannedCastCostOf prices as the
// offer does: bit 1 (flashback: flashbackCostOf), castFace (turnFaceCast
// rewrote the card to the face, whose own cost it reads) and castMayPlay (the
// any-type/any-colour riders it applies). Every other bit is an alternative or
// added cost it does not read: bit 0 kicker, castWarp/castWarpRecast (warp,
// airbend), castFree, castFaceDown (altPlayCost).
const castRestPricedModes = 2 | castFace | castMayPlay

// castRestRefuses reports whether cast a keeps manaStep's arm because
// plannedCastCost prices a cost the offer prices differently: the offer's
// price of these casts is grantPlayCost's (mana_grantedoffers.go), not
// plannedCastCost's printed one, so the gate's two probes and the payment
// search's stop would run against a cost the cast never charges.
//
//   - a special-target cast pays its alternative cost: impending, evoke,
//     dash, foretell, overload, cleave, surge (the target byte's TCode, the
//     card's feature beside it, as grantPlayCost and legal.go read it), the
//     card's own mana alternative (castAltTgt, k.gen.altFree), a printed or
//     granted escape cast and a beam me up cast (mode bit 2's target byte;
//     kw_escape.go, kw_beammeup.go). The face-cast family is not affected:
//     turnFaceCast rewrote the card to the face before this, so
//     plannedCastCost reads the face's own cost.
//   - a kicked cast pays the Kicker part on top of the printed cost:
//     grantPlayCostRaise composes it at its call site (plannedCastCostOf's
//     mode read strips bit 1).
//   - a warp cast pays the warp alternative, which the build's cost arm
//     composes from s.g.warpGrant; plannedCastCost never reads it.
//   - a face-down cast pays {3} and a free cast nothing (altPlayCost's cases);
//     plannedCastCostOf never reads castFaceDown or castFree, so it would
//     price the printed cost.
//
// The mode check is an allowlist (castRestPricedModes), so an alternative-cost
// mode added later stays refused until it is audited against altPlayCost.
func (s *State) castRestRefuses(k *prep, o Obj, a Action) bool {
	m := a.Mode()
	if m&^castRestPricedModes != 0 {
		return true // a mode bit plannedCastCost's price never reads (see castRestPricedModes)
	}
	c := s.objs[o].card
	cd := &k.t.Cards[c]
	if m&2 != 0 {
		// a mode-2 cast whose target byte names the escape or beam casts
		return cd.Escape && a.Target() == TCode(castPrintedEscapeTgt) ||
			k.gen != nil && k.gen.anyEscapeGrant && a.Target() == TCode(castEscapeTgt) ||
			cd.BeamMeUp && (a.Target() == castBeamTgt || a.Target() == castBeamGrantTgt)
	}
	if m != 0 || k.gen == nil {
		return false
	}
	switch {
	case a.Target() == castAltTgt:
		return k.gen.altFree != nil && k.gen.altFree[c] != 0
	case a.Target() == castImpTgt:
		return k.gen.anyImpending && cd.Impending > 0
	case a.Target() == castEvokeTgt:
		return k.gen.anyEvoke && cd.HasEvoke
	case a.Target() == castDashTgt:
		return k.gen.anyDash && cd.HasDash
	case a.Target() == castForetellTgt:
		return k.gen.anyForetell && cd.ForetellCost != (kop.ManaCost{})
	case a.Target() == castOverloadTgt:
		return k.gen.anyOverload && cd.HasOverload
	case a.Target() == castCleaveTgt:
		return k.gen.anyCleave && cd.CleaveAb != 0
	case a.Target() == castSurgeTgt:
		return k.gen.anySurge && cd.HasSurge
	}
	return false
}

// payCastPrefixRest pays, before the cast's build begins, what a prefix
// search left of the cast's planned cost to the potential-play search. The
// search's remaining gap taps fire at manaStep (bsMana), after bsPush has
// placed the spell, so their Taps trigger lands above it; gorge's
// potential-play search proves the cast by activating the gaps at its asks,
// below the spell (a land's tap under an opponent's Manabarbs, a Spring
// Cleaning cast). This is payPrefixRest's gate for the cast arm: the cost is
// the cast planner's (plannedCastCost), whose planned=false return keeps
// manaStep's window arm for every shape it cannot price (modal, additional,
// optional and target-dependent costs, a mana-spent reader, a contribution;
// kicker and flashback), and the build's X and pip arms keep their paths.
// plannedCastCost strips the Phyrexian pips (gorge pay.PlanManaHalf), so the
// Phyrexian guard reads the raw composed cost. restricted floating mana
// keeps the shortened cost the planner pays. A cast has no {T}-costing host,
// so payPrefixRest's held-tap block does not apply.
func (s *State) payCastPrefixRest(k *prep, a Action, p uint8) {
	o := Obj(a.Source())
	cost, ok := s.plannedCastCost(k, p, o, a.Mode())
	if !ok {
		return // the cast planner declines the shape: manaStep's window arms keep their paths
	}
	if s.castRestRefuses(k, o, a) {
		return // a cost the offer prices differently: manaStep's window arms keep their paths
	}
	if cost.X != 0 || cost.XPip != 0 || cost.Hyb[0] != 0 || cost.Tb != 0 || cost.Snow != 0 {
		return // the build's X, twobrid and snow arms keep their paths
	}
	raw := k.cards[s.objs[o].card].cost
	if a.Mode()&2 != 0 {
		raw = s.flashbackCostOf(k, o) // the flashback face's own cost
	}
	if raw.Phyrexian() {
		return
	}
	if s.poolPays(p, &cost) {
		return // the prefix settled it
	}
	if s.restrActNow(k, p) != 0 {
		return // admitted restricted mana shortens the cost the planner pays (mana_window.go)
	}
	// gorge's search proves the play at its asks by activating the census
	// gaps until the planner prices the payment from the floats
	// (potential_script.go sr.priced). The kernel's prefix proved only the
	// offer -- a planned cast stays offered without paying -- so the
	// Taps-deferred unit its skip left behind is activated here, before the
	// push. The probes are dry: they tap nothing. The first is the planner's
	// census (the Taps-deferred sources out, the card being cast out of the
	// hand reserve, a cast's payment: the dynamic {T}, sacrifice sources stay
	// out, planCastDynSacSkip); the second counts the Taps-deferred sources
	// as units, so a play no gap subset prices keeps manaStep's window arm.
	tapsDef := s.scriptTapsDefer(k, p)
	if k.gen == nil || !k.gen.anyTapsDefer || DebugNoTapsDefer || !tapsDef.any() {
		return // no Taps-deferred source can be the remainder: the post-push payment, as before
	}
	if s.castRestPriced(k, p, o, &cost) {
		return // the planner prices the payment: it pays at payment, post-push, as before
	}
	var chk planner
	chk.dry, chk.try, chk.tapsIn, chk.castPay = true, true, true, true
	s.tapNeed(&chk, p, &cost, 0, false)
	s.plan(k, p, &chk, o)
	if !chk.found {
		return // a play no gap subset prices keeps manaStep's window arm
	}
	// the payment search's script, played: deferGapSearch explores the gaps'
	// subsets, every answer each gap's activation poses (a two-colour Combo
	// gap's colour ask), the line that ends the payer's line cut and the
	// search's next sibling tried, and stops where the floats price the
	// payment (castRestPriced); the replay is scriptPrefixAny's tapsDef
	// branch (mana_window.go), each tap's trigger flushed below the spell
	var order [MaxPerm]Slot
	n := s.scriptGapOrder(k, a, p, tapsDef, order[:])
	var ans [MaxPerm]uint8
	var pick [MaxPerm]int8
	var dead bool
	cp := castRestPay{o: o, cost: cost}
	if !s.deferGapSearch(k, a, p, order[:n], tapsDef, &cp, &ans, &pick, &dead) {
		return // the search prices no line of gap taps: manaStep's arm keeps the post-push payment
	}
	for j := 0; j < n && !s.over; j++ {
		if pick[j] < 0 {
			continue // the found script went on without this gap
		}
		var spill [kop.NumManaKinds]uint8
		s.scriptGapTap(k, p, a, order[j], slotMask{}, slotMask{}, slotMask{}, tapsDef, slotMask{}, int(ans[j]), &spill)
		if s.npend != 0 {
			// gorge answers each gap's activation as its own decision, so
			// the trigger queue is drained before the next tap (CR 603.3b)
			s.flushPrefixTriggers(k)
			if s.npend != 0 {
				s.resolvePrefixStack(k, s.sp, false)
			}
		}
	}
}

// DebugRestrictedFloatIgnored (TestRestrictedFloatLoweredActivationMutation):
// a lowered activation reads no floating restricted mana, as before the fix.
var DebugRestrictedFloatIgnored bool

// restrFloatActMask is the mask of p's floating restricted batches (State.rb)
// the activated-ability payment d describes may spend (restrFloatAdmits, so
// Activated.* restrictions match as Spell.* do); 0 for any other payment.
func (s *State) restrFloatActMask(k *prep, p uint8, d *payDesc) uint8 {
	if DebugRestrictedFloatIgnored || d.class != pdAct || !s.rbAny(p) {
		return 0
	}
	return s.restrFloatAdmits(k, p, d)
}

// restrFloatPays reports whether p's pool with its restricted batches of mask
// m pays cost.
func (s *State) restrFloatPays(p, m uint8, cost *kop.ManaCost) bool {
	var av avail
	for c, n := range s.pl[SeatIx(p)].pool {
		if n > 0 {
			av.addUnits(1<<c, n)
		}
	}
	s.restrAvailAdd(SeatIx(p), m, &av)
	return av.affords(cost, pipMaskOf(cost), 0)
}

// hybResolveAct is hybResolve for the activation d describes: the witness's
// first affordable pip resolution counts the floating restricted mana of the
// batches in rm and the restricted sources the activation may spend (gorge's
// planHybridCost reads the planner's pool for it: Eclipsed Realms' any-colour
// {G} pays the {G/W} before a Plains' {W}).
func (s *State) hybResolveAct(k *prep, p uint8, c *kop.ManaCost, d *payDesc, rm uint8, skip Slot) kop.ManaCost {
	if DebugRestrictedFloatIgnored {
		return s.hybResolve(k, p, *c, 0)
	}
	var av avail
	s.availFor(k, p, &av)
	if rm != 0 {
		s.restrAvailAdd(p, rm, &av)
	}
	var multi [2]dynUnit
	nm := 0
	if k.restrSrc {
		av, multi, nm = s.restrSrcAvail(k, p, *d, &av, skip)
	}
	return av.hybWitnessRiderMulti(*c, 0, false, multi[:nm])
}

// plansAnyColour reports whether pl's plan taps a source that makes any colour
// (a Chromatic Lantern): gorge's search keeps floating past a node whose
// remainder only such a source pays.
func plansAnyColour(pl *planner) bool {
	for i := 0; i < pl.nu; i++ {
		if bits.OnesCount8(pl.types[pl.utyp[i]].mask&0x1f) > 1 {
			return true
		}
	}
	return false
}

// runPrefixesGated is runPrefixes behind its four gates, tested inline so a table
// with none of them skips the call.
func (s *State) runPrefixesGated(k *prep, a Action, p uint8, grants, grantFirst bool) (ran, tapX bool) {
	if grants || s.scriptAny(k) && isScriptPlayKind(a.Kind()) || a.Kind() == AkActivate && k.gen != nil && k.gen.rs != nil && k.gen.rs.chosenPair {
		return s.runPrefixes(k, a, p, grants, grantFirst)
	}
	return false, false
}
