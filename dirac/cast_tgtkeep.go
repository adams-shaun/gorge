package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// witnessKeepsTargets is the plain cast's keep-targets guard: gorge offers the
// cast only while some way to pay it leaves the spell a legal target on the
// battlefield. gorge's payment planner prices a plan its census pays outright
// and runs no such check on the payment action (payment_plan.go
// paymentActionsForPriority), so the guard holds only when the census cannot
// pay the cost without the sources whose acquisition consumes a permanent —
// the planner's deferred fee abilities (the Cave of Temptation case below) —
// and the potential planner prices the play only through its witness or a
// scripted prefix, each replayed before it is offered (potential_plan.go
// potentialWitnessReaches, potential_script.go priced).
//
// The census mirror: the census that prices the offer (its script gaps
// included, offerSourcesAfford's gaps read) must pay the composed cost with a
// proper subset of the targets consumed — some plan that leaves a target
// standing to be targeted (CR 601.2c, gorge potential_script.go targetsGone
// at the priced node). Consuming a target buys its sacrifice-bought units —
// the tap-and-sacrifice unit (a Treasure's "{T}, Sacrifice: Add {C}", CR
// 605.1a) and the sacrifice-only one (Tinder Wall's "Sacrifice this: Add
// {R}{R}") — so the test per target t prices the census with t's whole
// production held out and only its non-consuming units put back (a tap-made
// one, or a life-only one: paying life does not leave the battlefield, CR
// 601.2c): the weakest test,
// the plan that consumes every target but t and taps the rest. A target the
// census cannot pay without is stranded by every plan: gorge's
// "unsupported: census_incomplete" verdict for the Stealth Mission cast at
// tail-76 pair 9, whose {2}{U} is paid only by Tinder Wall's sacrifice beside
// Cave of Temptation's "{1}, {T}: Add one mana of any color" (CR 605.3a: the
// ability's own fee is paid from floating mana during spell payment), the
// Wall being the spell's only Creature.YouCtrl target.
//
// The whole read runs on a fork (face_castface.go faceWitnessReaches): the
// census walks it repeats must not leave bookkeeping the game's later offers
// read, and the tier-1 census runs in the planner's plain mode (script_gen.go
// plainOptions) — gorge's payment planner prices a plan its own census pays.
func (s *State) witnessKeepsTargets(k *prep, p uint8, c CardID, o Obj, cost *kop.ManaCost) bool {
	sp := tspec(k, c, spellAb(k, c), 0)
	if sp == nil {
		return true // no targets: no plan can strand one
	}
	if cost == nil {
		return true
	}
	f := s.fork()
	defer s.unfork()
	var t1 avail
	m := f.g.availX
	f.g.availX = availPlain
	f.availFor(k, p, &t1)
	if t1.affords(cost, pipMaskOf(cost), 0) {
		f.g.availX = m
		return true // the tier-1 census pays: gorge's payment planner prices the cast outright
	}
	f.g.availX = m | availGaps // the offer's own affordability census (legal.go offerSourcesAfford)
	var c0 avail
	f.availFor(k, p, &c0)
	if !c0.affords(cost, pipMaskOf(cost), 0) {
		// the census does not pay the cast at all: its offer, if any comes,
		// is the script routes', and gorge's search strands or funds its own
		// lines. The guard stays out of them, as the plain walk's offer did.
		f.g.availX = m
		return true
	}
	ok := f.specKeepsTarget(k, p, o, sp, cost)
	// A charm's other modes' targets are as good: gorge's castTargetsAvailable
	// takes the cast while any mode keeps a legal target (CR 700.2, 601.2c), so
	// Vandalize's land mode stands when the plan consumes every artifact it
	// could destroy.
	if ab := spellAb(k, c); !ok && ab != 255 {
		a := &k.t.Cards[c].Abilities[ab]
		for j := 1; a.NModes > 1 && j < int(a.NTargets) && !ok; j++ {
			if a.TgtModes[j]&a.TgtModes[0] == 0 {
				ok = f.specKeepsTarget(k, p, o, &a.Targets[j], cost)
			}
		}
		// A mode no target slot covers announces no targets at all (CR 700.2,
		// 601.2c): the payment strands nothing of its requirement, so it keeps
		// the cast announceable the way a standing candidate does -- Destructive
		// Tampering's can't-block mode beside a plan that consumes the Treasure
		// its destroy-artifact mode would target (dr-mbc-aabb pair 116).
		if !ok && a.NModes > 1 {
			var covered uint8
			for i := 0; i < int(a.NTargets); i++ {
				if tm := a.TgtModes[i]; tm == 0 {
					covered = 0xff // a slot without a mode bound announces targets in every mode
				} else {
					covered |= tm
				}
			}
			for mm := 0; mm < int(a.NModes) && !ok; mm++ {
				if covered&(1<<mm) == 0 && !(k.gen != nil && k.gen.anyCharmPick && s.charmPickBarred(a, o, mm)) {
					ok = true
				}
			}
		}
	}
	f.g.availX = m
	return ok
}

