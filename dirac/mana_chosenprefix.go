package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A chosen-colour widened pair (Sunken Citadel's land-only {C}{C} beside its
// {C}) is no unit of gorge's census, so a play the census alone cannot pay is
// a potential play whose prefix search activates the pair sources (CR
// 601.2g, 602.2b): the oracle plays the prefix before the play's targets,
// floating the mana (a pair's two units are restricted, State.rb).
// planChosenTrial pays the same play with the pair in the payment, which taps
// the source too late. chosenPairPrefix plays gorge's search order (rules
// scriptSearch.visit): the pair sources, in battlefield order, each first at
// its plain {T} ability and then at the pair — the two answers of the
// source's "choose a mana ability" ask, in ability order — stopping at the
// first node the play prices at, and leaving a source out only when neither
// of its answers can still fund the play: the search's first-found path,
// which its lex order makes the answers' greedy walk with the largest
// continuation as the feasibility test.

// Assignment answers of the walk (chosenPairWalk's path entries).
const (
	chosenSkip = iota
	chosenPlain
	chosenPair
)

// chosenPairPrefix floats the mana p's activation a needs from its
// chosen-colour pair sources.
func (s *State) chosenPairPrefix(k *prep, a Action, p uint8) {
	cost, ex, _, ok := s.grantPlayCost(k, p, a, true)
	if !ok {
		return
	}
	d := s.grantPayDesc(k, a)
	d.noPair = true
	if s.affordsUngrantedPlay(k, p, a, &cost, ex, d) {
		return // the play prices at the search's root: no script
	}
	d.noPair = false
	if !s.affordsUngrantedPlay(k, p, a, &cost, ex, d) {
		return // not a pair's play (a grant or a script pays it)
	}
	var gaps slotMask
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		sl := b.first()
		c := s.cardAt(sl)
		if sl == ex || k.gen.flags[c]&gfRestrSrc == 0 || !s.chosenPairSrc(k, c) {
			continue
		}
		if pm := &s.perms[sl]; pm.flags&pfTapped != 0 || s.activationSick(k, sl) || s.genLost(k, sl) {
			continue
		}
		gaps = gaps.with(sl)
	}
	var order [MaxPerm]Slot
	n := s.battlefieldOrder(p, gaps, order[:], 0)
	if n == 0 {
		return
	}
	var path [MaxPerm]uint8
	var av avail
	var ms modalSet
	s.ungrAvail(k, p, ex, &av, &ms, slotMask{})
	s.chosenPairWalk(k, a, p, &cost, ex, d, order[:n], path[:], &av, &ms)
	for i, sl := range order[:n] {
		switch path[i] {
		case chosenPlain:
			s.chosenGapDo(k, p, sl, true) // the plain {T} floats into the pool
		case chosenPair:
			s.tapChosenPair(k, p, sl) // the pair floats restricted (State.rb)
		}
	}
	// the walk's largest continuation priced the play at its root (the entry
	// check), so the path prices it too; an odd yield can break that step,
	// and the pairs-only walk it replaced then pays the rest
	d.noPair = true
	for i := 0; i < n && !s.affordsUngrantedPlay(k, p, a, &cost, ex, d); i++ {
		if path[i] == chosenSkip {
			s.tapChosenPair(k, p, order[i])
		}
	}
}

// chosenPairWalk is the search's first-found path over the pair sources
// order: each source's plain answer, then its pair, committed only when the
// largest continuation from the next source still prices the play — gorge's
// lex order, whose first-found path the largest continuation makes greedy
// (a subtree holds a pricing node exactly when its first node's largest
// continuation does). path records each source's answer; av (ungrAvail's
// census, with ms) is left holding the committed path's.
func (s *State) chosenPairWalk(k *prep, a Action, p uint8, cost *kop.ManaCost, ex Slot, d payDesc, order []Slot, path []uint8, av *avail, ms *modalSet) {
	// The census is ungrAvail's: an untapped source's plain ability is still
	// a unit of it, and each answer taps its source, so it lifts that unit
	// out and adds the answer's mana in its place — the pair's two restricted
	// units for the pair, the plain unit back for the plain {T}.
	apply := func(t *avail, sl Slot, plain bool) {
		pc := s.manaPCGen(k, sl)
		t.remove(s.srcMaskAt(pc, sl), pc.manaN)
		if plain {
			t.addUnits(s.srcMaskAt(pc, sl), max(pc.manaN, 1))
			return
		}
		c := s.cardAt(sl)
		rec := k.gen.restrRecOf(c)
		ab := &k.t.Cards[c].Abilities[rec.ab]
		t.addUnits(1<<bits.TrailingZeros8(s.restrWideMask(&k.gen.rs.views[rec.view], sl)&0x1f), max(ab.ManaN, 1))
	}
	// largest prices the play at t with every remaining source at the pair:
	// a pair adds the most its answers can, so the subtree under a committed
	// answer holds a pricing node exactly when this does.
	largest := func(t *avail, from int) bool {
		u := *t
		for i := from; i < len(order); i++ {
			apply(&u, order[i], false)
		}
		return s.affordsUngrantedPlayAv(k, p, a, cost, ex, d, &u, ms)
	}
	d.noPair = true
	for i := 0; i < len(order); i++ {
		if s.affordsUngrantedPlayAv(k, p, a, cost, ex, d, av, ms) {
			return // the play prices here: the path so far is the script
		}
		pc := s.manaPCGen(k, order[i])
		if s.canTapForManaPC(k, order[i], pc) {
			u := *av
			apply(&u, order[i], true)
			if largest(&u, i+1) {
				*av, path[i] = u, chosenPlain
				continue
			}
		}
		u := *av
		apply(&u, order[i], false)
		if largest(&u, i+1) {
			*av, path[i] = u, chosenPair
			continue
		}
		// neither answer can still fund the play: the source is left out
	}
}

// tapChosenPair taps the chosen-colour pair source sl for its land-only
// ability: the restricted mana floats (State.rb).
func (s *State) tapChosenPair(k *prep, p uint8, sl Slot) {
	c := s.cardAt(sl)
	rec := k.gen.restrRecOf(c)
	ab := &k.t.Cards[c].Abilities[rec.ab]
	col := bits.TrailingZeros8(s.restrWideMask(&k.gen.rs.views[rec.view], sl) & 0x1f)
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	if !s.floatRestrOnly(k, p, sl, ab, col, max(ab.ManaN, 1)) {
		s.capFull(StallRestrSrc) // never a silent spill
	}
}

// chosenPairSrc: card c's restricted ability is a chosen-colour pair.
func (s *State) chosenPairSrc(k *prep, c CardID) bool {
	rec := k.gen.restrRecOf(c)
	return rec != nil && rec.kind == rkWiden && k.gen.rs.views[rec.view].wideChosen
}
