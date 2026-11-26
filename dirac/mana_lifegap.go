package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A life-only mana ability BESIDE the source's own {T} mana ability
// (kop.GapLifeManaAb but not kop.SoleGapLifeManaAb: the mixed card the census
// gap stays for). A SOLE life-only ability is no gap here: gorge's planner
// prices it as the source's last-resort census unit with Consequence{Life: N}
// (pay.PaymentPlanShapeTierOf), and the kernel joins it to the pain census
// instead (mana_pain.go prepSoleLifeMana). For the mixed card the life
// ability is the alternative the planner does not list a unit for: a potential
// play the planner proves insufficient gets gorge's one prefix search (rules
// scriptSearch.visit), which activates the source and pays the life. The
// kernel models that ability as a census gap whose activation pays the life
// and floats the mana (AvailScript's gaps walk), and beside a paid filter as
// an otherLife step of the filter search (mana_mixgaps.go), so the life funds
// the filter's fee and the filter's mana pays the play.
//
// The source neither taps nor needs to be unsick for its life ability: paying
// life is not a {T} cost, so its mask is not gapMask's (a tapped or
// summoning-sick source stays usable). The life total is not part of the
// filter search's state key, so every activation checks the payer can pay
// (lifeGapAt; lifeGapMask at the root).

// useLifeGap: some card's gapOther mana ability is a life-only one
// (kop.GapLifeManaAb). prepCensusGap sets it, as it is the registered ability
// that counts, not any ability of the table, so its detector holds for none
// (a use bit, not a genPrep field: the fixed State layout is pinned).
var useLifeGap = newGenUse(func(*kop.Table) bool { return false })

// lifeGapSource returns sl's life-only census ability whatever its cost is
// payable: the shape test the filter search uses to route the source to its
// last-resort pass.
func (s *State) lifeGapSource(k *prep, sl Slot) (*kop.Ability, bool) {
	if k.gen == nil || !k.gen.uses.has(useLifeGap) {
		return nil, false
	}
	c := s.cardAt(sl)
	if !k.gen.has(c, gfGapOther) {
		return nil, false
	}
	a := &k.t.Cards[c].Abilities[k.gen.gapOther(c)]
	if !kop.GapLifeManaAb(a) {
		return nil, false
	}
	return a, true
}

// lifeGapAt returns the life-only mana ability of sl when sl is a census gap
// of p with a payable life cost. The life check is per node: the life total is
// not part of the filter search's state key.
func (s *State) lifeGapAt(k *prep, p uint8, sl Slot) (*kop.Ability, bool) {
	a, ok := s.lifeGapSource(k, sl)
	if !ok || int(s.pl[SeatIx(p)].life) < int(a.Cost.N) {
		return nil, false
	}
	return a, true
}

// isLifeGapAt reports whether sl is a payable life-only census gap of p.
func (s *State) isLifeGapAt(k *prep, p uint8, sl Slot) bool {
	_, ok := s.lifeGapAt(k, p, sl)
	return ok
}

// lifeGapSurvives reports whether activating life gap sl leaves its payer above
// 0 life: the lethal guard of gorge's planner (pay/search.go walk: a plan whose
// summed life reaches the payer's life is never a plan) and of its prefix
// search (a branch that ends the game is no node, scriptSearch.visit), CR
// 119.4, 104.3b. A payment to 0 life is payable (lifeGapAt) but no plan and no
// script: the plan falls to the next last-resort source (a Treasure's
// sacrifice) and the search offers no play only that payment funds.
func (s *State) lifeGapSurvives(k *prep, p uint8, sl Slot) bool {
	a, ok := s.lifeGapAt(k, p, sl)
	return ok && int(s.pl[SeatIx(p)].life) > int(a.Cost.N)
}

// lifeGapPlanCost is the payment plan's life arm (mana.go plan): a unit whose
// source is a payable life-only census gap pays its life cost and does not
// tap (a life cost is no {T} cost, lifeGapDo). The arm is the autopay
// surface's: on the lowering surface the ability is no planner unit (the
// recorded life-gap repros' shape), so the plan pays the life only there.
func (s *State) lifeGapPlanCost(k *prep, p uint8, sl Slot) bool {
	if s.cfg&cfgGorgeAutopay == 0 {
		return false
	}
	if a, ok := s.lifeGapAt(k, p, sl); ok {
		s.payLifeCost(k, p, &a.Cost, 0)
		return true
	}
	return false
}

