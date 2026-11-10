package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A fee-paid each-colour gap (Tarnation Vista): "{T}: Add one mana of the
// chosen colour" beside "{1}, {T}: For each colour among monocoloured
// permanents you control, add one mana of that colour".
//
// gorge's census leaves the second ability an unrelaxable gap (its Special
// production names no colour, so paymentPlanCensus.paid is empty and
// fees() == 0): the prefix search never funds the fee from a land. It
// activates the card (a gap) and the activation poses a choose of the card's
// mana abilities only when the pool already holds the fee -- otherwise the
// chosen {T} is the one ability the activation has, and it taps for its colour
// with no ask. The ask lists the chosen {T} first, then the fee-paid ability;
// the fee is paid at once from the pool, generic in C W U B R G order (after
// the restricted mana it may spend), and the colours follow.
//
// eachSearch is scriptSearch.visit over the board's each-colour gaps alone:
// depth first in battlefield order, the chosen {T} before the fee-paid ability,
// each distinct payer state once, and the same node accounting (the priority
// node, and the ask's own node when it is posed). A board with another gap kind
// (a filter, a restricted source) keeps its own arm, which taps this card with
// tapGap's default (the fee-paid ability when the pool holds the fee).

// maxEachGaps bounds the gaps one search orders; a board with more leaves the
// later ones untapped.
const maxEachGaps = 8

// eachFee is the generic mana sl's each-colour ability costs: a Tarnation
// Vista's {1}, 0 for a Bloom Tender's free {T}.
func (s *State) eachFee(k *prep, sl Slot) int {
	c := s.cardAt(sl)
	return k.t.Cards[c].Abilities[k.gen.eachCol(c)].Cost.Mana.MV()
}

// eachChosenMask is the colours sl's chosen-colour {T} makes: none for a free
// gap (it has no such ability) and none while the card has chosen nothing.
func (s *State) eachChosenMask(k *prep, sl Slot) uint8 {
	pc := s.manaPCGen(k, sl)
	if !pc.manaChosen {
		return 0
	}
	return s.srcMaskAt(pc, sl) & 0x1f
}

// eachFeePayable reports whether p's floating mana, the restricted batches the
// fee may spend among it, holds sl's fee: the one condition under which the
// activation offers the fee-paid ability.
func (s *State) eachFeePayable(k *prep, p uint8, sl Slot, fee int) bool {
	return poolTotal(&s.pl[p&1])+s.filtFeeRestrN(k, p, sl) >= fee
}

// eachGapUnits adds the units gap sl's tap can add to a gaps walk: its colours,
// but not a fee-paid one's when neither the pool nor another gap's tap can
// fund its fee (the walk has no way to take a unit out, so a funded fee is
// not netted: eachSearch decides the plays the walk lets through).
func (s *State) eachGapUnits(k *prep, p uint8, sl Slot, a *avail) {
	if fee := s.eachFee(k, sl); fee > 0 && !s.eachFeePayable(k, p, sl, fee) && !s.scriptGaps(k, p).andNot(slotBit(sl)).any() {
		return
	}
	out := s.eachColorOut(k, p, sl)
	for c := 0; c < 5; c++ {
		if out&(1<<c) != 0 {
			a.addUnits(1<<c, 1)
		}
	}
}

// eachFeeBoard reports whether p has a live fee-paid each-colour gap.
func (s *State) eachFeeBoard(k *prep, p uint8) bool {
	if !k.gen.anyEachCol {
		return false
	}
	for b := s.scriptGaps(k, p); b.any(); b = b.rest() {
		if s.eachFee(k, b.first()) > 0 {
			return true
		}
	}
	return false
}

// eachProduce is the mana of gap sl's tap (sl already tapped): a free gap's
// colours; a fee-paid one's chosen colour (ans 0) or, its fee paid from the
// pool, its each-colour set (ans 1).
func (s *State) eachProduce(k *prep, p uint8, sl Slot, ans uint8) {
	pl := &s.pl[p&1]
	if fee := s.eachFee(k, sl); fee > 0 {
		if ans == 0 {
			pc := s.manaPCGen(k, sl)
			if m := s.eachChosenMask(k, sl); m != 0 {
				c := bits.TrailingZeros8(m)
				s.poolAdd(p, c, int(max(pc.manaN, 1)))
				s.addExtra(k, p, sl, pc)
				s.artTag(k, p, sl, c)
			}
			return
		}
		fee -= s.filtFeeRestr(k, p, sl, fee)
		for _, c := range [6]int{kop.C, kop.W, kop.U, kop.B, kop.R, kop.G} {
			for fee > 0 && pl.pool[c] > 0 {
				pl.pool[c]--
				fee--
			}
		}
		if k.artMana {
			s.artSync(p) // a slot the fee emptied no longer holds an artifact's mana
		}
	}
	out := s.eachColorOut(k, p, sl)
	for c := 0; c < 5; c++ {
		if out&(1<<c) != 0 {
			pl.pool[c]++
		}
	}
}

