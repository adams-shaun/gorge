package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// "whenever you cast a spell using mana produced by
// CARDNAME" (Barracks of the Thousand; gorge ValidSA$ Spell.ManaFromCard.
// StrictlySelf). gorge keeps the producing source of every unit an
// observing source (its face reads the identity) adds as a source-bearing
// pool batch, consumes those batches first when a cast pays, and records
// the consumed batches' sources in Object.ManaSpentSources
// (engine/rules/cast_mana_sources.go). The kernel pays a cast by tapping the
// planned sources into the pool and spending the pool at once, so the
// observed source's batch is spent exactly when this cast's payment tapped
// the source: the planner's tap sites (mana.go plan, mana_window.go windowTap)
// record it (noteTapSrc), and the SpellCast match reads the record
// (castProvOK) while the cast's build is still live.
// Guards: genPrep.castTapOn (some card carries the reader).

// pcard.castSrc bits.
const (
	csTreasure uint8 = 1 << iota
	csObserved
	csManaHaste // a ManaHasteValid rider's source (mana_spentrider.go)
)

// prepManaSrc sets the reader's per-card marks and genPrep.castTapOn.
func prepManaSrc(t *kop.Table, g *genPrep, p *prep) {
	tre := false
	for c := range t.Cards {
		for i := range t.Cards[c].Abilities {
			a := &t.Cards[c].Abilities[i]
			if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigSpellCast && a.Trig.Flags2&kop.Tf2ManaFromSelf != 0 {
				p.cards[c].castSrc |= csObserved
				g.castTapOn = true
			}
			if a.Kind == kop.AbMana && a.ManaHasteValid != 0 {
				p.cards[c].castSrc |= csManaHaste
				g.castTapOn, g.anyManaHaste = true, true
			}
			if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigSpellCast && a.Trig.Flags3&kop.Tf3ManaFromTreasure != 0 {
				tre = true
			}
		}
	}
	if tre {
		g.castTapOn = true
		markTreasureSrc(t, p) // trig_validsa.go
	}
	g.castTapOn = g.castTapOn || g.castRingOn
}

// noteTapSrc records that the cast being built tapped the source at sl for
// mana: an observed source (the SpellCast reader's own source). A Treasure's
// mana is marked when it is spent, not when it is tapped (treSync). Further
// plain observed sources (several Path of Ancestry: gorge's ManaSpentSources
// lists each, and each one's rider reads its own spend) are kept in obsMore;
// one past obsMore, or a second beside a deferred or rider-haste source,
// stalls loudly.
//
//go:noinline
func (s *State) noteTapSrc(k *prep, pc *pcard, sl Slot) {
	if pc.castSrc&csObserved != 0 {
		b := s.bld()
		switch {
		case b.obsSl == 0:
			b.obsSl = sl + 1
		case b.obsHas(sl):
		case !s.plainObs(k, b.obsSl-1) || !s.plainObs(k, sl) || !b.obsAdd(sl):
			s.capFull(StallObsSrc)
			return
		}
	}
	if pc.castSrc&(csManaHaste|csObserved) == csManaHaste {
		s.noteHasteSrc(k, sl) // mana_spentrider.go
	}
}

// plainObs: the observed source at sl is read only by its own SpellCast rider
// (castProvOK): no rider-haste grant (manaHasteRider) and no deferred source
// (fireDeferredRider, deferredSpent), the readers that take obsSl alone.
func (s *State) plainObs(k *prep, sl Slot) bool {
	c := s.cardAt(sl)
	return k.cards[c].castSrc&csManaHaste == 0 && (k.gen == nil || !k.gen.has(c, gfDeferred))
}

// obsHas: the cast being built recorded the observed source at sl.
func (b *build) obsHas(sl Slot) bool {
	if b.obsSl == sl+1 {
		return true
	}
	for _, o := range b.obsMore {
		if o == sl+1 {
			return true
		}
	}
	return false
}

// obsAdd records sl in obsMore's first free entry; false when it is full.
func (b *build) obsAdd(sl Slot) bool {
	for i := range b.obsMore {
		if b.obsMore[i] == 0 {
			b.obsMore[i] = sl + 1
			return true
		}
	}
	return false
}

// castProvOK is castTrigGen's cold gate for the cast-provenance bits
// (Tf2CastFromHand, Tf2ManaFromSelf).
//
//go:noinline
func (s *State) castProvOK(t *kop.Trigger, l trigListener, e *stackEntry) bool {
	if t.Flags2&kop.Tf2CastFromHand != 0 && !s.castFromHandBy(e, l.ctrl) {
		return false // trig_gravedrawn.go
	}
	if t.Flags2&kop.Tf2ManaFromSelf != 0 {
		// the payment of the cast still being built tapped this very
		// source (a stack copy or a later read never matches)
		if debugSonManaMutate == 1 {
			return false // test only: the reader never matches
		}
		if debugSonManaMutate == 2 {
			return true // test only: the reader ignores the payment
		}
		return l.sl != NoSlot && s.bld().e.kind == seSpell && s.bld().e.obj == e.obj && s.bld().obsHas(l.sl)
	}
	return true
}

// debugSonManaMutate is the lockstep mutation switch (test only): 1 the
// reader never matches, 2 it matches every cast.
var debugSonManaMutate uint8

// SetDebugSonManaMutate sets the mutation switch; tests only.
func SetDebugSonManaMutate(m uint8) { debugSonManaMutate = m }

// build.obsWin bits.
const (
	obsWinTapped uint8 = 1 << iota // the observed source was tapped in the manual window before the payment
	obsWinPaying                   // the payment began: later taps are the payment's own (obsWinPool)
)

// obsWinPool is p's pool as the cast's payment begins, and whether it is a
// valid baseline: the observed source was tapped before it (obsWinTapped) and
// this is the payment's first pass (a window resumed after a tap-another pick
// re-enters finishBuild with the first pass's taps already made). Later taps
// are the payment's own (windowTapUseful): their mana is spent.
func (s *State) obsWinPool(p uint8) (pool [kop.NumManaKinds]uint8, ok bool) {
	if !s.buildLive() {
		return pool, false
	}
	b := s.bld()
	ok = b.obsWin == obsWinTapped
	if ok {
		pool = s.pl[SeatIx(p)].pool
	}
	b.obsWin |= obsWinPaying
	return pool, ok
}

// obsWinSettle: gorge records the observed source only when the payment
// consumed a unit it produced, and it consumes those units first. A source
// tapped in the manual window before the payment floats its mana: the cast
// paid with other mana (Barracks' {W} left floating beside an X=0 {G} cost)
// did not use it. The source counts only when the payment spent some colour
// it makes. Without a valid baseline (ok false: no pre-payment tap, or a
// resumed window) obsSl stands as recorded.
func (s *State) obsWinSettle(k *prep, p uint8, before [kop.NumManaKinds]uint8, ok bool) {
	if !s.buildLive() {
		return
	}
	b := s.bld()
	b.obsWin = 0
	if !ok || b.obsSl == 0 {
		return
	}
	if b.obsMore[0] != 0 {
		// two observed sources floating before the payment: which one's
		// units it spent is unmodelled
		s.capFull(StallObsSrc)
		return
	}
	mask := s.windowOwnMask(k, b.obsSl-1)
	for c := 0; c < kop.NumManaKinds; c++ {
		if mask&(1<<c) != 0 && s.pl[SeatIx(p)].pool[c] < before[c] {
			return
		}
	}
	b.obsSl = 0
}
