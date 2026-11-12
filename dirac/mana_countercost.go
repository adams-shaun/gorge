package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Counter-cost mana (Wall of Roots: "Put a -0/-1 counter on Wall of
// Roots: Add {G}. Activate only once each turn."). gorge's planner census
// cannot execute an AddCounter cost (pay.PlanLastResortCostOK), so the
// source is a census gap with a fixed relaxed alternative (potential_plan.go
// paymentPlanRelaxProof: "Wall of Roots' counter"): a play the lands cannot
// pay but the relaxed census can is searched for a prefix that activates
// the gap first (potentialPrefixScript), the sources then paying the rest.
// The kernel models it as one more script gap kind beside the each-colour,
// restricted and sacrifice-only ones (script_gen.go): its mana in the gaps
// walk (availCtr), and at the play the chain's activations in battlefield
// order, a source with a plain {T} first ability taken through both branches
// its activation ask lists (the {T} tap before the counter cost), an
// any-colour ability (Vivid Grove's second) branched on its colour
// ask (ctrScript), until the walk lists the play. Each activation puts a
// -0/-1 counter (kop.CtrM0M1, read by ptGen) and spends the once-each-turn
// use (perm.actUsed). Everything is behind prep.ctrMana.
//
// The all-at-once removal shape (kop.CtrEachManaAb: Ramos, Dragon Engine's
// "Remove five +1/+1 counters from Ramos: Add {W}{W}{U}{U}{B}{B}{R}{R}{G}{G}.
// Activate only once each turn.") is the same gap: gorge relaxes it to one
// free alternative holding the whole production. Its pcard keeps the whole
// colour mask (no prepEachMana split: it is no tap source), the gaps walk
// adds every colour (not one of them), the activation removes the counters
// as the activation path's CostRemoveCtr does, and the gap is offered only
// while the source holds them (gorge manaAbilityPayablePool).

// ctrManaAb reports whether mana ability a costs only counters put on its
// source, once each turn, with a fixed production.
func ctrManaAb(a *kop.Ability) bool { return kop.CtrManaAb(a) }

// ctrAbOf is card c's counter-cost mana ability (prep.ctrAb): its first mana
// ability (Wall of Roots, Ramos), or a later {T} one beside a plain {T} first
// (Weather Maker), nil none.
func (k *prep) ctrAbOf(c CardID) *kop.Ability {
	if int(c) >= len(k.ctrAb) || k.ctrAb[c] < 0 {
		return nil
	}
	return &k.t.Cards[c].Abilities[k.ctrAb[c]]
}

// ctrSecond: card c's counter-cost ability is a later one beside a plain {T}
// first (Weather Maker), so activating the source asks which ability.
func (k *prep) ctrSecond(c CardID) bool {
	return int(c) < len(k.ctrAb) && k.ctrAb[c] >= 0 && k.ctrAb[c] != k.cards[c].manaAb
}

// scriptCtr: p's counter-cost sources that can be activated now (a Wall of
// Roots has no {T}: summoning sickness does not matter, CR 302.6; a source
// whose cost taps it must be untapped and, a creature, not sick), a slot mask.
//
//go:noinline
func (s *State) scriptCtr(k *prep, p uint8) slotMask {
	var m slotMask
	if !k.ctrMana || DebugNoCtrMana {
		return m
	}
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		sl := b.first()
		c := s.cardAt(sl)
		a := k.ctrAbOf(c)
		if a == nil || s.perms[sl].actUsed&(1<<uint(k.ctrAb[c])) != 0 || s.genLost(k, sl) || k.gen != nil && k.gen.anyCantAct && s.actForbidden(k, sl, true) {
			continue
		}
		if a.Cost.Flags&kop.CostRemoveCtr != 0 && s.perms[sl].ctrN(s, int(a.Cost.Ctr)) < int(a.Cost.N) {
			continue // too few counters to remove (legal.go actCostOK's CostRemoveCtr)
		}
		if a.Cost.Flags&kop.CostTap != 0 && !s.canTapForMana(k, sl) {
			continue // its {T}: the source is untapped and not summoning sick (the second ability beside a plain {T} first)
		}
		m = m.with(sl)
	}
	return m
}

// availCtr adds p's counter-cost sources' mana to the gaps walk.
//
//go:noinline
func (s *State) availCtr(k *prep, p uint8, av *avail) {
	for b := s.scriptCtr(k, p); b.any(); b = b.rest() {
		c := s.cardAt(b.first())
		a := k.ctrAbOf(c)
		if a.Cost.Flags&kop.CostTap != 0 && k.ctrSecond(c) {
			// A later {T} ability beside a plain {T} first (Weather Maker's
			// {C}{C} beside its any-colour tap): both are one tap, and the
			// plain walk already counts the first's unit, which pays any
			// generic unit of the second's. Only the units beyond it are new
			// (CR 605.3b: one activation per untapped source). A {C} pip
			// the plain unit cannot pay is not priced by this.
			cols, n := tapXUnit(a)
			if cols == 1<<kop.C {
				av.addUnits(cols, n-1)
				continue
			}
		}
		addCtrUnits(av, a)
	}
}

