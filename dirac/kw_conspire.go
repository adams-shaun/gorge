package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Conspire as a granted keyword (CR 702.78a): Raiding Schemes' AffectedZone$
// Stack grant (StatStackKw, StackKwConspire), copied from gorge (rules
// legal_walk_hand.go's "conspired" offer, cast_asks.go conspireAsk,
// trigger_granted.go checkGrantedConspireTriggers). The printed keyword has
// no form in the kernel (kgen refuses K:Conspire).
//
//   - the conspired cast is its own AkCastConspire option beside the plain
//     cast (a granted conspire pairs with a printed casualty, so AkCastCasualty
//     cannot carry it). It costs no extra mana and is offered when the plain
//     cast was, the grant reaches the spell, and at least two untapped
//     creatures the caster controls share a colour with it.
//   - bsConspire poses the tap election after the tap-cost asks and before
//     convoke (gorge: conspireAsk follows them): exactly two eligible creatures
//     are forced (nothing posed); more are a two-pick ChGenConspire over the
//     eligible set; fewer degrade the cast to the plain one. The picks are
//     pkTapX taps, settled at payment.
//   - one trigger is queued with a granted spell's PAID cast only (gorge's
//     synthesized trigger carries the printed path's CheckSVar$ Count$Conspired
//     intervening "if", CR 702.78a/603.4): paid, it is the abDelayed
//     OpCopySpell entry casualty uses, whose controller may choose new targets
//     (CR 707.10c); an unpaid cast queues no trigger at all, so nothing
//     occupies the stack.
const (
	casConspire  uint8 = 4 // build.cas: the conspired cast, election not begun
	casConspHalf uint8 = 5 // one creature of the two picked
	casConspPaid uint8 = 6 // both tapped
)

// DebugConspireNoCopy is the mutation hook: the paid election queues no copy
// trigger, so the lockstep must diverge.
var DebugConspireNoCopy bool

// conspireCand: slot sl is an untapped creature of p's sharing a colour with
// the mask want (gorge conspireCandidates).
func (s *State) conspireCand(k *prep, sl Slot, want kop.ColorMask) bool {
	return s.perms[sl].flags&pfTapped == 0 && s.isCreature(k, sl) && s.colorsAt(k, sl)&want&kop.ColAny != 0
}

// conspireOffer: the conspired cast of p's spell card c (object o) is
// available: the grant reaches it and two creatures could be tapped.
func (s *State) conspireOffer(k *prep, p uint8, c CardID, o Obj) bool {
	if !s.grantConspire(k, p, c, o, true) { // the offer: the card is in the hand
		return false
	}
	want, n := k.t.Cards[c].Colors, 0
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		if s.conspireCand(k, m.first(), want) {
			n++
		}
	}
	return n >= 2
}

// beginConspireCast begins the conspired cast of o.
func (s *State) beginConspireCast(k *prep, o Obj) {
	s.beginCast(k, o, 0, 0)
	s.bld().cas = casConspire
}

// conspireOptions poses the next creature of the tap election; false = the
// election is settled (or degraded) and the build moves on.
func (s *State) conspireOptions(k *prep, d *Decision) bool {
	b := s.bld()
	e := &b.e
	want := k.t.Cards[e.card].Colors
	d.N = 0
	n := 0
	for m := s.ctrlM[e.ctrl]; m.any(); m = m.rest() {
		sl := m.first()
		if !s.picked(s.perms[sl].obj) && s.conspireCand(k, sl, want) {
			d.add(MakeAction(AkChoose, uint8(s.perms[sl].obj), 0, 0))
			n++
		}
	}
	if b.cas == casConspire {
		switch {
		case n < 2:
			d.N = 0
			b.cas = 0 // gorge: "conspire no longer payable; casting without conspire"
			return false
		case n == 2:
			for i := 0; i < int(d.N); i++ {
				b.picks[b.pickN] = Obj(d.Options[i].Source())
				b.pickKs[b.pickN] = pkTapX
				b.pickN++
			}
			d.N = 0
			b.cas = casConspPaid
			s.syncBuild()
			return false
		}
	}
	d.Kind, d.Sub, d.Player = DkChoose, ChGenConspire, e.ctrl
	return true
}

// answerConspire records a tapped creature; the second one closes the election.
func (s *State) answerConspire(k *prep, a Action) {
	b := s.bld()
	b.picks[b.pickN] = Obj(a.Source())
	b.pickKs[b.pickN] = pkTapX
	b.pickN++
	if b.cas == casConspire {
		b.cas = casConspHalf
		s.syncBuild()
		return
	}
	b.cas = casConspPaid
	s.nextStage(k)
	s.syncBuild()
}

// conspireTrigger queues the paid cast's granted-conspire trigger, one for all
// granting statics (gorge checkGrantedConspireTriggers). The synthesized
// trigger carries the printed expansion's CheckSVar$ Count$Conspired
// intervening "if" (CR 702.78a, 603.4): an unpaid cast puts nothing on the
// stack.
func (s *State) conspireTrigger(k *prep, e *stackEntry) {
	if k.gen == nil || !k.gen.anyConspire {
		return
	}
	n, _ := s.stackKwN(k, kop.StackKwConspire, e.card, e.obj, e.ctrl, s.castFromHandBy(e, e.ctrl))
	if n == 0 || s.bld().cas != casConspPaid {
		return
	}
	op := kop.OpCopySpell
	if DebugConspireNoCopy {
		op = kop.OpNop
	}
	s.enqueue(e.obj, s.objs[e.obj].ver, e.card, abDelayed, e.ctrl, e.obj, e.ctrl, int16(op))
}
