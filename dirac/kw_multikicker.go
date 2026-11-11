package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Multikicker (CR 702.43, Forge's K:Multikicker:<cost>): "you may pay [cost]
// any number of times as you cast this spell". gorge offers the multikicked
// cast as its own priority option paying the base plus ONE payment
// (legal_walk_hand.go) and then asks the count 0..max before Convoke/X and the
// targets (cast_asks.go multikickAsk); the answered count folds that many
// payments into the total cost and rides the pay-time CastInfo's FlagMultikicked
// Amount into Object.TimesKicked, which Count$TimesKicked reads on the spell
// and on the permanent it becomes (events/apply_cast.go, effects/count_body_paid.go).
//
// The kernel mirrors that shape:
//   - the card's kicked cast (AkCast mode bit 0, sfKicked) is offered when the
//     base plus one payment is affordable (legal.go), and its build cost adds
//     the chosen count's payments (cast.go buildCost);
//   - the count ask is a DkChoose/ChMultikick stage after the tap costs and
//     before Convoke/X (buildProgram's bsMulti), bounded by what the payment
//     window can still make (multikickMax);
//   - a zero answer clears sfKicked: an exactly plain cast, as gorge's
//     modeFlags maps a count-0 multikicked mode to "";
//   - the count lives in cold.mkObj[obj] while the spell is on the stack and
//     moves to cold.mk[slot] as the permanent enters, where
//     Count$TimesKicked (kop.ExKicked N 1) reads it (exprs.go timesKicked).

// maxMultikick bounds the count ask, gorge multikickAsk's own 64-iteration
// cap. A real board's mana runs out first.
const maxMultikick = 64

// multikickCount is the multikicker payment count of the spell e (0 for a
// permanent or a table without Multikicker).
func (s *State) multikickCount(e *stackEntry) int {
	if e == nil || int(e.obj) >= len(s.cold.mkObj) {
		return 0
	}
	return int(s.cold.mkObj[e.obj])
}

// setMultikick records e's payment count.
func (s *State) setMultikick(e *stackEntry, n int) {
	if int(e.obj) < len(s.cold.mkObj) {
		s.cold.mkObj[e.obj] = uint8(n)
	}
}

// kickerPayment is the per-payment cost a kicked cast adds: Multikicker for a
// Multikicker card, else the plain Kicker (an Offspring or OptWaterbend card
// keeps its cost in Kicker too).
func kickerPayment(cd *kop.CardDef) kop.ManaCost {
	if cd.HasMultikicker {
		return cd.Multikicker
	}
	return cd.Kicker
}

// multikickMax is the largest multikicker count gorge multikickAsk would
// offer: base + N payments walked through the same affordability check the
// payment uses, bounded by the floating pool for an {X}-less spell (the
// oracle lowering has already tapped base + one payment into it) and by the
// window for an {X} spell. The build cost is recomputed at each count so a
// reduction applies to the total exactly as at payment.
func (s *State) multikickMax(k *prep) int {
	e := &s.bld().e
	cd := &k.t.Cards[e.card]
	if e.flags&sfKicked == 0 || !cd.HasMultikicker {
		return 0
	}
	var av avail
	fromHand := s.castFromHandBy(e, e.ctrl)
	fromSources := cd.Cost.X != 0
	if fromSources {
		// An {X} multikicked cast's oracle witness taps for the X too, so its
		// pool already holds what the sources can make; the count ask reads
		// the same window the X ask does.
		var fav avail
		s.availXWindow(k, e.ctrl, &av)
		av = *s.castGrantAvail(k, e.ctrl, e.obj, &av, &fav)
	} else {
		// gorge multikickAsk bounds a non-X count by the FLOATING POOL at ask
		// time (its "pool-only at ask time"): the oracle's lowering witness
		// has tapped base + one payment into it. Untapped sources do not
		// count, exactly as an {X}-less cast's pool-only bound.
		pl := &s.pl[e.ctrl]
		for c := 0; c < kop.NumManaKinds; c++ {
			if n := pl.pool[c]; n > 0 {
				av.addUnits(1<<uint(c), n)
			}
		}
		// a twobrid multikicker cost takes the delve credit per face (the
		// affordsTb leaf, gorge FeasibleAny's delve argument); the plain
		// generic credit is gateDelveCredit's below.
		av.delve = s.tbDelveCredit(k, e.ctrl, e.card, e.obj, fromHand)
	}
	save := s.multikickCount(e)
	max := 0
	for n := 1; n <= maxMultikick; n++ {
		s.setMultikick(e, n)
		cost := s.buildCost(k)
		// gorge countCandPayable takes the cast's delve credit before the
		// mana gate, so a granted Delve (Teval) raises the payable count;
		// the delve picks are announced after this ask, so the bound reads
		// the whole graveyard, as gorge's does. K'rrik's PayLifeInsteadOf:B
		// grant is no dirac static (kgen refuses it), so no life credit is
		// folded here.
		cost = s.gateDelveCredit(k, e.ctrl, e.card, e.obj, fromHand, cost)
		if !av.affords(&cost, pipMaskOf(&cost), 0) {
			break
		}
		max = n
	}
	s.setMultikick(e, save)
	return max
}

// multikickOptions poses the count ask, 0..max. false: not a multikicked
// cast, or nothing is payable (the explicitly chosen mode degrades to the
// plain cast, gorge multikickAsk's max==0 arm).
func (s *State) multikickOptions(k *prep, d *Decision) bool {
	e := &s.bld().e
	if e.kind != seSpell || !k.t.Cards[e.card].HasMultikicker || e.flags&sfKicked == 0 {
		return false
	}
	max := s.multikickMax(k)
	if max == 0 {
		e.flags &^= sfKicked
		return false
	}
	d.Kind, d.Sub, d.Player = DkChoose, ChMultikick, e.ctrl
	for n := 0; n <= max; n++ {
		d.add(MakeAction(AkChoose, 0, 0, uint8(n)))
	}
	return true
}

// answerMultikick settles the chosen count. Zero declines the optional cost
// and leaves an exactly plain cast; a positive count rides the spell object.
func (s *State) answerMultikick(k *prep, a Action) {
	e := &s.bld().e
	if n := int(a.Mode()); n == 0 {
		e.flags &^= sfKicked
		s.setMultikick(e, 0)
	} else {
		s.setMultikick(e, n)
	}
	s.nextStage(k)
}