// addCtrUnits adds counter-cost ability a's mana to av: every colour at once
// for the all-at-once shape (Ramos), else one unit of its fixed colour.
func addCtrUnits(av *avail, a *kop.Ability) {
	cols, n := tapXUnit(a)
	if a.ManaEach {
		for c := 0; c < kop.NumManaKinds; c++ {
			if cols&(1<<c) != 0 {
				av.addUnits(1<<c, n) // every colour at once, not a choice of one
			}
		}
		return
	}
	av.addUnits(cols, n)
}

// windowCtrOffer: counter-cost source o is one gorge's ward or unless window
// lists and can take now (it asks no colour), and its colour adds something the
// clamp keeps.
//
//go:noinline
func (s *State) windowCtrOffer(k *prep, p uint8, o Slot, decided bool, want uint8) bool {
	if !k.ctrMana || !s.scriptCtr(k, p).has(o) {
		return false
	}
	return !decided || uint8(k.ctrAbOf(s.cardAt(o)).ManaColors)&want != 0
}

// ctrGap activates counter-cost source sl for p: its counters, its use this
// turn, its mana into the pool (gorge's activation: the cost's
// CounterChange, then the mana). An any-colour ability (Vivid Grove's second
// one) asks its colour: col is the answer, the lowest colour when negative.
//
//go:noinline
func (s *State) ctrGap(k *prep, p uint8, sl Slot, col int) {
	c := s.cardAt(sl)
	a := k.ctrAbOf(c)
	pm := &s.perms[sl]
	if a.Cost.Flags&kop.CostRemoveCtr != 0 {
		pm.seeCtr(s) // the activation path's CostRemoveCtr (mana_window.go)
		pm.subCtrN(s, int(a.Cost.Ctr), int(a.Cost.N))
	} else {
		n := s.placeCounters(k, sl, int(a.Cost.Ctr), int(a.Cost.N), p, ctrPutCost) // a cost: only the non-EffectOnly$ replacements
		if n > 0 && k.anyCtrAdded {
			s.ctrNoteBy(k, sl, int(a.Cost.Ctr), p) // a cost: gorge pay_engine publishes the payer
		}
	}
	if a.Cost.Flags&kop.CostOncePerTurn != 0 {
		pm.actUsed |= 1 << uint(k.ctrAb[c])
	}
	taps := a.Cost.Flags&kop.CostTap != 0
	if taps {
		pm.flags |= pfTapped // the second ability's {T} (Weather Maker)
		s.manaTapped(k, sl)
	}
	s.touchSl(sl)
	cols, n := tapXUnit(a)
	if ctrAnyColour(a) {
		if col < 0 {
			col = bits.TrailingZeros8(cols)
		}
		cols = 1 << uint(col)
	}
	for c := 0; c < kop.NumManaKinds; c++ {
		if cols&(1<<c) != 0 {
			s.poolAdd(p&1, c, int(n))
			if taps {
				s.artTag(k, p, sl, c)
			}
		}
	}
	if taps {
		s.tapExtraStall(k, sl)
	}
}

// ctrAnyColour: counter-cost ability a makes one mana of a colour its
// activation asks (Vivid Grove's "{T}, remove a charge counter: add one mana
// of any color"), not a fixed colour or every colour at once.
func ctrAnyColour(a *kop.Ability) bool {
	return !a.ManaEach && a.ManaColors&(a.ManaColors-1) != 0
}

// ctrAnsCol is the colour counter-cost source sl's mana ability makes for the
// prefix search's answer ans of play a (ctrScript's path): an any-colour
// ability's ask is answered in scriptOrderAnswers' order, the play's
// announced colours first (tapXSearch.cols), 0 the first one; -1 for a fixed
// colour ability (Wall of Roots, Ramos), whose step asks nothing.
func (s *State) ctrAnsCol(k *prep, a Action, sl Slot, ans int) int {
	ab := k.ctrAbOf(s.cardAt(sl))
	if ab == nil || !ctrAnyColour(ab) {
		return -1
	}
	var order [5]uint8
	for c := range order {
		order[c] = uint8(c)
	}
	needFirst(order[:], s.playNeed(k, a))
	return int(order[ans])
}