// specKeepsTarget is witnessKeepsTargets' per-spec read over the offer's
// census (availX holds availGaps): true when sp has no legal target, a target
// no mana activation takes, or one candidate whose whole production the plan
// can do without.
func (s *State) specKeepsTarget(k *prep, p uint8, o Obj, sp *kop.TargetSpec, cost *kop.ManaCost) bool {
	var buf [MaxOptions]TCode
	n := s.targetCands(k, sp, p, NoSlot, o, &buf)
	if n == 0 {
		return true // a cast with no legal target (an X-resolves-to-zero announcement) is cast without one
	}
	if sp.TFlags&kop.TgfBoundsV == 0 && sp.Min > 1 {
		return s.minTargetsKept(k, p, o, sp, cost)
	}
	for i := 0; i < n; i++ {
		tc := buf[i]
		if isPlayerOrNone(tc) {
			return true // a player target is never on the battlefield to lose
		}
		tobj := Obj(tc)
		if tobj >= MaxObj || s.objs[tobj].zone != kop.ZoneBattlefield {
			return true // a stack, graveyard or exile target: no mana activation takes it
		}
		var c1 avail
		s.availSansObj(k, p, tobj, &c1)
		if c1.affords(cost, pipMaskOf(cost), 0) {
			return true
		}
	}
	return false
}

// availSansObj is the walk's census with battlefield object o's whole
// production held out and its non-consuming units put back: the object is out
// only as the sacrifice that acquires its mana (CR 601.2c — the target is
// chosen before the costs are paid; only the sacrifice takes it), and a plan
// may tap it for mana, or pay its life, without consuming it. The census
// counts the slot's production once (availForMode's loop), so holding the
// slot out takes its sacrifice-bought unit with it; the gap units the script
// walk adds beside it (availScript's helpers read the board's masks, not the
// loop's exclusions) stay, as do the units of the targets the plan keeps.
//
// A life-only source ("Pay 3 life: Add {R}", mana_pain.go's prepSoleLifeMana)
// is such a kept unit: it is not tapped and never leaves the battlefield, so
// the guard must price it even when it is the spell's own target. Restoring it
// only when painTight left the units alone keeps the budget model's count
// exact (a binding summed life guard is still the conservative answer).
func (s *State) availSansObj(k *prep, p uint8, o Obj, a *avail) {
	sl := s.objs[o].slot
	if fm := s.fodderOnlySources(k, p, sl); fm.any() {
		// a sacrifice-another source whose one fodder is the target has no
		// unit once the plan takes the target for its own cost (CR 601.2c)
		held := s.ctrlM[p]
		s.ctrlM[p] = held.andNot(fm)
		defer func() { s.ctrlM[p] = held }()
	}
	s.availForMode(k, p, a, slotMask{}.with(sl), false)
	m := s.g.availX
	s.g.availX = availPlain
	pc := s.manaPCGen(k, sl)
	s.g.availX = m
	if lifeOnlyPain(pc) && !a.pain.on && !pc.manaSac && !pc.manaSacOnly {
		s.addPainAvail(k, sl, pc, p, a, nil, true)
		return
	}
	if pc.manaTap && !pc.manaSac && !pc.manaSacOnly && s.canTapForManaPC(k, sl, pc) {
		a.addUnits(s.srcMaskAt(pc, sl), s.srcYieldAt(k, sl, pc))
	}
}

