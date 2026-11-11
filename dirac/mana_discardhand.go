package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Lion's Eye Diamond: "Discard your hand, Sacrifice this: Add three mana of
// any one color. Activate only as an instant." (kop.DiscardHandManaAb). It is
// a mana ability (CR 605.1a) that is activated only when its controller could
// cast an instant, so never while a spell or ability is being paid for (CR
// 605.4; gorge pay.InstantSpeedOnly keeps it out of every payment window and
// of the planner census). gorge's potential-play census lists it as a gap, its
// relaxed alternative three free units of any colour (paymentPlanRelaxed-
// Alternatives: the sacrifice makes it once-only), and a play the relaxed proof
// pays gets a prefix search over the gaps alone with no funding (the ability
// has no mana fee): activate a Diamond, answer its colour ask (the play's
// announced colours first, then W U B R G), and stop at the first payer
// priority where the play is offered or the planner pays it from the pool. A
// play from the hand never completes: the cost discards it.
//
// Modelled only when the Diamonds are p's only census gaps (dhBoard); a table
// holding a Diamond beside another gap kind is refused at its start
// (StallDiamondMix, mana_discardhand_mix.go).

// maxDHGaps bounds the Diamonds one search walks.
const maxDHGaps = 4

// dhGaps: p's Diamonds whose ability can be activated now. The cost taps
// nothing, so a tapped Diamond activates too (gorge gates only tap costs on
// the source's tap state).
func (s *State) dhGaps(k *prep, p uint8) slotMask {
	var m slotMask
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		sl := b.first()
		if !k.gen.has(s.cardAt(sl), gfDiscHand) || s.genLost(k, sl) ||
			k.gen.anyCantAct && s.actForbidden(k, sl, true) {
			continue
		}
		m = m.with(sl)
	}
	return m
}

// dhBoard reports whether p controls a live Diamond and no census gap of
// another kind.
func (s *State) dhBoard(k *prep, p uint8) bool {
	dh := s.dhGaps(k, p)
	if !dh.any() {
		return false
	}
	if s.scriptGaps(k, p).or(s.scriptRestr(k, p)).or(s.scriptAnyR(k, p)).or(s.scriptTapsDefer(k, p)).or(s.scriptCtr(k, p)).or(s.scriptFilt(k, p)).any() {
		return false
	}
	if k.tapMana && s.scriptTapX(k, p).any() || s.liveGrantGap(k, p) || s.grantGapLive(k, p) {
		return false
	}
	return !s.gapOthers(k, p).andNot(dh).any()
}

// availDH adds each live Diamond's relaxed units: three of any colour.
func (s *State) availDH(k *prep, p uint8, a *avail) {
	for b := s.dhGaps(k, p); b.any(); b = b.rest() {
		a.addUnits(0x1f, s.dhAmount(k, b.first()))
	}
}

// dhAmount is the mana the Diamond at sl adds.
func (s *State) dhAmount(k *prep, sl Slot) uint8 {
	cd := &k.t.Cards[s.cardAt(sl)]
	for j := range cd.Abilities {
		if a := &cd.Abilities[j]; kop.DiscardHandManaAb(a) {
			return max(a.ManaN, 1)
		}
	}
	return 1
}

// dhActivate activates the Diamond at sl for colour col: p's whole hand is
// discarded in hand order, then the Diamond is sacrificed (gorge pays the
// discard before the sacrifice), then the mana is added.
func (s *State) dhActivate(k *prep, p uint8, sl Slot, col int) {
	n := int(s.dhAmount(k, sl))
	s.payDiscardHandCost(k, SeatIx(p))
	s.artTag(k, p, sl, col)
	s.sacFilterSource(k, sl)
	s.poolAdd(p, col, n)
}

type dhStep struct {
	sl  Slot
	col uint8
}

// dhSearch is gorge's prefix search over p's Diamonds.
type dhSearch struct {
	a     Action
	p     uint8
	cols  [5]uint8 // the colour ask's answers in search order (needFirst)
	nodes int
	path  [maxDHGaps]dhStep
	np    int
}

// visit is scriptSearch.visit at payer priority node t (root: no gap used yet,
// so the play is not priced there).
func (ds *dhSearch) visit(k *prep, t *State, d *Decision, root bool) bool {
	if ds.nodes >= potentialScriptNodes {
		return false
	}
	ds.nodes++
	if !root {
		t.restrOptions(k, d, ds.p)
		if hasOption(d, ds.a) {
			return true
		}
	}
	if ds.np == maxDHGaps {
		return false
	}
	var order [MaxPerm]Slot
	n := t.battlefieldOrder(ds.p, t.dhGaps(k, ds.p), order[:], 0)
	for i := 0; i < n; i++ {
		if ds.nodes >= potentialScriptNodes {
			return false
		}
		ds.nodes++ // the colour ask's node
		for _, col := range ds.cols {
			c := t.fork()
			c.dhActivate(k, ds.p, order[i], int(col))
			ds.path[ds.np] = dhStep{order[i], col}
			ds.np++
			found := ds.visit(k, c, d, false)
			t.unfork()
			if found {
				return true
			}
			ds.np--
		}
	}
	return false
}

// dhScript reports whether gorge's prefix search over p's Diamonds finds a
// script for play a; with apply it plays the script on s (the play's prefix).
func (s *State) dhScript(k *prep, a Action, p uint8, apply bool) bool {
	ds := dhSearch{a: a, p: p, cols: [5]uint8{kop.W, kop.U, kop.B, kop.R, kop.G}}
	needFirst(ds.cols[:], s.playNeed(k, a))
	var d Decision
	if !ds.visit(k, s, &d, true) {
		return false
	}
	if apply {
		for _, st := range ds.path[:ds.np] {
			s.dhActivate(k, p, st.sl, int(st.col))
		}
	}
	return true
}
