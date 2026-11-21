package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// An exile-from-graveyard mana source (Molt Tender: "{T}, Exile a card from
// your graveyard: Add one mana of any color", kop.ExileGManaAb) is one more
// source kind of the tap-another search (mana_tapanother.go). gorge pays the
// exile in its mana-activation walk (pay manacostwalk.go): the candidates are
// the payer's graveyard cards matching the spec, in zone order, less those a
// cast's cost picks reserved; a count short of N drops the activation, and
// the exile is asked as one "mana_exile" pick (AskManaExile), even with a
// single candidate. gorge's payment planner prices no Exile part, so the source
// is a census gap relaxed to one free unit of its production: the gaps walk
// counts it (addTapXUnits) and a play only its mana pays is found by the
// prefix search, which activates it, answers the pick in zone order (the cost
// is paid in gorge's commit order: the source's {T}, the exile, the mana) and
// asks the colour of an any-colour one. The manual cast / activation window
// lists it and poses the same pick (ChWinExile, mana_windowtapx.go); the
// unless window does not (it lists no exile cost either).
//
// A pick step's cand is the card's index in its owner's graveyard, not a
// battlefield slot (exileGCands), so the tap-another paths that read cand as a
// slot (the two-permanent, tap-own-source and grant paths) are never reached by
// this kind: it sits in scriptTapX like Bolg's Company and branches off before
// them.

// DebugExileGravePickLast picks the last graveyard candidate first where the
// search and the window answer the exile pick (mutation testing only: the
// lockstep must catch it).
var DebugExileGravePickLast bool

// ChWinExile is the manual window's exile pick of an exile-from-graveyard
// mana source: source = the graveyard card exiled (gorge's "mana_exile").
const ChWinExile ChoiceKind = 207

// exileGOf: card c's tap-another gap is an exile-from-graveyard source.
func (k *prep) exileGOf(c CardID) bool {
	a := k.tapXAbOf(c)
	return a != nil && kop.ExileGManaAb(a)
}

// pickXOf: card c's tap-another gap asks a permanent or card pick the search
// branches on in sacStep: a sacrifice-another or an exile-from-graveyard source.
func (k *prep) pickXOf(c CardID) bool { return k.sacXOf(c) || k.exileGOf(c) }

// exileGCands lists the graveyard cards exile source sl's cost can take, as
// indices into the controller's graveyard, in zone order (gorge's candidates:
// the payer's graveyard matching the spec, less the cast's reserved picks).
func (s *State) exileGCands(k *prep, p uint8, sl Slot, ab *kop.Ability, out *[MaxPerm]Slot) int {
	f := ab.Cost.Filter
	var fl kop.FilterFlags
	if f != 0 {
		fl = k.t.Filters[f].Flags
	}
	src := s.perms[sl].obj
	pl := &s.pl[SeatIx(p)]
	n := 0
	for i := 0; i < int(pl.graveN); i++ {
		o := pl.grave[i]
		if fl&kop.FSelf != 0 && o != src || fl&kop.FOther != 0 && o == src || s.picked(o) {
			continue
		}
		if f != 0 && !s.matchCardGen(k, f, s.objs[o].card, s.objs[o].owner(), p, o) {
			continue
		}
		out[n] = Slot(i)
		n++
	}
	return n
}

// exileGOptions adds the exile pick's options: the candidates in graveyard
// order.
func (s *State) exileGOptions(k *prep, p uint8, d *Decision) {
	var cs [MaxPerm]Slot
	n := s.exileGCands(k, p, s.g.winSrc, k.tapXAbOf(s.cardAt(s.g.winSrc)), &cs)
	for _, gi := range cs[:n] {
		d.add(MakeAction(AkChoose, TCode(s.pl[SeatIx(p)].grave[gi]), 0, 0))
	}
}

// graveIdx is the index of graveyard card o in its owner's graveyard.
func (s *State) graveIdx(o Obj) Slot {
	pl := &s.pl[s.objs[o].owner()]
	for i := 0; i < int(pl.graveN); i++ {
		if pl.grave[i] == o {
			return Slot(i)
		}
	}
	return NoSlot
}

// exileGDo activates exile source sl for p, exiling the graveyard card at index
// gi, in gorge's commit order (commitManaDiscard): the source's {T}, the exile,
// then the mana (the colour asked after the exile when it is any colour).
func (s *State) exileGDo(k *prep, p uint8, sl, gi Slot, colour uint8) {
	ab := k.tapXAbOf(s.cardAt(sl))
	if ab.Cost.Flags&kop.CostTap != 0 {
		s.perms[sl].flags |= pfTapped
		s.touchSl(sl)
		s.manaTapped(k, sl)
	}
	s.exileFromGrave(s.pl[SeatIx(p)].grave[gi])
	cols, n := tapXUnit(ab)
	if ab.ManaColors == kop.ColAny {
		cols = 1 << colour
	}
	s.poolAdd(p, bits.TrailingZeros8(cols), int(n))
	s.artTag(k, p, sl, bits.TrailingZeros8(cols))
	s.tapExtraStall(k, sl)
	s.sba(k)
}

// exileGStep is visit's branch for an exile source sl with nc candidates cs:
// the "mana_exile" ask is posed even with one candidate, in graveyard order,
// then the colour ask of an any-colour one (the announced colours first, as
// the tap-another cost's). stop: visit returns found.
func (rs *tapXSearch) exileGStep(k *prep, t *State, sl Slot, cs *[MaxPerm]Slot, nc int, cols *[5]uint8) (found, stop bool) {
	if !rs.tick() {
		return false, true
	}
	for ci := 0; ci < nc; ci++ {
		i := ci
		if DebugExileGravePickLast {
			i = nc - 1 - ci
		}
		if f, st := rs.colourStep(k, t, sl, cs[i], cols); st {
			return f, true
		}
	}
	return false, false
}

// exileGGap is the filter search's branch for an exile source (mixgaps tapXGap):
// the exile ask, then colours' answers for each candidate (cand is a graveyard
// index; the colour ask of an any-colour source is colours' own).
func (fs *filtSearch) exileGGap(nc int, cs *[MaxPerm]Slot, colours func(a, b Slot) bool) bool {
	if !fs.tick() {
		return false
	}
	for ci := 0; ci < nc; ci++ {
		i := ci
		if DebugExileGravePickLast {
			i = nc - 1 - ci
		}
		if colours(cs[i], NoSlot) {
			return true
		}
	}
	return false
}