// minTargetsKept is specKeepsTarget for a slot that announces at least
// sp.Min targets ("six target nonland permanents", Aether Gale), read the way
// gorge reads it: not by asking whether some plan keeps Min candidates but by
// the plan its search prices. Gorge's prefix search (potential_script.go
// priced) plays the script's taps, asks the planner for the payment of what
// is left, and refuses the play when that plan consumes permanents
// (witnessReachesMode) unless the cast still offers at the end or announces
// before paying and the candidates the plan did not consume still make a legal
// declaration (witnessAnnouncesBeforePaying, survivorsCanTarget, CR 601.2c-h);
// a keep-sources plan (potentialWitnessKeeping) is the remaining way. The mirror
// plays the same script on a fork (scriptPrefix), plans the payment on a second
// (tapForPK, try), and counts the candidates the plan left standing
// (targetsKept). A plan that is not found, or consumes nothing, strands no
// candidate.
func (s *State) minTargetsKept(k *prep, p uint8, o Obj, sp *kop.TargetSpec, cost *kop.ManaCost) bool {
	// potentialWitnessKeeping, and the search's own deeper nodes: a plan that
	// keeps every sacrificing source pays the cost with the gaps' mana (the
	// offer's census, availGaps), so no consumption strands a candidate
	held := s.sacSourcesHeld(k, p)
	var av avail
	s.availFor(k, p, &av)
	s.ctrlM[p] = s.ctrlM[p].or(held)
	if av.affords(cost, pipMaskOf(cost), 0) {
		return true
	}
	c := s.objs[o].card
	r := s.fork()
	defer s.unfork()
	r.g.availX = availPlain // the planner prices the printed sources (script_gen.go plainOptions)
	if a := MakeAction(AkCast, TCode(o), 0, 0); r.scriptAny(k) {
		r.scriptPrefix(k, a, p)
		if r.npend != 0 {
			r.flushPrefixTriggers(k)
		}
	}
	t := r.fork()
	defer r.unfork()
	n0 := t.npend
	var pk planner
	pk.try = true
	t.tapForPK(k, p, cost, 0, o, false, false, &pk)
	if !pk.found || !plannerConsumesSac(&pk) {
		return true
	}
	return t.spellTargetsFeasible(k, c, p) || r.targetsKept(k, p, sp, t, o) || t.witnessEndsGameWon(k, p, c, n0)
}

// fodderOnlySources: the sacrifice-another mana sources of p's script walk
// (Diamond Kaleidoscope's "Sacrifice a Prism token: Add one mana of any
// color") whose only sacrifice candidate is the permanent at slot t. A cast
// that targets t cannot also fund its cost by sacrificing t, so gorge's prefix
// search (potential_script.go targetsGone) finds no script through them.
func (s *State) fodderOnlySources(k *prep, p uint8, t Slot) slotMask {
	var fm slotMask
	var cs [MaxPerm]Slot
	for b := s.scriptTapXModelled(k, p); b.any(); b = b.rest() {
		sl := b.first()
		if sl == t || k.tapXAbOf(s.cardAt(sl)).Cost.Flags&kop.CostSac == 0 {
			continue
		}
		if s.tapXCands(k, p, sl, &cs) == 1 && cs[0] == t {
			fm = fm.with(sl)
		}
	}
	return fm
}

// plainCensusPays reports whether p's tier-1 census (availPlain: the pool and
// the printed sources, no gaps and no grants) pays cost outright. gorge's
// payment planner then prices the cast and attaches its planned payment
// (payment_plan.go paymentActionsForPriority), which runs no witness replay:
// the oracle plays that plan at payment (CR 601.2g-h), after the targets, and
// no prefix search runs. A plan that sacrifices more sources than the replay's
// wait budget lets resolve (Pain Distributor's trigger for each of five
// Treasures, witnessWaits) is still the planned payment, whatever a granted
// source (Rishkar's {T}: Add {G}) could do first.
func (s *State) plainCensusPays(k *prep, p uint8, cost *kop.ManaCost) bool {
	f := s.fork()
	defer s.unfork()
	m := f.g.availX
	f.g.availX = availPlain
	var av avail
	f.availFor(k, p, &av)
	ok := av.affords(cost, pipMaskOf(cost), 0)
	f.g.availX = m
	return ok
}
