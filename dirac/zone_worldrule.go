package dirac

// worldLosers is the CR 704.5k world rule (gorge rules/sba.go worldRule):
// with two or more World permanents on the battlefield, every one except the
// one that has had the supertype for the shortest time goes to its owner's
// graveyard; a tie for the shortest time sends them all. World is printed
// only in the corpus (no effect grants or removes it), so the card record's
// flag is the supertype, and a face-down permanent has none (CR 708.2a).
// The youngest has the greatest layer timestamp: gorge reads
// Object.Timestamp, set at entry and kept across a control change, which
// lstamp mirrors (perm.ts is not: joined restamps it).
// The move is a plain one (gorge "world rule" MoveZone): the caller sends the
// losers through dieZone like the other graveyard casualties.
func (s *State) worldLosers(k *prep) (out slotMask) {
	var w slotMask
	for m := s.occ.andNot(s.faceDownSlots()); m.any(); m = m.rest() {
		if sl := m.first(); k.t.Cards[s.cardAt(sl)].World {
			w = w.with(sl)
		}
	}
	if !w.many() {
		return slotMask{}
	}
	var top uint16
	ties := 0
	for m := w; m.any(); m = m.rest() {
		switch ts := s.layerTs(m.first()); {
		case ties == 0 || ts > top:
			top, ties = ts, 1
		case ts == top:
			ties++
		}
	}
	if ties > 1 {
		return w
	}
	for m := w; m.any(); m = m.rest() {
		if sl := m.first(); s.layerTs(sl) != top {
			out = out.with(sl)
		}
	}
	return out
}
