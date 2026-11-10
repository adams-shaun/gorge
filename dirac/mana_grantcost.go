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

// prepGrantCost records granted mana ability a's life or sacrifice cost on
// its view gp.
func prepGrantCost(gp *pcard, a *kop.Ability) {
	plain := a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0
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
	return s.canTapForMana(k, sl) || s.grantSacOK(k, sl)
}

// grantTapOK is the grant search's "the source can tap" test: canTapForMana,
// or, when its first granted view is a sacrifice-only one, an untapped
// source that another of its granted abilities can tap.
func (s *State) grantTapOK(k *prep, sl Slot) bool {
	if s.canTapForMana(k, sl) {
		return true
	}
	if !s.manaPCGen(k, sl).grantSac {
		return false
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
// payment, or the source's sacrifice).
func grantHasCost(pc *pcard) bool { return grantCostLife(pc) > 0 || pc.grantSac }

// potentialGrantSac adds sl's live granted sacrifice-only abilities to gorge's
// potential pool (availPotentialFor): its walk admits a sacrifice cost with no
// {T}, so a tapped or sick source counts (an any-colour one is open). A
// restricted grant is left out of a payment's pool.
func (s *State) potentialGrantSac(k *prep, sl Slot, payment bool, add func(Slot, uint8, uint8), open *bool) {
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
	for _, c := range order[:no] {
		u := t.fork()
		u.pl[SeatIx(g.p)].pool[c] += pn
		u.artTag(g.k, g.p, sl, int(c))
		u.grantSacDo(g.k, sl)
		g.path[g.np] = grantStep{i: uint8(i), col: c, n: n, gi: gi, sl: sl, sac: true}
		g.np++
		var ok bool
		switch {
		case u.over:
			g.node() // the game ended: gorge's node finds nothing
		case u.npend != 0:
			ok = g.settle(u)
		default:
			ok = g.visit(u)
		}
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
	s.leave(k, sl, kop.ZoneGraveyard, false)
	if s.g.on && s.objs[o].of&ofToken != 0 && s.objs[o].zone == kop.ZoneGraveyard {
		s.ceaseManaToken(o)
	}
	s.sba(k)
}

// grantSacApply replays prefix step st, a granted sacrifice-only ability,
// on s: the mana, the sacrifice, the state-based check.
func (s *State) grantSacApply(k *prep, p uint8, st *grantStep) {
	if ab := restrAbOfStep(k, st); ab != nil {
		if !s.floatRestrAb(k, p&1, st.sl, ab, int(st.col), st.n) {
			s.capFull(StallRestrSrc) // never a silent spill
		}
	} else {
		s.pl[SeatIx(p)].pool[st.col] += st.n
	}
	s.grantSacDo(k, st.sl)
}

// sacrificed: the path holds a step that sacrificed its source (the play's
// only target, or the Equipment's creature, may be gone: waitsForStack
// re-reads the offer).
func (g *grantSrch) sacrificed() bool {
	for i := 0; i < g.np; i++ {
		if g.path[i].kind == stepAct && g.path[i].sac {
			return true
		}
	}
	return false
}

// grantCostLife is the life a granted view's activation pays (0: none).
func grantCostLife(pc *pcard) uint8 {
	if pc.painLife && pc.painCol == 0 {
		return pc.painN
	}
	return 0
}

// grantCostPayable: p can pay pc's life now (CR 119.4: life at least N).
func (s *State) grantCostPayable(p uint8, pc *pcard) bool {
	return int(s.pl[p&1].life) >= int(grantCostLife(pc))
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
	for _, c := range order[:no] {
		u := t.fork()
		u.perms[sl].flags |= pfTapped
		u.pl[g.p&1].pool[c] += pn
		u.artTag(g.k, g.p, sl, int(c))
		if g.k.gen.anyTapExtra && !skip {
			xv := u.srcExtraPick(g.k, sl, pc, int(c))
			u.addXv(g.p&1, &xv)
		}
		u.grantCostPay(g.k, g.p, pc)
		g.path[g.np] = grantStep{i: uint8(i), col: c, n: n, gi: gi, sl: sl}
		g.np++
		var ok bool
		if u.pl[g.p&1].life <= 0 {
			g.node() // the payment ends the game before the next priority (CR 704.5a): gorge's node finds nothing
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
		s.loseLife(k, p&1, int(n))
	}
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
		if !s.floatRestrAb(k, p&1, sl, ab, int(st.col), st.n) {
			s.capFull(StallRestrSrc) // never a silent spill
		}
	} else {
		s.pl[p&1].pool[st.col] += st.n
	}
	if k.gen.anyTapExtra {
		xv := s.srcExtraPick(k, sl, pc, int(st.col))
		s.addXv(p&1, &xv)
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