// lifeGapMask: p's life-only census gaps whose life cost is payable now. The
// source stays usable tapped and summoning-sick (only a lost ability or a
// can't-be-activated static takes it out), unlike gapMask's {T} sources.
func (s *State) lifeGapMask(k *prep, p uint8) slotMask {
	if k.gen == nil || !k.gen.uses.has(useLifeGap) {
		return slotMask{}
	}
	var m slotMask
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		sl := b.first()
		if s.pickedForMana(s.perms[sl].obj) {
			continue // a Convoke/Waterbend pick is tapped for that cast, not for mana
		}
		if s.genLost(k, sl) || k.gen.anyCantAct && s.actForbidden(k, sl, true) {
			continue
		}
		if s.isLifeGapAt(k, p, sl) {
			m = m.with(sl)
		}
	}
	return m
}

// lifeGapAnswers is the answers of sl's activation ask in the script search: a
// fixed production asks nothing (one answer), one of any colour asks which
// (W U B R G).
func (s *State) lifeGapAnswers(k *prep, sl Slot) uint8 {
	a := &k.t.Cards[s.cardAt(sl)].Abilities[k.gen.gapOther(s.cardAt(sl))]
	if a.ManaColors == kop.ColAny {
		return 5
	}
	return 1
}

// lifeGapDo activates life gap sl for p with answer ans: pay the life, then
// add the mana. The source is not tapped (a life cost is no {T} cost) and no
// use is spent: the ability is repeatable while the payer has the life.
func (s *State) lifeGapDo(k *prep, p uint8, sl Slot, ans int) {
	c := s.cardAt(sl)
	a := &k.t.Cards[c].Abilities[k.gen.gapOther(c)]
	s.payLifeCost(k, p, &a.Cost, 0)
	col := bits.TrailingZeros8(uint8(a.ManaColors))
	if a.ManaColors == kop.ColAny {
		col = ans
	}
	s.pl[SeatIx(p)].pool[col] += max(a.ManaN, 1)
	s.artTag(k, p, sl, col)
}

// lifePainDo plays sl's sole life-only mana ability (lifeOnlyPain, Treasonous
// Ogre's "Pay 3 life: Add {R}", prepSoleLifeMana): a life pass step of the
// filter search, gorge's payment's last-resort alternative the witness replay
// activates (potentialWitnessReaches). Its cost taps nothing (lifeOnlyPain),
// so only the life is paid and the mana floats; the untapped unit stays in the
// census for the play's own gate to count again (mana_sacsource.go), which is
// why one activation can fund a pipped pip and a spare unit alike. Unlike
// lifeGapDo's gap step, the ability is the planner view's (a census unit has
// no gapOther index) and the branch may take it again later on the path (its
// activation is not once-per-path, fs.lifeUsed).
func (s *State) lifePainDo(k *prep, p uint8, sl Slot, ans int) {
	pc := s.manaPCAt(k, sl)
	if pc.manaAb < 0 || int(pc.manaAb) >= len(k.t.Cards[s.cardAt(sl)].Abilities) {
		return // the admission (canLifeOnlyMana) priced the printed ability; never a silent no-op either way
	}
	a := &k.t.Cards[s.cardAt(sl)].Abilities[pc.manaAb]
	s.payLifeCost(k, p, &a.Cost, 0)
	col := bits.TrailingZeros8(uint8(a.ManaColors))
	if a.ManaColors == kop.ColAny {
		col = ans
	}
	s.pl[SeatIx(p)].pool[col] += max(a.ManaN, 1)
	s.artTag(k, p, sl, col)
}

// lifeGap branches life gap sl at a priority node of the filter search: its
// colour ask when it makes one of any colour (gorge's scriptOrderAnswers puts
// the play's announced colours first), its one answer otherwise. The life
// check is the node's (lifeGapAt): a path that already paid the life may have
// left too little for another activation.
func (fs *filtSearch) lifeGap(k *prep, t *State, sl Slot, funded int, try func(otherStep) bool) bool {
	if !t.isLifeGapAt(k, fs.p, sl) {
		return false
	}
	n := t.lifeGapAnswers(k, sl)
	if n > 1 && !fs.tick() {
		return false
	}
	if n == 1 {
		return try(otherStep{kind: otherLife})
	}
	var order [5]uint8
	for i := range order {
		order[i] = uint8(i)
	}
	needFirst(order[:], fs.need) // gorge's scriptOrderAnswers on the colour ask
	for _, c := range order {
		if try(otherStep{kind: otherLife, ans: c}) {
			return true
		}
	}
	return false
}