// ctrChainTap applies the prefix search's chain step for gap sl, answer ans
// of play a (ctrScript's node build, scriptPrefix's replay): a counter-cost
// source with a plain {T} first mana ability answers 0 with that ability's
// tap (gorge's activation ask lists it first: Activate Vivid Creek for mana,
// Add U), later answers its counters, use and mana (ctrGap; the SBA a -0/-1
// counter may kill with), any other gap its tap (tapGap).
func (s *State) ctrChainTap(k *prep, p uint8, a Action, sl Slot, ctr, restr slotMask, ans int, held *[kop.NumManaKinds]uint8) {
	if ctr.has(sl) {
		off := 0
		if k.ctrSecond(s.cardAt(sl)) { // the ask's plain {T} first ability is answer 0
			if ans == 0 {
				s.ctrAb1(k, p, sl)
				return
			}
			off = 1
		}
		s.ctrGap(k, p, sl, s.ctrAnsCol(k, a, sl, ans-off))
		s.sba(k) // the script returns to priority after the step: a Wall of Roots out of toughness dies before the next one
		return
	}
	s.tapGap(k, p, sl, restr, held)
}

// ctrAb1 applies the chain's ab1 branch for counter-cost source sl: the
// card's plain {T} first mana ability (ctrSecond), tapped, its fixed mana
// into the pool — the counters and the once-each-turn use stay (gorge's
// activation: the {T} cost, then the mana).
//
//go:noinline
func (s *State) ctrAb1(k *prep, p uint8, sl Slot) {
	pm := &s.perms[sl]
	pm.flags |= pfTapped
	s.manaTapped(k, sl)
	s.touchSl(sl)
	c := s.cardAt(sl)
	cols, n := tapXUnit(&k.t.Cards[c].Abilities[k.cards[c].manaAb])
	for cc := 0; cc < kop.NumManaKinds; cc++ {
		if cols&(1<<cc) != 0 {
			s.poolAdd(p&1, cc, int(n))
			s.artTag(k, p, sl, cc)
		}
	}
	s.tapExtraStall(k, sl)
}

// ctrScript is gorge's prefix search (scriptSearch.visit) for play a over the
// chain order: p's gaps tapped in battlefield order, a counter-cost source
// with a plain {T} first mana ability branched on the two branches its
// activation ask lists (the {T} tap first, then the counter-cost one), an
// any-colour counter ability branched on its five colour answers (ctrAnsCol's
// order, every answer an activation poses branched on, in engine order),
// depth first; at each node after a tap the walk (restrOptions) is asked for
// a. It returns the number of gaps the found script taps (path holds their
// answers, 0 where the gap asks nothing) and whether a was offered at that
// node, dead there as scriptDeadEnd reads it (restricted mana floats and the
// plain walk does not price the play). Each node is rebuilt on a fresh fork
// (the scriptDFS walk), so the caller's state is untouched.
func (s *State) ctrScript(k *prep, a Action, p uint8, order []Slot, ctr, restr slotMask, path []uint8, noGrant bool) (int, bool, bool) {
	var d Decision
	depth := 0 // gaps tapped at this node: order[:depth], answers path[:depth]
	for {
		if depth > 0 {
			t := s.fork()
			t.g.noGrant = noGrant // (scriptDeadEnd's restrictedGrantLive count; the root's is unset)
			var held [kop.NumManaKinds]uint8
			for j := 0; j < depth; j++ {
				t.ctrChainTap(k, p, a, order[j], ctr, restr, int(path[j]), &held)
			}
			t.restrOptions(k, &d, p)
			if hasOption(&d, a) {
				dead := held != ([kop.NumManaKinds]uint8{}) && !t.scriptCompletes(k, p, a)
				s.unfork()
				return depth, true, dead
			}
			s.unfork()
		}
		if depth < len(order) { // the node's first child
			path[depth] = 0
			depth++
			continue
		}
		for { // a leaf: the deepest gap with an answer left, else done
			if depth == 0 {
				return 0, false, true
			}
			j := depth - 1
			c := s.cardAt(order[j])
			nch := 1
			if ab := k.ctrAbOf(c); ab != nil {
				nch = 1
				if k.ctrSecond(c) {
					nch = 2 // the plain {T} first ability before the counter one (ctrChainTap)
				}
				if ctrAnyColour(ab) {
					nch += 4 // the colour ask's five answers (ctrAnsCol)
				}
			}
			if int(path[j])+1 < nch {
				path[j]++
				break
			}
			depth--
		}
	}
}

// sbaCtrTough is CR 704.5f for a creature whose toughness only -0/-1
// counters lowered: the SBA's no-damage shortcut reads the printed
// toughness, so the counters are read here, once per pass.
//
//go:noinline
func (s *State) sbaCtrTough(k *prep, die, gy *slotMask) {
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if die.has(sl) || s.perms[sl].ctrN(s, int(kop.CtrM0M1)) == 0 || !s.isCreature(k, sl) {
			continue
		}
		if _, t := s.pt(k, sl); t <= 0 {
			*die = die.with(sl)
			*gy = gy.with(sl)
		}
	}
}

// DebugNoCtrMana hides the counter-cost gaps (the wallroots mutation test
// proves the pool's plays need them).
var DebugNoCtrMana bool
