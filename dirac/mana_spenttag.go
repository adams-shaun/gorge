package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Producer-tagged Count$CastTotalManaSpent (CR 601.2h; gorge's
// Object.ManaTreasureSpent / ManaCaveSpent / ManaDesertSpent): a card that
// reads Count$CastTotalManaSpent <Tag> (CardDef.SpentTag) records the units of
// that tag the cast's payment spent in place of the total, exactly as the Snow
// form records the snow units (mana_snow.go). The kernel already tallies the
// Treasure units floating in each player's pool (genState.poolTre); Cave and
// Desert units need their own per-seat/colour tally, a cold region (a State
// field would fail TestNoNewFixedStateFields).

const tagKinds = 2 // Cave, Desert (Treasure rides genState.poolTre)

// tagIndex maps a kop.SpentTag to the cold region's tag slot.
func tagIndex(tag uint8) int {
	switch tag {
	case kop.SpentTagCave:
		return 0
	case kop.SpentTagDesert:
		return 1
	}
	return -1
}

var useSpentTag = newGenUse(tableAnySpentTag)

var coldTagN = newCold[uint8]("spenttag", func(g *genPrep) int {
	if g.uses.has(useSpentTag) {
		return tagKinds * MaxSeats * kop.NumManaKinds
	}
	return 0
})

func tableAnySpentTag(t *kop.Table) bool {
	for i := range t.Cards {
		if t.Cards[i].SpentTag != 0 {
			return true
		}
	}
	return false
}

// tableSpentTags reports whether any card reads the Treasure-tagged form, and
// whether any reads a Cave or Desert form (the tally's producers' marks).
func tableSpentTags(t *kop.Table) (tre, tag bool) {
	for i := range t.Cards {
		switch t.Cards[i].SpentTag {
		case kop.SpentTagTreasure:
			tre = true
		case kop.SpentTagCave, kop.SpentTagDesert:
			tag = true
		}
	}
	return tre, tag
}

func tagSlot(tag uint8, seat, col int) int {
	return tagIndex(tag)*MaxSeats*kop.NumManaKinds + seat*kop.NumManaKinds + col
}

// tagSync clamps p's Cave/Desert tallies to the units its pool still holds.
// gorge's takeUnit spends plain units first, then the typed units in their
// Treasure > Cave > Desert order, then snow last: the remaining Cave units are
// therefore min(tallied, left - Treasure), Desert's min(tallied, left -
// Treasure - Cave), after the snow units are removed from left.
func (s *State) tagSync(p uint8) {
	sx := coldTagN.of(s)
	if sx == nil {
		return
	}
	i := SeatIx(p)
	snow := coldSnow.of(s)
	for c := 0; c < kop.NumManaKinds; c++ {
		left := int(s.pl[i].pool[c])
		if snow != nil {
			if n := int(snow[int(i)*kop.NumManaKinds+c]); n < left {
				left -= n
			} else {
				left = 0
			}
		}
		if tre := int(s.g.poolTre[i][c]); tre < left {
			left -= tre
		} else {
			left = 0
		}
		cave := &sx[tagSlot(kop.SpentTagCave, int(i), c)]
		if int(*cave) > left {
			*cave = uint8(left)
		}
		left -= int(*cave)
		des := &sx[tagSlot(kop.SpentTagDesert, int(i), c)]
		if int(*des) > left {
			*des = uint8(left)
		}
	}
}

// tagTotal is the units of tag floating in p's pool, the tally synced first.
func (s *State) tagTotal(tag uint8, p uint8) int {
	i := SeatIx(p)
	switch tag {
	case kop.SpentTagTreasure:
		s.treSync(p)
		n := 0
		for c := 0; c < kop.NumManaKinds; c++ {
			n += int(s.g.poolTre[i][c])
		}
		return n
	case kop.SpentTagCave, kop.SpentTagDesert:
		s.tagSync(p)
		sx := coldTagN.of(s)
		if sx == nil {
			return 0
		}
		n := 0
		base := tagSlot(tag, int(i), 0)
		for c := 0; c < kop.NumManaKinds; c++ {
			n += int(sx[base+c])
		}
		return n
	}
	return 0
}

// tagAdd records n Cave/Desert units a typed source added to p's pool.
func (s *State) tagAdd(k *prep, p uint8, card CardID, col, n int) {
	if n <= 0 || k.gen == nil {
		return
	}
	pc := &k.cards[card]
	tag := uint8(0)
	switch {
	case pc.castSrc&csCave != 0:
		tag = kop.SpentTagCave
	case pc.castSrc&csDesert != 0:
		tag = kop.SpentTagDesert
	}
	if tag == 0 {
		return
	}
	sx := coldTagN.of(s)
	if sx == nil {
		return
	}
	t := &sx[tagSlot(tag, int(SeatIx(p)), col)]
	*t = uint8(min(int(*t)+n, 255))
}

// spentPreSpend, as a cast's payment begins, keeps the card's producer-tagged
// pool tally for castSpentRec.
func (s *State) spentPreSpend(k *prep, e *stackEntry) {
	if k.gen == nil || coldTagN.of(s) == nil {
		return
	}
	tag := k.t.Cards[e.card].SpentTag
	if tag == 0 {
		return
	}
	s.bld().tagPre = uint8(min(s.tagTotal(tag, e.ctrl), 255))
}

// castSpentRec is the amount a cast records as its spend (histCast): the Snow
// form's snow units (snowSpentRec), a producer-tagged card's units of its tag,
// or the total.
func (s *State) castSpentRec(k *prep, e *stackEntry, spent int) int {
	if k.gen == nil {
		return spent
	}
	if k.gen.anySnow {
		spent = s.snowSpentRec(k, e, spent)
	}
	if DebugSpentTagTotal || coldTagN.of(s) == nil {
		return spent
	}
	tag := k.t.Cards[e.card].SpentTag
	if tag == 0 || k.t.Cards[e.card].SpentSnow {
		return spent
	}
	post := s.tagTotal(tag, e.ctrl)
	return max(int(s.bld().tagPre)-post, 0)
}

// DebugSpentTagTotal records a tagged cast's total, not its tag's units
// (mutation testing only): the lockstep must catch it.
var DebugSpentTagTotal bool

// tagClearSeat empties seat p's Cave/Desert tallies (its pool emptied).
func (s *State) tagClearSeat(p uint8) {
	sx := coldTagN.of(s)
	if sx == nil {
		return
	}
	for t := 0; t < tagKinds; t++ {
		base := t*MaxSeats*kop.NumManaKinds + int(SeatIx(p))*kop.NumManaKinds
		clear(sx[base : base+kop.NumManaKinds])
	}
}

// tagClearCol empties seat p's Cave/Desert tally for one colour.
func (s *State) tagClearCol(p uint8, col int) {
	sx := coldTagN.of(s)
	if sx == nil {
		return
	}
	for t := 0; t < tagKinds; t++ {
		sx[t*MaxSeats*kop.NumManaKinds+int(SeatIx(p))*kop.NumManaKinds+col] = 0
	}
}

// tagClearAll empties every seat's Cave/Desert tally.
func (s *State) tagClearAll() {
	if sx := coldTagN.of(s); sx != nil {
		clear(sx)
	}
}

// manaTagTyped reports a printed producer word that types the card's mana
// (a Cave or Desert land), as its subtypes would.
func manaTagTyped(tag uint8) bool {
	return tag == kop.ManaTagCave || tag == kop.ManaTagDesert
}

// spentTagMark is the castSrc spend-tally mark a producer with printed word
// tag carries: set only when the table has a reader of that tag (spentTre for
// Treasure, spentTag for Cave / Desert); 0 otherwise.
func spentTagMark(tag uint8, spentTre, spentTag bool) uint8 {
	switch {
	case tag == kop.ManaTagTreasure && spentTre:
		return csTreasure
	case tag == kop.ManaTagCave && spentTag:
		return csCave
	case tag == kop.ManaTagDesert && spentTag:
		return csDesert
	}
	return 0
}