// eachDefaultAns is the ability tapGap takes for gap sl: the fee-paid one when
// the pool holds its fee, else the chosen {T}.
func (s *State) eachDefaultAns(k *prep, p uint8, sl Slot) uint8 {
	if fee := s.eachFee(k, sl); fee > 0 && s.eachFeePayable(k, p, sl, fee) {
		return 1
	}
	return 0
}

type eachKey struct {
	pool   [kop.NumManaKinds]uint8
	tapped uint32 // bit i: the search's i-th gap is tapped
}

type eachStep struct {
	sl  Slot
	ans uint8 // 0: the chosen {T} (a free gap's colours), 1: the fee-paid ability
}

// eachSearch is gorge's prefix search over the board's each-colour gaps.
type eachSearch struct {
	a     Action
	p     uint8
	own   Slot // the source whose {T} is the play's own cost (NoSlot: none)
	order [MaxPerm]Slot
	n     int
	nodes int
	ns    int
	seen  [potentialScriptNodes]eachKey
	path  [maxEachGaps]eachStep
	np    int
}

// tick counts one visited node; false once the budget is spent.
func (fs *eachSearch) tick() bool {
	if fs.nodes >= potentialScriptNodes {
		return false
	}
	fs.nodes++
	return true
}

// visit is scriptSearch.visit at payer priority node t.
func (fs *eachSearch) visit(k *prep, t *State, d *Decision) bool {
	if !fs.tick() {
		return false
	}
	t.restrOptions(k, d, fs.p)
	if hasOption(d, fs.a) {
		return true
	}
	key := eachKey{pool: t.pl[fs.p&1].pool}
	for i := 0; i < fs.n; i++ {
		if t.perms[fs.order[i]].flags&pfTapped != 0 {
			key.tapped |= 1 << i
		}
	}
	for i := 0; i < fs.ns; i++ {
		if fs.seen[i] == key {
			return false
		}
	}
	if fs.ns < len(fs.seen) {
		fs.seen[fs.ns] = key
		fs.ns++
	}
	for i := 0; i < fs.n; i++ {
		sl := fs.order[i]
		if t.perms[sl].flags&pfTapped != 0 || sl == fs.own {
			continue
		}
		fee := t.eachFee(k, sl)
		if fee == 0 {
			if fs.try(k, t, i, 0, d) {
				return true
			}
			continue
		}
		chosen, paid := t.eachChosenMask(k, sl) != 0, t.eachFeePayable(k, fs.p, sl, fee)
		switch {
		case chosen && paid:
			// the activation asks which ability; the chosen {T} is listed first
			if !fs.tick() {
				return false
			}
			if fs.try(k, t, i, 0, d) || fs.try(k, t, i, 1, d) {
				return true
			}
		case chosen:
			if fs.try(k, t, i, 0, d) {
				return true
			}
		case paid:
			if fs.try(k, t, i, 1, d) {
				return true
			}
		}
	}
	return false
}

// try taps the search's i-th gap by ability ans on a copy of t and continues
// the search there.
func (fs *eachSearch) try(k *prep, t *State, i int, ans uint8, d *Decision) bool {
	c := t.fork()
	sl := fs.order[i]
	c.perms[sl].flags |= pfTapped
	c.touchSl(sl)
	c.manaTapped(k, sl)
	c.eachProduce(k, fs.p, sl, ans)
	fs.path[fs.np] = eachStep{sl, ans}
	fs.np++
	found := fs.visit(k, c, d)
	if !found {
		fs.np--
	}
	t.unfork()
	return found
}

// eachScript reports whether gorge's prefix search over p's each-colour gaps
// finds a script for play a; with apply it plays the script on s (the play's
// prefix).
//
//go:noinline
func (s *State) eachScript(k *prep, a Action, p uint8, apply bool) bool {
	var fs eachSearch
	fs.a, fs.p, fs.own = a, p, NoSlot
	if a.Kind() == AkActivate {
		_, fs.own, _ = s.activationMana(k, Obj(a.Source()), a.Mode())
	}
	fs.n = min(s.battlefieldOrder(p, s.scriptGaps(k, p), fs.order[:], 0), maxEachGaps)
	var d Decision
	if !fs.visit(k, s, &d) {
		return false
	}
	if apply {
		for _, st := range fs.path[:fs.np] {
			s.perms[st.sl].flags |= pfTapped
			s.touchSl(st.sl)
			s.manaTapped(k, st.sl)
			s.eachProduce(k, p, st.sl, st.ans)
		}
	}
	return true
}
