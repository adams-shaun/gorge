package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Deferred mana sources (Pyromancer's Goggles, kop.Ability.ManaDeferred:
// "{T}: Add {R}. When that mana is spent to cast a red instant or sorcery
// spell, copy that spell ..."). gorge's payment planner defers a mana
// ability carrying TriggersWhenSpent$ (pay.PlanHasSpecialProductionParam), so
// its census never lists it: autopay never taps it (pcard.manaAb stays -1)
// and only a potential play's prefix script, or the manual mana window, does.
// The census has a gap there, relaxable (gorge paymentPlanRelaxedAlternatives
// gives the source a free {R} alternative), so the kernel treats it as a
// scripted-prefix gap of the restricted kind (scriptRestr returns it with
// them: gapMask gfRestrScript|gfDeferred) whose mana floats in the plain pool
// and not in State.rb.
//
// The mana is an artifact's, typed (artTag). A cast that spends the floating
// {R} is paid by the deferred source (gorge ManaSpentSources): the rider reads
// that through the observed-source record of the cast's build (obsSl,
// Tf2ManaFromSelf; mana_spendtrigger.go). genState.defSrc holds the source's
// slot + 1 while its {R} floats; pay compares the pool's {R} across the payment.

// deferCol is the colour (a kop.W..kop.G index) of card c's deferred mana
// ability: Pyromancer's Goggles' {R}, Gilanra's {G}. Every "{R}" above is this.
func (k *prep) deferCol(c CardID) int {
	for i := range k.t.Cards[c].Abilities {
		if a := &k.t.Cards[c].Abilities[i]; a.Kind == kop.AbMana && a.ManaDeferred {
			return bits.TrailingZeros8(uint8(a.ManaColors))
		}
	}
	return kop.R
}

// floatDeferred adds the deferred source sl's mana to p's pool and remembers
// the source and its colour (tapGap's deferred arm). A second deferred source
// of another colour floating beside the first would leave the first's spend
// unattributed: stall loudly.
func (s *State) floatDeferred(k *prep, p uint8, sl Slot) {
	col := k.deferCol(s.cardAt(sl))
	if d := s.g.defSrc[SeatIx(p)]; d != 0 && d != uint8(sl)+1 && int(s.g.defCol[SeatIx(p)]) != col && s.pl[SeatIx(p)].pool[s.g.defCol[SeatIx(p)]] != 0 {
		s.capFull(StallObsSrc)
	}
	s.pl[SeatIx(p)].pool[col]++
	s.artTag(k, p, sl, col)
	s.g.defSrc[SeatIx(p)], s.g.defCol[SeatIx(p)] = uint8(sl)+1, uint8(col)
}

// deferredPool is p's floating deferred mana before a payment, 0 when none,
// and the source's slot + 1 (the payment's pool spend forgets it).
func (s *State) deferredPool(k *prep, p uint8) (r uint8, src uint8) {
	if k.gen == nil || !k.gen.anyDeferred || s.g.defSrc[SeatIx(p)] == 0 {
		return 0, 0
	}
	return s.pl[SeatIx(p)].pool[s.g.defCol[SeatIx(p)]], s.g.defSrc[SeatIx(p)]
}

// deferredSpent records the deferred source on the cast being built when the
// payment spent some of its {R} (r0 was the pool's before).
//
//go:noinline
func (s *State) deferredSpent(k *prep, p uint8, r0, src uint8) {
	if s.pl[SeatIx(p)].pool[s.g.defCol[SeatIx(p)]] >= r0 {
		return
	}
	// gorge spends the source's batch first, so its unit is gone now even when
	// same-colour mana from other sources still floats (Gilanra's {G} beside
	// Forests'): a later payment of that colour is not the source's
	s.g.defSrc[SeatIx(p)], s.g.defCol[SeatIx(p)] = 0, 0
	if !s.buildLive() || s.bld().e.kind != seSpell {
		return
	}
	sl := Slot(src - 1)
	if s.bld().obsSl != 0 && s.bld().obsSl != sl+1 {
		s.capFull(StallObsSrc)
		return
	}
	s.bld().obsSl = sl + 1
}

// scriptDeferred: p's activatable deferred sources, a slot mask (a subset of
// scriptRestr's).
func (s *State) scriptDeferred(k *prep, p uint8) slotMask {
	if k.gen == nil || !k.gen.anyDeferred {
		return slotMask{}
	}
	return s.gapMask(k, p, gfDeferred)
}

// deferRider: a is the TriggersWhenSpent$ rider of the deferred source at sl,
// which onCast's listener walk skips. gorge queues the rider apart from the
// ordinary cast triggers (cast_commit.go fireManaSpentTriggers), after them and
// casualty, before cascade, so its place in a same-controller trigger-order
// ask is last.
func (s *State) deferRider(k *prep, sl Slot, a *kop.Ability) bool {
	return k.gen.anyDeferred && a.Trig.Flags2&kop.Tf2ManaFromSelf != 0 && k.gen.has(s.cardAt(sl), gfDeferred)
}

// fireDeferredRider queues the rider of the deferred source the cast's payment
// spent (build.obsSl), when it is still on the battlefield; castTrigGen gates
// the spell's colour and type and the payment (castProvOK).
//
//go:noinline
func (s *State) fireDeferredRider(k *prep, e *stackEntry) {
	if !s.buildLive() || s.bld().obsSl == 0 {
		return
	}
	sl := Slot(s.bld().obsSl - 1)
	if !s.occ.has(sl) || !k.gen.has(s.cardAt(sl), gfDeferred) {
		return
	}
	s.slotTriggers(k, sl, func(a *kop.Ability) bool { return a.Trig.Event == kop.TrigSpellCast && s.deferRider(k, sl, a) }, func(l trigListener, a *kop.Ability) {
		s.castTrigGen(k, l, a, e)
	})
}
