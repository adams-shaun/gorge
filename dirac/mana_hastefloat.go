package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The haste rider of mana that floats (CR 106.6; mana_spentrider.go). Carnelian
// Orb of Dragonkind's mana can be tapped at priority, before the cast: gorge
// keeps the unit as a typed batch (ArtifactR) carrying the AddsKeywords$ grant,
// and a spell payment carves that batch ahead of the other units of its kind
// (pay.emitRestrictedManaSpend), so the grant rides the spell whose payment
// spent the unit. The kernel taps a source inside the cast's payment and
// pools the unit as plain mana, so it must tell whether this payment or a
// later cast spent it.
//
// coldHaste tallies, per seat and colour, the rider units still floating, and
// the source that made them. gorge's pay.takeUnit spends plain units first,
// then the typed ones in tag order (Artifact before ArtifactTreasure), so
// after any pool spend the units left are the most the slot can hold behind
// the Treasure's: min(tallied, left - snow - Treasure) (hasteSync, as treSync
// does for a Treasure). A unit that went was spent; when a spell is being
// built that is the cast's rider source, and manaHasteRider reads it as it
// reads a source the payment tapped.
//
// Approximations: a typed unit of a lower tag (a Cave's or a Desert's) or
// another Artifact unit of the same colour in the slot is not told apart from
// plain mana; a source that left the battlefield and whose slot was reused by
// another card grants nothing (gorge still would); a second rider source
// floating beside the first stalls (StallObsSrc).

// hasteFloat is one seat's floating rider units.
type hasteFloat struct {
	n    [kop.NumManaKinds]uint8
	sl   Slot   // 1 + the slot of the source that made them
	card CardID // that source's card, to tell a reused slot
}

func (h *hasteFloat) any() bool {
	for _, n := range h.n {
		if n != 0 {
			return true
		}
	}
	return false
}

// useHasteFloat: some mana ability carries a ManaHasteValid rider.
var useHasteFloat = newGenUse(func(t *kop.Table) bool {
	for c := range t.Cards {
		for i := range t.Cards[c].Abilities {
			if a := &t.Cards[c].Abilities[i]; a.Kind == kop.AbMana && a.ManaHasteValid != 0 {
				return true
			}
		}
	}
	return false
})

var coldHaste = newCold[hasteFloat]("hastefloat", func(g *genPrep) int {
	if g.uses.has(useHasteFloat) {
		return MaxSeats
	}
	return 0
})

// hasteTag records that the rider source at sl added a unit of colour col to
// p's pool (a cast's payment tap, or a unit an earlier cast left floating: the
// oracle's lowered witness taps for the largest X); it is called after the
// unit is added (artTag). Whether a cast spent it is hasteSync's, when the
// pool has been spent.
func (s *State) hasteTag(k *prep, p uint8, sl Slot, col int) {
	hx := coldHaste.of(s)
	c := s.cardAt(sl)
	if hx == nil || k.cards[c].castSrc&(csManaHaste|csObserved) != csManaHaste {
		return
	}
	// every lowered rider is the same grant (kgen manaSpentRiders): a second
	// source's units count beside the first's, and the first stays the source
	h := &hx[SeatIx(p)]
	if !h.any() {
		h.sl, h.card = sl+1, c
	}
	// a tally the pool no longer holds (a boundary that emptied it unseen)
	// cannot outlive the units before this one
	if lim := s.pl[SeatIx(p)].pool[col]; lim > 0 && h.n[col] >= lim {
		h.n[col] = lim - 1
	}
	if h.n[col] < 255 {
		h.n[col]++
	}
}

// hasteSync clamps p's rider tally to what the pool still holds (artSync, after
// treSync's clamp) and, when a spell is being built, takes a unit that went as
// the cast's rider source.
func (s *State) hasteSync(p uint8) {
	hx := coldHaste.of(s)
	if hx == nil {
		return
	}
	i := SeatIx(p)
	h := &hx[i]
	spent := false
	for c := range h.n {
		if h.n[c] == 0 {
			continue
		}
		pool := int(s.pl[i].pool[c])
		left := pool - min(int(s.g.poolTre[i][c]), pool)
		if s.g.poolArt[i]&poolArtSnow != 0 {
			if sx := coldSnow.of(s); sx != nil {
				left -= int(sx[int(i)*kop.NumManaKinds+c])
			}
		}
		left = max(left, 0)
		if left < int(h.n[c]) {
			h.n[c] = uint8(left)
			spent = true
		}
	}
	if !spent || !s.buildLive() || s.bld().e.kind != seSpell || h.sl == 0 || s.cardAt(h.sl-1) != h.card {
		return
	}
	// a reader's source (Tf2ManaFromSelf) beside the rider's stalls loudly, as
	// two readers do; a second rider source keeps the first
	if b := s.bld(); b.obsSl == 0 {
		b.obsSl = h.sl
	} else if b.obsSl != h.sl && s.k().cards[s.cardAt(b.obsSl-1)].castSrc&(csManaHaste|csObserved) != csManaHaste {
		s.capFull(StallObsSrc)
	}
}

// hasteSettle runs when a spell's mana payment is done: the pool is as the
// payment left it, whether or not every spend path synced the tally.
func (s *State) hasteSettle(p uint8) {
	if s.buildLive() && s.bld().e.kind == seSpell {
		s.hasteSync(p)
	}
}

// hasteClear empties the rider tally of every seat (colours: all), or only
// the colours of seat i that prot does not protect.
func (s *State) hasteClear(i uint8, prot uint8) {
	if hx := coldHaste.of(s); hx != nil {
		for c := range hx[i].n {
			if prot&(1<<uint(c)) == 0 {
				hx[i].n[c] = 0
			}
		}
	}
}
