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

// prepGrantCost records granted mana ability a's life cost on its view gp.
func prepGrantCost(gp *pcard, a *kop.Ability) {
	if a.Cost.Flags == kop.CostTap|kop.CostPayLife && a.Cost.N >= 1 && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0 {
		gp.manaTap, gp.painLife, gp.painN = true, true, a.Cost.N
	}
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