// availLifeGap adds each life-only source's census unit to a: one of its fixed
// colour, or of any. availScript's gaps walk and the X ask's window
// (availXWindow: gorge's xAsk pays an announced X through the cast window,
// which activates the source) both count it.
func (s *State) availLifeGap(k *prep, p uint8, a *avail) {
	for b := s.lifeGapMask(k, p); b.any(); b = b.rest() {
		sl := b.first()
		if s.pickCommittedSrc(k, sl) {
			continue // the pick taps it; removeSrc cannot take these units out (cast_xpicked.go)
		}
		ab := &k.t.Cards[s.cardAt(sl)].Abilities[k.gen.gapOther(s.cardAt(sl))]
		n := max(ab.ManaN, 1)
		if ab.ManaColors == kop.ColAny {
			a.addUnits(0x1f, n)
			continue
		}
		a.addUnits(1<<bits.TrailingZeros8(uint8(ab.ManaColors)), n)
	}
}

// windowLifeGapListed: o is a payable life-only source (Treasonous Ogre) that
// gorge's CR 601.2g cast window lists as an "activate" option in its battlefield
// place, whose colour the clamp keeps (decided/want: paymentUsefulColours).
func (s *State) windowLifeGapListed(k *prep, p uint8, o Slot, decided bool, want uint8) bool {
	if k.gen == nil || !k.gen.uses.has(useLifeGap) {
		return false
	}
	if s.genLost(k, o) || k.gen.anyCantAct && s.actForbidden(k, o, true) {
		return false // a lost ability or a can't-be-activated static (lifeGapMask's gate)
	}
	a, ok := s.lifeGapAt(k, p, o)
	if !ok {
		return false
	}
	return !decided || a.ManaColors == kop.ColAny || uint8(a.ManaColors)&want != 0
}

// windowLifeGapTap activates the window's life-only pick o: the life is paid
// and the mana floats. A source of any colour answers the first pip the pool
// leaves owed (the cost less the restricted mana the payment may spend,
// windowOwedCost), else the lowest colour.
func (s *State) windowLifeGapTap(k *prep, p uint8, o Slot, cost *kop.ManaCost) {
	var owed kop.ManaCost
	s.windowOwedCost(k, p, cost, &owed)
	ans := 0
	for c := 0; c < 5; c++ {
		if owed.Pips[c] > s.pl[SeatIx(p)].pool[c] {
			ans = c
			break
		}
	}
	s.lifeGapDo(k, p, o, ans)
}

// windowLifeCtrGapTap taps window gap sl when it is a counter-cost source
// (windowCtrGapTap, which windowPick lists first) or a life-only one (the life
// pays, the mana floats), and reports whether it did.
func (s *State) windowLifeCtrGapTap(k *prep, p uint8, sl Slot, cost *kop.ManaCost, census bool) bool {
	if s.windowCtrGapTap(k, p, sl, census) {
		return true
	}
	if _, life := s.lifeGapSource(k, sl); life {
		s.windowLifeGapTap(k, p, sl, cost)
		return true
	}
	return false
}

// lifeGapPriced reports whether the autopay surface lists the plain cast of o
// that the prefix search pays through p's life-only gaps: gorge's planner
// prices it, the source being a unit of its last-resort phase
// (PlanLastResortChoices), so the cast is a planned one and not a scripted one
// the surface drops. That holds when the life gaps are the board's only gaps
// (beside another kind, a filter included, the planner's phases are not
// modelled) and each activation leaves the payer above 0 life: the planner
// refuses a unit whose pain is at least the life (pay/classify.go), where the
// search's own check (lifeGapAt) allows paying down to 0. A spell the planner
// never prices (an {X} cost: plannedCastCost's shape test) stays dropped.
func (s *State) lifeGapPriced(k *prep, p uint8, o Obj) bool {
	life := s.lifeGapMask(k, p)
	if !life.any() || k.gen.anyFilt && (s.scriptFilt(k, p).any() || s.mixBoardFilt(k, p, filtNone)) {
		return false
	}
	if _, shape := s.plannedCastCost(k, p, o, 0); !shape {
		return false
	}
	if s.scriptGaps(k, p).or(s.scriptRestr(k, p)).or(s.scriptAnyR(k, p)).or(s.scriptTapsDefer(k, p)).or(s.scriptCtr(k, p)).
		or(s.scriptTapXModelled(k, p)).or(s.scriptCtrModelled(k, p)).or(s.anyDifferentGaps(k, p)).any() ||
		k.gen.anyDiscHand && s.dhBoard(k, p) || k.gen.anyComboAny && s.caBoard(k, p) {
		return false
	}
	for b := life; b.any(); b = b.rest() {
		a, _ := s.lifeGapSource(k, b.first())
		if int(s.pl[SeatIx(p)].life) <= int(a.Cost.N) {
			return false
		}
	}
	return true
}

// lifeGapShaped reports whether gorge's cast planner prices o at all (an {X}
// cost, say, it never does), so the planner's phase 2 cannot pay it.
func (s *State) lifeGapShaped(k *prep, p uint8, o Obj) bool {
	_, shape := s.plannedCastCost(k, p, o, 0)
	return shape
}
