package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Granted mana abilities with a life cost (dirac/kgen lower_grantedmana.go
// grantedManaCostShape): Forgotten Monument's Caves' "{T}, Pay 1 life: Add
// one mana of any color". gorge's planner census lists no granted ability
// (pay.PaymentAbility), so it is a census gap its prefix search activates as
// a real activation: the life is paid (CR 119.4: only while the payer's
// life is at least N; a payment to 0 ends the game before the next
// priority, CR 704.5a). The kernel's search step does the same on a fork of
// the working copy (costBranch, the template of mana_grantpain.go
// painBranch), and grantApply replays it for real (grantCostApply).
//
// The grant view (genPrep.grantPC) carries the cost in painLife/painN, with
// painCol left zero: the painland readers (availGrant's addPainAvail, the
// window's pain rider) key on painCol and never price this view.
//
// Lotus Ring's "Sacrifice this creature: Add three mana of any one color",
// granted to the equipped creature, has no {T}: a tapped, attacking or
// summoning-sick creature activates it (CR 602.2b, 302.6 reads only {T}).
// Its view carries grantSac (never manaSacOnly, so the printed Eldrazi
// Spawn readers never see a grant). The search's step sacrifices the
// source on a fork (sacBranch) and the state-based check runs at once, so
// the Equipment is unattached before the play is re-read (CR 704.3,
// 704.5n); grantApply replays it (grantSacApply), and the manual mana
// window sacrifices it as it does a printed sacrifice-only source
// (windowSacOnly).
//
// Lotus Ring's upstream script and Goldspan Dragon's granted Treasure
// ability cost {T} AND the sacrifice ("{T}, Sacrifice this creature: Add
// three mana of any one color"). That view carries manaSac beside manaTap
// (gen.go): priced as the tap grant it is -- only an untapped, non-sick
// bearer activates it (CR 302.6) -- and the search's step taps the source,
// floats the mana and sacrifices it (tapSacBranch, grantTapSacApply), as
// the manual mana window's tap of such a view does (mana_window.go, the
// pc.manaSac arm).

// prepGrantCost records granted mana ability a's cost parts on its view gp:
// the tap-only grant manaTap, the {T} + pay N life one (manaTap and the
// pain), the sacrifice-only one grantSac, and the {T} + sacrifice this one
// manaTap + manaSac (Lotus Ring's upstream script, Goldspan Dragon's
// granted Treasure ability: priced as the tap grant it is, the step and the
// window sacrifice the source, mana_grantcost.go).
func prepGrantCost(gp *pcard, a *kop.Ability) {
	plain := a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0
	gp.manaTap = (a.Cost.Flags == kop.CostTap || a.Cost.Flags == kop.CostTap|kop.CostSacSelf) && plain
	gp.manaSac = a.Cost.Flags == kop.CostTap|kop.CostSacSelf && plain
	if a.Cost.Flags == kop.CostTap|kop.CostPayLife && a.Cost.N >= 1 && plain {
		gp.manaTap, gp.painLife, gp.painN = true, true, a.Cost.N
	}
	if a.Cost.Flags == kop.CostSacSelf && plain {
		gp.grantSac = true
	}
}

// grantSacLive: pc is a granted sacrifice-only view sl can activate now (no
// {T}: tapped or sick alike; only a lost ability stops it).
func (s *State) grantSacLive(sl Slot, pc *pcard) bool {
	return pc.grantSac && s.perms[sl].ovr.flags&(ovLoseAll|ovLoseAllEOT) == 0
}

// grantSacOK: one of sl's granted mana abilities is a live sacrifice-only one.
func (s *State) grantSacOK(k *prep, sl Slot) bool {
	if k.gen == nil || len(k.gen.grantPC) == 0 || !s.grantMay(k, sl) || k.gen.anyCantAct && s.actForbidden(k, sl, true) {
		return false
	}
	var gi [maxGrants]int16
	for i, n := 0, s.grantIdxs(k, sl, &gi); i < n; i++ {
		if s.grantSacLive(sl, s.grantView(k.gen, gi[i], sl)) {
			return true
		}
	}
	return false
}

// grantGapUsable: sl holds a granted mana ability its controller can
// activate now: an untapped source's {T} one, or a sacrifice-only one.
func (s *State) grantGapUsable(k *prep, sl Slot) bool {
	if s.grantIdx(k, sl) < 0 {
		return false
	}
	return s.canTapForMana(k, sl) || s.grantSacOK(k, sl) || s.grantTapOK(k, sl)
}

// grantTapOK is the grant search's "the source can tap" test: canTapForMana,
// or, when its first granted view is a sacrifice-only one, or its own ability
// is (an Eldrazi Spawn's {C} beside Path of Annihilation's granted tap), an
// untapped source that another of its granted abilities can tap.
func (s *State) grantTapOK(k *prep, sl Slot) bool {
	if s.canTapForMana(k, sl) {
		return true
	}
	return s.grantTapByGrant(k, sl, s.manaPCGen(k, sl))
}

// grantTapByGrant is grantTapOK past its canTapForMana test, with pc the
// manaPCGen view of sl, which a caller may have read already in this state
// (filtOtherMana's sacrifice-only test): each manaPCGen walks sl's granted
// abilities.
func (s *State) grantTapByGrant(k *prep, sl Slot, pc *pcard) bool {
	if !pc.grantSac && !(k.sacOnlyMana && pc.manaSacOnly) {
		return false // (a printed sacrifice-only source, an Eldrazi Spawn, taps only by a grant)
	}
	var gi [maxGrants]int16
	for i, n := 0, s.grantIdxs(k, sl, &gi); i < n; i++ {
		if g := s.grantView(k.gen, gi[i], sl); !g.grantSac && s.canTapForManaPC(k, sl, g) {
			return true
		}
	}
	return false
}

// grantHasCost: granted view pc's step pays more than a tap (a life
// payment, or the source's sacrifice -- alone, or after the tap of a
// granted {T}, Sacrifice ability).
func grantHasCost(pc *pcard) bool { return grantCostLife(pc) > 0 || pc.grantSac || pc.manaSac }

// potentialGrantSac adds sl's live granted sacrifice-only abilities to gorge's
// potential pool (availPotentialFor): its walk admits a sacrifice cost with no
// {T}, so a tapped or sick source counts (an any-colour one is open). A
// restricted grant is left out of a payment's pool.
func (s *State) potentialGrantSac(k *prep, sl Slot, payment bool, add func(Slot, uint8, uint8), open *bool) {
	s.potentialGrantTap(k, sl, payment, add, open)
	if len(k.gen.grantPC) == 0 || !s.grantSacOK(k, sl) {
		return
	}
	var gi [maxGrants]int16
	for i, n := 0, s.grantIdxs(k, sl, &gi); i < n; i++ {
		gv := s.grantView(k.gen, gi[i], sl)
		if !gv.grantSac || payment && k.gen.grantRestr[gi[i]] != 0 {
			continue
		}
		if gv.manaCol == kop.ColAny && !gv.manaChosen && !gv.manaMulti {
			*open = true
			continue
		}
		add(sl, s.srcMaskAt(gv, sl), gv.manaN)
	}
}

// availGrantSacOnly adds to a the live granted sacrifice-only abilities of sl,
// a source that cannot tap (tapped or sick) and whose own view is no
// sacrifice-only grant: Lotus Ring's "Sacrifice this creature: Add three mana"
// on a tapped Ancient Cornucopia. The source's own {T} ability and its granted
// {T} ones are spent; the sacrifice needs no {T}, so gorge's census still lists
// it (CR 602.2, 605.1a). The source is sacrificed once: one unit, the grants'
// colours joined, as availGrant joins them, or each its own unit under the
// relaxed walk census.
func (s *State) availGrantSacOnly(k *prep, sl Slot, a *avail) {
	if !s.grantSacOK(k, sl) {
		return
	}
	var gi [maxGrants]int16
	ng := s.grantIdxs(k, sl, &gi)
	if a.xdOn {
		ng = s.grantsAdmitted(k, sl, &gi, ng, &a.xd)
	}
	var mask, n, oneN uint8
	for i := 0; i < ng; i++ {
		gv := s.grantView(k.gen, gi[i], sl)
		if !s.grantSacLive(sl, gv) {
			continue
		}
		if s.g.availX&availWalk != 0 {
			s.addGrantUnits(a, s.srcMaskAt(gv, sl), gv.manaN)
			if gv.manaOne && s.g.availX&availHyp == 0 {
				s.addOneColour(a, gv.manaN) // a grant's "two mana of any one color" is one colour, whichever is chosen
			}
			continue
		}
		mask |= s.srcMaskAt(gv, sl)
		if gv.manaN > n {
			n = gv.manaN
		}
		if gv.manaOne {
			oneN = max(oneN, gv.manaN)
		}
	}
	if mask != 0 {
		s.addGrantUnits(a, mask, n)
		if oneN == n && n > 1 && mask == uint8(kop.ColAny) {
			s.addOneColour(a, n) // the joined units are one colour, whichever is chosen
		}
	}
}

// windowSacOnly: the manual window's view pc is a sacrifice-only mana
// ability, printed (Eldrazi Spawn) or granted (Lotus Ring): the window
// sacrifices the source instead of tapping it.
func windowSacOnly(k *prep, pc *pcard) bool {
	return k.sacOnlyMana && pc.manaSacOnly || pc.grantSac
}

// windowSacNow: sl is a sacrifice-only source the manual window may take
// now, printed or granted.
func (s *State) windowSacNow(k *prep, sl Slot) bool {
	if k.sacOnlyMana && s.canSacOnlyNowAt(k, sl) {
		return true
	}
	return k.gen != nil && len(k.gen.grantPC) != 0 && s.grantSacLive(sl, s.windowGrantPC(k, sl))
}

// sacOnlyVisit is visit's step for candidate i at sl when it cannot tap: its
// live sacrifice-only granted abilities, each a step sacrificing it (a
// "choose a mana ability" ask when there are several).
func (g *grantSrch) sacOnlyVisit(t *State, i int, sl Slot) bool {
	var gi [maxGrants]int16
	var sac [maxGrants]int16
	n := 0
	for j, ng := 0, t.grantIdxs(g.k, sl, &gi); j < ng; j++ {
		if t.grantSacLive(sl, t.grantView(g.k.gen, gi[j], sl)) {
			sac[n] = gi[j]
			n++
		}
	}
	if n == 0 || g.k.gen.anyCantAct && t.actForbidden(g.k, sl, true) {
		return false
	}
	if n > 1 && !g.node() {
		return false
	}
	for j := 0; j < n; j++ {
		pc := t.grantView(g.k.gen, sac[j], sl)
		mask := t.srcMaskAt(pc, sl)
		g.skipPool = !g.grantAdmits(t, sl, sac[j])
		if g.sacBranch(t, i, sl, sac[j], pc, mask, n == 1 || mask&uint8(kop.ColAny) == uint8(kop.ColAny)) {
			return true
		}
	}
	return false
}

// tappedHolderStep is visit's step for candidate i at sl when it cannot tap: its
// granted sacrifice-only abilities, then its printed one (printedSacInPlace).
func (g *grantSrch) tappedHolderStep(t *State, i int, sl Slot, stepped *slotMask) bool {
	return g.ng[i] > 0 && (g.sacOnlyVisit(t, i, sl) || g.printedSacInPlace(t, i, sl, stepped))
}

// printedSacInPlace is visit's step for a gap holder that cannot tap its
// granted ability now (spent by an earlier step) but still has its PRINTED
// sacrifice-only mana ability (an Eldrazi Spawn's), which costs no {T}. Gorge's
// prefix search lists every activation of the gap sources in battlefield
// order, so the tapped holder's sacrifice is branched where its object sits,
// before a later holder's tap, and a sacrifice whose dies trigger stacks
// leaves the line waiting on it while the later holders are tried (CR 605.3b,
// 602.2b). A holder that could not activate its grant at the root (idle: a
// summoning-sick Spawn beside Path of Annihilation) is no gap of gorge's: its
// printed sacrifice is the planner's, priced at each node, never a step (CR
// 302.6, 605.1a). stepped records the holders taken here for printedSacPass.
func (g *grantSrch) printedSacInPlace(t *State, i int, sl Slot, stepped *slotMask) bool {
	if g.idle.has(sl) {
		return false
	}
	*stepped = stepped.with(sl)
	return g.printedSacVisit(t, i, sl)
}

// printedSacPass is visit's last gap step: each holder whose granted {T}
// ability is spent or unusable still has its PRINTED sacrifice-only mana
// ability (an Eldrazi Spawn's), which costs no {T}, and printedSacInPlace did
// not already branch it (stepped). It is branched only once the node is not
// priced and the other gaps are tried.
func (g *grantSrch) printedSacPass(t *State, sacd bool, stepped slotMask) bool {
	for i := 0; i < g.gapN; i++ {
		sl := g.cand[i]
		if g.ng[i] == 0 || g.idle.has(sl) || stepped.has(sl) || t.grantTapOK(g.k, sl) || g.tapx[i] && t.scriptTapX(g.k, g.p).has(sl) {
			continue
		}
		if sacd && (!t.occ.has(sl) || t.perms[sl].obj != g.cobj[i]) {
			continue // sacrificed by an earlier step
		}
		if g.printedSacVisit(t, i, sl) {
			return true
		}
	}
	return false
}

// printedSacVisit steps candidate i's printed sacrifice-only mana ability at
// sl (printedSacPass's one branch).
func (g *grantSrch) printedSacVisit(t *State, i int, sl Slot) bool {
	ppc := t.manaPCGen(g.k, sl)
	if !t.canSacOnlyNow(g.k, sl, ppc) || g.k.gen.anyCantAct && t.actForbidden(g.k, sl, true) {
		return false
	}
	g.skipPool = false
	return g.sacBranch(t, i, sl, -1, ppc, t.srcMaskAt(ppc, sl), true)
}

// grantStepBranch branches the cost step of granted ability gi at sl: a
// sacrifice-only step sacrifices the source (sacBranch), a granted {T},
// Sacrifice step taps it and sacrifices it (tapSacBranch), a life step pays
// the life (costBranch), and a plain one taps (branch).
func (g *grantSrch) grantStepBranch(t *State, i int, sl Slot, gi int16, pc *pcard, mask uint8, ask bool) bool {
	switch {
	case pc.grantSac:
		return g.sacBranch(t, i, sl, gi, pc, mask, ask) // the step sacrifices the source
	case pc.manaSac:
		return g.tapSacBranch(t, i, sl, gi, pc, mask, ask) // the step taps the source and sacrifices it
	case grantCostLife(pc) > 0:
		return g.costBranch(t, i, sl, gi, pc, mask, ask) // the step pays its life
	}
	return g.branch(t, i, sl, gi, mask, pc.manaN, ask)
}

// sacBranch is costBranch for a sacrifice-only granted ability: each colour
// of mask in the source's ask order, the step taken on a fork that floats
// the mana, sacrifices the source and runs the state-based check (the
// Equipment that granted it falls off, CR 704.5n), as grantSacApply
// replays it. A sacrifice whose dies trigger stacks settles first.
func (g *grantSrch) sacBranch(t *State, i int, sl Slot, gi int16, pc *pcard, mask uint8, ask bool) bool {
	if mask == 0 {
		mask = 1 << kop.C
	}
	skip := g.skipPool
	g.skipPool = false
	if ask && mask&(mask-1) != 0 && !g.node() {
		return false
	}
	n := max(pc.manaN, 1)
	pn := n
	if skip {
		pn = 0 // mana the play cannot spend prices nothing
	}
	var order [kop.NumManaKinds]uint8
	no := t.stepColours(g.k, sl, gi, mask, &order)
	needFirst(order[:no], g.need)
	for _, c := range g.subsumeCols(t, order[:no], ask, t.stepNamed(g.k, sl, gi, mask)) {
		u := t.fork()
		u.pl[SeatIx(g.p)].pool[c] += pn
		u.artTag(g.k, g.p, sl, int(c))
		u.grantSacDo(g.k, sl)
		g.path[g.np] = grantStep{i: uint8(i), col: c, n: n, gi: gi, sl: sl, sac: true}
		g.np++
		var ok bool
		sv := g.refreshGrants(u) // (the sacrificed source's grants, or the count a grant's condition reads, are gone below the step: mana_grantlive.go)
		switch {
		case u.over:
			g.stallOf(u) // (a core's stall ends the game at the capFull)
			g.node()     // the game ended: gorge's node finds nothing
		case u.npend != 0:
			ok = g.settle(u)
		default:
			ok = g.visit(u)
		}
		g.restoreGrants(&sv)
		t.unfork()
		if ok {
			return true
		}
		g.np--
	}
	return false
}

// grantSacDo sacrifices sl to pay a granted sacrifice-only ability at
// priority (the template is grantApply's printed Treasure), then runs the
// state-based check that unattaches the Equipment (CR 704.3, 704.5n).
func (s *State) grantSacDo(k *prep, sl Slot) {
	o := s.perms[sl].obj
	s.fireSac(k, sl)
	s.leaveAskedFirst(k, sl) // two exile-instead effects: the script's order ask, answered first (mana_grantsacask.go)
	if s.g.on && s.objs[o].of&ofToken != 0 && s.objs[o].zone == kop.ZoneGraveyard {
		s.ceaseManaToken(o)
	}
	s.sba(k)
	s.answerOrderAsksFirst(k)
}

// grantSacApply replays prefix step st, a granted sacrifice-only ability,
// on s: the mana, the sacrifice, the state-based check.
func (s *State) grantSacApply(k *prep, p uint8, st *grantStep) {
	if ab := restrAbOfStep(k, st); ab != nil {
		if !s.floatRestrAb(k, SeatIx(p), st.sl, ab, int(st.col), st.n) {
			s.capFull(StallRestrSrc) // never a silent spill
		}
	} else {
		s.pl[SeatIx(p)].pool[st.col] += st.n
		s.artTag(k, p, st.sl, int(st.col)) // the search branch's tag: a snow source's unit is snow (mana_snow.go)
	}
	s.grantSacDo(k, st.sl)
}

// sacrificed: the path holds a step that sacrificed its source, or a
// sacrifice-another source's candidate (stepSacX; mana_grantsacx.go): the play's
// only target, or the Equipment's creature, may be gone (waitsForStack
// re-reads the offer), and a later candidate may be the permanent gone
// (visit skips it).
func (g *grantSrch) sacrificed() bool {
	for i := 0; i < g.np; i++ {
		if (g.path[i].kind == stepAct && g.path[i].sac) || g.path[i].gi == stepSacX {
			return true
		}
	}
	return false
}

// witnessEats: the printed plan pays the cast play at this node only by
// sacrificing the play's only target (Larger Than Life on the Eldrazi Spawn
// whose granted {T} floated the {G}, the Spawn's own sacrifice paying the
// {1}). gorge's scriptSearch.priced replays a plan that consumes a permanent
// (witnessReachesFrom) and takes it only when the play is still offered after
// it, so the node does not price the play and the search goes on (gorge's
// potential_plan.go: a witness that sacrifices the play's only target is not a
// way to pay it). xWitnessEats is that replay for a spell.
func (g *grantSrch) witnessEats(t *State, cost *kop.ManaCost) bool {
	if !g.d.hasPlay || !isCastKind(g.d.play.Kind()) || !t.anySacSource(g.k, g.p) {
		return false
	}
	o := Obj(g.d.play.Source())
	if o == NoObj || int(o) >= len(t.objs) {
		return false
	}
	if g.np == 0 && g.d.play.Kind() == AkCast && t.plainCensusPays(g.k, g.p, cost) {
		return false // the root: the census pays outright, so gorge's planned payment prices the cast (plainCensusPays)
	}
	return t.xWitnessEats(g.k, g.p, t.objs[o].card, cost, 0, o, 0)
}

// witnessKeeps: the node's printed plan neither sacrifices the play's only
// target (witnessEats) nor taps it out of an untapped-target requirement
// (playTargetsFeasible) nor waits on the stack, so the play is still offered
// after it and the node may price it (CR 601.2c: a target must remain legal).
func (g *grantSrch) witnessKeeps(t *State, cost *kop.ManaCost) bool {
	return !g.witnessEats(t, cost) && !g.waitsForStack(t) && t.playTargetsFeasible(g.k, g.p, g.d)
}

// playTargetsFeasible reports the play d still has a candidate for every
// mandatory target slot on the post-prefix state s. A prefix activation can
// tap a permanent, and a target whose filter requires it untapped is no
// candidate once tapped (Coordinated Clobbering's "Tap one or two target
// untapped creatures you control" paid by tapping the only untapped creature
// in reach): gorge's scripted prefix re-runs the offer walk on the tapped
// board (potential_script.go potentialWitnessOffers -> rules
// castTargetsAvailable), so it refuses the play and the search must not price
// it. A depletion land's rider is the other direction: the prefix taps and
// sacrifices the only nonbasic land Dwarven Miner's activation could target,
// so the post-prefix ability census has no candidate and the play must not be
// priced either (gorge's potentialWitnessReaches replays the same play).
func (s *State) playTargetsFeasible(k *prep, p uint8, d payDesc) bool {
	if !d.hasPlay || k.t == nil || int(d.card) >= len(k.t.Cards) {
		return true
	}
	if d.play.Kind() == AkActivate {
		ab := d.play.Mode()
		if isGainedMode(ab) || int(ab) >= len(k.t.Cards[d.card].Abilities) {
			return true // a gained or granted ability's target census is its own offer's
		}
		return s.abilityTargetsFeasible(k, &k.t.Cards[d.card].Abilities[ab], p, d.src, 0xff)
	}
	if !isCastKind(d.play.Kind()) {
		return true
	}
	if d.play.Kind() == AkCast && d.play.Target() == castOverloadTgt {
		// CR 702.96b: the overloaded cast's "each" has no target to
		// announce, so the printed spell's target check does not apply.
		return true
	}
	o := Obj(d.play.Source())
	if o == NoObj || int(o) >= len(s.objs) {
		return true
	}
	return s.spellTargetsFeasibleFrom(k, d.card, p, o)
}

// grantCostLife is the life a granted view's activation pays (0: none).
func grantCostLife(pc *pcard) uint8 {
	if pc.painLife && pc.painCol == 0 {
		return pc.painN
	}
	return 0
}

// grantCostPayable: p can pay pc's life now (CR 119.4: life at least N; no
// life to pay is always payable, so a player at 0 or less life under Herald
// of Eternal Dawn still taps a granted source that costs none).
func (s *State) grantCostPayable(p uint8, pc *pcard) bool {
	n := grantCostLife(pc)
	return n == 0 || int(s.pl[SeatIx(p)].life) >= int(n)
}

// grantAlts counts the granted abilities of sl a "choose a mana ability" ask
// lists while sl can tap: every one but a life payment p cannot make.
func (s *State) grantAlts(k *prep, p uint8, sl Slot, gi *[maxGrants]int16, ng int) int {
	n := 0
	for i := 0; i < ng; i++ {
		if s.grantCostPayable(p, s.grantView(k.gen, gi[i], sl)) {
			n++
		}
	}
	return n
}

// costBranch is branch for a granted ability with a life cost: each colour
// of mask in the source's ask order, the step taken on a fork that taps the
// source, floats the mana and pays the life, as grantCostApply replays it.
func (g *grantSrch) costBranch(t *State, i int, sl Slot, gi int16, pc *pcard, mask uint8, ask bool) bool {
	if mask == 0 {
		mask = 1 << kop.C
	}
	skip := g.skipPool
	g.skipPool = false
	if ask && mask&(mask-1) != 0 && !g.node() {
		return false
	}
	n := max(pc.manaN, 1)
	pn := n
	if skip {
		pn = 0 // mana the play cannot spend prices nothing
	}
	var order [kop.NumManaKinds]uint8
	no := t.stepColours(g.k, sl, gi, mask, &order)
	needFirst(order[:no], g.need)
	for _, c := range g.subsumeCols(t, order[:no], ask, t.stepNamed(g.k, sl, gi, mask)) {
		u := t.fork()
		u.perms[sl].flags |= pfTapped
		u.pl[SeatIx(g.p)].pool[c] += pn
		u.artTag(g.k, g.p, sl, int(c))
		if g.k.gen.anyTapExtra && !skip {
			xv := u.srcExtraPick(g.k, sl, pc, int(c))
			u.addXv(SeatIx(g.p), &xv)
		}
		u.grantCostPay(g.k, g.p, pc)
		g.path[g.np] = grantStep{i: uint8(i), col: c, n: n, gi: gi, sl: sl}
		g.np++
		var ok bool
		if u.pl[SeatIx(g.p)].life <= 0 {
			g.stallOf(u) // (a step's stall the node latched after visit's entry; a core's stall ends the game at the capFull)
			g.node()     // the payment ends the game before the next priority (CR 704.5a): gorge's node finds nothing
		} else {
			ok = g.next(u, sl)
		}
		t.unfork()
		if ok {
			return true
		}
		g.np--
	}
	return false
}

// grantCostPay pays granted view pc's life for p: a cost, not damage (CR
// 119.4), so no damage replacement reads it (mana_pain.go painDamage's
// painLife arm).
func (s *State) grantCostPay(k *prep, p uint8, pc *pcard) {
	if n := grantCostLife(pc); n > 0 {
		s.loseLife(k, SeatIx(p), int(n))
	}
}

// tapSacBranch is costBranch for a granted {T}, Sacrifice ability (Lotus
// Ring's upstream script, Goldspan Dragon's granted Treasure ability): the
// step taps the source, floats the mana and sacrifices it on a fork, and the
// state-based check runs at once (the Equipment that granted it falls off,
// CR 704.3, 704.5n), as grantTapSacApply replays it. The source must be
// untapped and non-sick to reach it (canTapForManaPC, CR 302.6): priced as
// the tap grant the view is, never a tapped bearer's step.
func (g *grantSrch) tapSacBranch(t *State, i int, sl Slot, gi int16, pc *pcard, mask uint8, ask bool) bool {
	if mask == 0 {
		mask = 1 << kop.C
	}
	skip := g.skipPool
	g.skipPool = false
	if ask && mask&(mask-1) != 0 && !g.node() {
		return false
	}
	n := max(pc.manaN, 1)
	pn := n
	if skip {
		pn = 0 // mana the play cannot spend prices nothing
	}
	var order [kop.NumManaKinds]uint8
	no := t.stepColours(g.k, sl, gi, mask, &order)
	needFirst(order[:no], g.need)
	for _, c := range g.subsumeCols(t, order[:no], ask, t.stepNamed(g.k, sl, gi, mask)) {
		u := t.fork()
		u.perms[sl].flags |= pfTapped
		u.pl[SeatIx(g.p)].pool[c] += pn
		u.artTag(g.k, g.p, sl, int(c))
		if g.k.gen.anyTapExtra && !skip {
			xv := u.srcExtraPick(g.k, sl, pc, int(c))
			u.addXv(SeatIx(g.p), &xv)
		}
		u.grantSacDo(g.k, sl)
		g.path[g.np] = grantStep{i: uint8(i), col: c, n: n, gi: gi, sl: sl, sac: true}
		g.np++
		var ok bool
		sv := g.refreshGrants(u) // (the sacrificed source's grants, or the count a grant's condition reads, are gone below the step: mana_grantlive.go)
		switch {
		case u.over:
			g.stallOf(u) // (a core's stall ends the game at the capFull)
			g.node()     // the game ended: gorge's node finds nothing
		case u.npend != 0:
			ok = g.settle(u)
		default:
			ok = g.visit(u)
		}
		g.restoreGrants(&sv)
		t.unfork()
		if ok {
			return true
		}
		g.np--
	}
	return false
}

// grantSacStepApply replays prefix step st, a sacrifice step: a granted {T},
// Sacrifice ability's tap, mana, sacrifice and state-based check
// (grantTapSacApply), or a sacrifice-only one's mana, sacrifice and
// state-based check (grantSacApply).
func (s *State) grantSacStepApply(k *prep, p uint8, st *grantStep) {
	GrantManaStats.Sac.Add(1) // the granted sacrifice step (mana_grantcost.go)
	if st.gi >= 0 && s.grantView(k.gen, st.gi, st.sl).manaSac {
		GrantManaStats.Taps.Add(1) // the granted {T} step's mana
		s.grantTapSacApply(k, p, st)
		return
	}
	s.grantSacApply(k, p, st)
}

// grantTapSacApply replays prefix step st, a granted {T}, Sacrifice ability,
// on s: the tap, the mana, the sacrifice, the state-based check.
func (s *State) grantTapSacApply(k *prep, p uint8, st *grantStep) {
	sl := st.sl
	pc := s.grantView(k.gen, st.gi, sl)
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	if ab := restrAbOfStep(k, st); ab != nil {
		if !s.floatRestrAb(k, SeatIx(p), sl, ab, int(st.col), st.n) {
			s.capFull(StallRestrSrc) // never a silent spill
		}
	} else {
		s.pl[SeatIx(p)].pool[st.col] += st.n
		s.artTag(k, p, sl, int(st.col)) // the search branch's tag: a snow source's unit is snow (mana_snow.go)
	}
	if k.gen.anyTapExtra {
		xv := s.srcExtraPick(k, sl, pc, int(st.col))
		s.addXv(SeatIx(p), &xv)
	}
	s.grantSacDo(k, sl)
}

// grantCostApply replays prefix step st, a granted ability with a life cost,
// on s: the tap, the mana, the life.
func (s *State) grantCostApply(k *prep, p uint8, st *grantStep) {
	sl := st.sl
	pc := s.grantView(k.gen, st.gi, sl)
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	if ab := restrAbOfStep(k, st); ab != nil {
		if !s.floatRestrAb(k, SeatIx(p), sl, ab, int(st.col), st.n) {
			s.capFull(StallRestrSrc) // never a silent spill
		}
	} else {
		s.pl[SeatIx(p)].pool[st.col] += st.n
		s.artTag(k, p, sl, int(st.col)) // the search branch's tag: a snow source's unit is snow (mana_snow.go)
	}
	if k.gen.anyTapExtra {
		xv := s.srcExtraPick(k, sl, pc, int(st.col))
		s.addXv(SeatIx(p), &xv)
	}
	s.grantCostPay(k, p, pc)
}

// windowPayableGrant is the view a window activation of sl takes among its
// admitted granted abilities gi[:n]: the first whose life its controller can
// pay (CR 119.4), else &noManaPC.
func (s *State) windowPayableGrant(k *prep, sl Slot, gi *[maxGrants]int16, n int) *pcard {
	for i := 0; i < n; i++ {
		if g := s.grantView(k.gen, gi[i], sl); s.grantCostPayable(s.perms[sl].ctrl, g) {
			return g
		}
	}
	return &noManaPC
}

// windowGrantYield is the mana a window activation of sl taking colour col makes:
// pc's own count, except when sl's own ability does not make col and an ability
// an effect granted it does (a Forest under Weirding Wood, "{T}: Add two mana of
// any one color", answering {R}): the first such grant the payment may spend and
// its controller can pay for makes its own count, not the Forest's one
// (CR 605.3b; gorge lists that ability and activates it).
func (s *State) windowGrantYield(k *prep, sl Slot, pc *pcard, col int) uint8 {
	if k.gen == nil || len(k.gen.grantPC) == 0 || s.windowOwnMask(k, sl)&(1<<col) != 0 || !s.grantMay(k, sl) {
		return pc.manaN
	}
	var gi [maxGrants]int16
	d := s.payDescNow(NoObj)
	n := s.grantsAdmitted(k, sl, &gi, s.grantIdxs(k, sl, &gi), &d)
	for i := 0; i < n; i++ {
		if g := s.grantView(k.gen, gi[i], sl); s.srcMaskAt(g, sl)&(1<<col) != 0 && s.grantCostPayable(s.perms[sl].ctrl, g) {
			return g.manaN
		}
	}
	return pc.manaN
}

// windowGrantAnswer is the view the window's tap of sl takes when its answer
// col is a colour the source's printed ability pc cannot make: the granted
// one that does. gorge's choose-a-mana-ability ask lists the printed ability
// first, the granted one when only it makes the owed colour
// (mana_grantedoffers.go), so the tap floats the granted ability's
// production at its own size and pays its own cost (CR 106.4) --
// windowGrantLife already prices its life cost on this. pc again when the
// printed ability makes col or no admitted grant does.
func (s *State) windowGrantAnswer(k *prep, sl Slot, col int, pc *pcard) *pcard {
	if k.gen == nil || len(k.gen.grantPC) == 0 || !s.grantMay(k, sl) || s.srcMaskAt(pc, sl)&(1<<uint(col)) != 0 {
		return pc
	}
	var gi [maxGrants]int16
	n := s.grantIdxs(k, sl, &gi)
	if n > 0 {
		d := s.payDescNow(NoObj)
		if n = s.grantsAdmitted(k, sl, &gi, n, &d); n > 0 {
			for i := 0; i < n; i++ {
				g := s.grantView(k.gen, gi[i], sl)
				if s.srcMaskAt(g, sl)&(1<<uint(col)) != 0 && s.grantCostPayable(s.perms[sl].ctrl, g) {
					return g
				}
			}
		}
	}
	return pc
}

// windowGrantLife is the life a window activation of sl taking colour col
// pays: pc's own when the window took a life-cost grant view, else, when
// sl's own ability does not make col, the least a granted ability making col
// costs (0 when a free one does). Forgotten Monument's Cave owing {R} beside
// its own {C} pays 1 life, as gorge's window activates the grant.
func (s *State) windowGrantLife(k *prep, sl Slot, pc *pcard, col int) uint8 {
	if n := grantCostLife(pc); n > 0 {
		return n
	}
	if k.gen == nil || len(k.gen.grantPC) == 0 || s.windowOwnMask(k, sl)&(1<<col) != 0 || !s.grantMay(k, sl) {
		return 0
	}
	var gi [maxGrants]int16
	least, found := uint8(0), false
	for i, n := 0, s.grantIdxs(k, sl, &gi); i < n; i++ {
		g := s.grantView(k.gen, gi[i], sl)
		if s.srcMaskAt(g, sl)&(1<<col) == 0 || !s.grantCostPayable(s.perms[sl].ctrl, g) {
			continue
		}
		if l := grantCostLife(g); !found || l < least {
			least, found = l, true
		}
	}
	return least
}

// potentialGrantTap adds the granted {T} abilities of an untapped source whose
// own ability cannot tap (an Eldrazi Spawn's sacrifice-only {C} beside Path of
// Annihilation's "{T}: Add one mana of any color") to gorge's potential pool:
// its walk admits the grant beside the printed ability (potentialManaFor), an
// any-colour one unbounded (CR 605.1a). A restricted grant is left out of a
// payment's pool, as potentialGrantSac leaves it.
func (s *State) potentialGrantTap(k *prep, sl Slot, payment bool, add func(Slot, uint8, uint8), open *bool) {
	if k.gen == nil || len(k.gen.grantPC) == 0 || !s.grantMay(k, sl) || s.canTapForMana(k, sl) || !s.grantTapOK(k, sl) ||
		k.gen.anyCantAct && s.actForbidden(k, sl, true) {
		return
	}
	var gi [maxGrants]int16
	for i, n := 0, s.grantIdxs(k, sl, &gi); i < n; i++ {
		gv := s.grantView(k.gen, gi[i], sl)
		if gv.grantSac || !s.canTapForManaPC(k, sl, gv) || payment && k.gen.grantRestr[gi[i]] != 0 {
			continue
		}
		if gv.manaCol == kop.ColAny && !gv.manaChosen && !gv.manaMulti {
			*open = true
			continue
		}
		add(sl, s.srcMaskAt(gv, sl), gv.manaN)
	}
}
