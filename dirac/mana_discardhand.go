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
// (StallDiamondMix, mana_diamondmix.go).

// maxDHGaps bounds the Diamonds one search walks.
const maxDHGaps = 4

// dhGaps: p's Diamonds whose ability can be activated now. The cost taps
// nothing, so a tapped Diamond activates too (gorge gates only tap costs on
// the source's tap state).
func (s *State) dhGaps(k *prep, p uint8) slotMask {
	var m slotMask
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		sl := b.first()
		if s.pickedForMana(s.perms[sl].obj) {
			continue // a Convoke/Waterbend pick is tapped for that cast, not for mana
		}
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

// ungrAvailDH is ungrAvail without the Diamond gaps: the planner's census
// prices no Diamond (its activation carries the discard-hand part the planner
// cannot execute, so gorge's census leaves it a gap), and a hand play whose
// price the rest of the census cannot pay is no plan of gorge's planner, whose
// prefix search over the Diamonds never completes for one -- the activation's
// cost discards the play's own hand card. gorge's verdict for it is
// census_incomplete with no script (potential_plan.go), so the autopay surface
// drops it.
func (s *State) ungrAvailDH(k *prep, p uint8, ex Slot, av *avail, ms *modalSet) {
	s.ungrAvail(k, p, ex, av, ms, s.dhGaps(k, p))
}

// dhWithholdsCast reports whether the hand spell o's cast, priced at cost, is
// payable only through the board's Diamonds: gorge's planner census holds no
// Diamond unit and its prefix search never funds a hand play, so the planner
// prices no plan and the surface drops the cast. A cast whose origin is not
// the hand (a commander-zone or may-play one) survives the discard, so its
// script can fund it and the census read stands; beside a gap of another kind
// (dhBoard false) the prefix search runs its own route, so nothing is
// withheld here either.
func (s *State) dhWithholdsCast(k *prep, p uint8, o Obj, cost *kop.ManaCost) bool {
	if k.gen == nil || !k.gen.anyDiscHand || s.objs[o].zone != kop.ZoneHand || !s.dhBoard(k, p) {
		return false
	}
	var av avail
	var ms modalSet
	s.ungrAvailDH(k, p, NoSlot, &av, &ms)
	d := payDesc{class: pdSpell, card: s.objs[o].card, src: NoSlot, og: s.restrOriginOfObj(o)}
	return !s.affordsUngrantedAv(k, p, cost, NoSlot, d, &av, &ms)
}

// ungrantedPaysNoDH reports whether the planner's census (no granted source)
// pays the hand spell o's cast at cost and the Diamonds are not the only
// thing that does (dhWithholdsCast).
func (s *State) ungrantedPaysNoDH(k *prep, p uint8, c CardID, o Obj, cost *kop.ManaCost) bool {
	return s.affordsUngrantedFor(k, p, cost, NoSlot, payDesc{class: pdSpell, card: c, src: NoSlot, og: s.restrOriginOfObj(o)}) &&
		!s.dhWithholdsCast(k, p, o, cost)
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
	// stall: the first stall a failed line's fork latched (stallOf), StallNone
	// none; dhScript declares it when the whole search fails
	stall uint8
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
			if !found {
				ds.stallOf(c) // the line failed; the fork may have latched a stall
			}
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
		if ds.stall != 0 {
			// every line failed and one ran into a rule gap the kernel stalls
			// by name: declare it, not "no payment" (tapXScript's stall; a
			// found path is not this, its apply replays it on s)
			s.capFull(ds.stall)
		}
		return false
	}
	if apply {
		for _, st := range ds.path[:ds.np] {
			s.dhActivate(k, p, st.sl, int(st.col))
		}
	}
	return true
}

// stallOf records the stall a failed line's fork latched (capFull) while the
// line ran to it, the first one, so dhScript declares it on the real state
// instead of dropping the play as unpayable: the fork cannot go on (its next
// stepGen ends the game), but the play could have been payable along it.
func (ds *dhSearch) stallOf(c *State) {
	if ds.stall == 0 {
		ds.stall = c.g.stalled
	}
}
