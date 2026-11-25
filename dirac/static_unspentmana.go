package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Printed S:Mode$ UnspentMana statics (kop.StatUnspentMana, CR 500.4; gorge
// unspentManaKeep at the one ManaClear emit site, engine/rules/turn.go): the
// pool slots a live static names keep their unspent mana as each step and
// phase ends — the cleanup-to-untap boundary included, so the mana stays
// until the static's gate stops holding and the next boundary empties it.
// gorge keeps whole pool slots (the ManaClear fold honours the keep letters);
// the pm tally (PersistentMana$) is untouched, and the restricted batches
// (State.rb) survive with the slot: the fold keeps a batch whose Persistent
// flag is set or whose colour slot the letters protect (mana_restrbatch.go
// rbKeep), so a kept slot's own units are ordinary mana, as gorge's are.
//
// Static.A is the colour mask in the pool kinds' bit order (W U B R G C);
// StUnspentYou binds the static's controller only, absent every seat. Like
// gorge the read happens at each boundary: a gate that stops holding between
// boundaries still protects the mana until the next one.

// useUnspentMana arms unspentMask: a table holding a StatUnspentMana static
// scans the statics at every boundary emptier.
var useUnspentMana = newGenUse(func(t *kop.Table) bool { return tableHasStatic(t, kop.StatUnspentMana) })

// unspentMask is player p's protected pool slots as a bit per pool kind (0:
// nothing protected), read at a boundary with k the boundary's prep.
func (s *State) unspentMask(k *prep, p uint8) uint8 {
	if g := k.gen; g == nil || !g.uses.has(useUnspentMana) {
		return 0
	}
	var m uint8
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatUnspentMana || !s.statCond(k, st) {
			return true
		}
		if a.Static.StFlags&kop.StUnspentYou != 0 && st.ctrl != p {
			return true
		}
		m |= uint8(a.Static.A) & maskKinds
		return true
	})
	return m
}

// maskKinds is every pool kind's bit (W U B R G C; Static.A carries the
// colour mask in the pool kinds' order).
const maskKinds = (1 << kop.NumManaKinds) - 1

// unspentProt is every seat's protected pool slots, read once for a boundary
// so the keep set cannot shift between a seat's empty and the next's.
func (s *State) unspentProt() [MaxSeats]uint8 {
	var out [MaxSeats]uint8
	k := s.k()
	if g := k.gen; g == nil || !g.uses.has(useUnspentMana) {
		return out
	}
	for p := uint8(0); p < s.nSeats(); p++ {
		out[p] = s.unspentMask(k, p)
	}
	return out
}

// unspentEmptyPool drops p's unprotected pool slots; prot holds the slots a
// live StatUnspentMana static protects (the ordinary step-boundary empties).
func (s *State) unspentEmptyPool(p uint8, prot uint8) {
	pool := &s.pl[p].pool
	for c := 0; c < int(kop.NumManaKinds); c++ {
		if prot&(1<<uint(c)) == 0 {
			pool[c] = 0
		}
	}
}

// talliesKeep empties a boundary's tag tallies but the entries of the colours
// a live StatUnspentMana static protects: gorge's clearNonPersistent drains a
// slot's TypedMana/ArtifactTyped/Snow tallies only for the slots it empties
// (engine/events/apply_state_helpers.go), so a keep-letter slot's tallies
// survive with its pool units. The tallies are per seat per colour, exactly
// the keep mask's per-slot index. restr carries the poolArt bits the caller
// kept for its restricted batches (poolArtRestr); a kept slot's colour bit
// and the snow bit ride with them, cleared when nothing of the kind survives.
func (s *State) talliesKeep(prot, restr [MaxSeats]uint8) {
	sx := coldSnow.of(s)
	ax := coldArtN.of(s)
	if prot == [MaxSeats]uint8{} && restr == [MaxSeats]uint8{} {
		// the common boundary, no keep: every seat's tallies empty (the
		// loop below, unrolled to clears)
		n := int(s.nSeats()) * kop.NumManaKinds
		if sx != nil {
			clear(sx[:n])
		}
		if ax != nil {
			clear(ax[:n])
		}
		for p := uint8(0); p < s.nSeats(); p++ {
			s.hasteClear(p, 0)
			s.g.poolTre[p] = [kop.NumManaKinds]uint8{}
			s.g.poolArt[p] = 0
		}
		return
	}
	for p := uint8(0); p < s.nSeats(); p++ {
		m := prot[p]
		i := SeatIx(p)
		s.hasteClear(i, m)
		var art uint8
		anySnow := false
		for c := 0; c < int(kop.NumManaKinds); c++ {
			if m&(1<<uint(c)) == 0 {
				if sx != nil {
					sx[int(i)*kop.NumManaKinds+c] = 0
				}
				if ax != nil {
					ax[int(i)*kop.NumManaKinds+c] = 0
				}
				s.g.poolTre[i][c] = 0
				continue
			}
			art |= s.g.poolArt[i] & (1 << uint(c))
			if sx != nil && sx[int(i)*kop.NumManaKinds+c] != 0 {
				anySnow = true
			}
		}
		if anySnow {
			art |= poolArtSnow
		}
		s.g.poolArt[i] = art | restr[i]
	}
}

// unspentProtAny reports whether any seat's keep mask is non-zero.
func (s *State) unspentProtAny(prot [MaxSeats]uint8) bool {
	for _, m := range prot {
		if m != 0 {
			return true
		}
	}
	return false
}
