package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Which kicker alternative a two-cost Kicker card paid (CR 702.33b; gorge
// Object.CastFlags FlagKicked1 / FlagKicked2, effects/filter_word.go
// wordKickedIndex). Card.Self+kicked 1 / kicked 2 read it (kop.PredKickedAlt).
//
//   - the record is cold.mkObj, the multikicker count's per-object byte
//     (kw_multikicker.go), one byte per object: bit0 the first alternative
//     paid, bit1 the second. One kicked family per card -- a Kicker2 card is
//     never Multikicker/Squad/Replicate (kgen refuses the pair,
//     TestKicker2BesideMultikickerRefuses) -- so the byte is never both, and
//     a table with either family allocates it once: prepareGen ORs the mkAny
//     bit into g.anyMultikick for a useKickAlt table, instead of a dedicated
//     region that would hold the State bar for the mbc pool (its 234-deck
//     table carries Thornscape Battlemage; the bar had 80 bytes of headroom).
//   - only setKickAlt writes it, from the cast path. A single-cost kicked cast
//     (sfKicked alone) sets no bit, so neither index reads true for it: gorge's
//     "the bare FlagKicked bit alone never matches an index form".
//   - it ends where the multikicker count ends: a card cast again (beginCast),
//     an entry that is not the resolving spell (turn_gen.go), a reused object
//     id (zone_objreuse.go). A new object was never alternative-kicked
//     (CR 400.7).
//   - the read is the battlefield permanent's object byte; off the battlefield
//     nothing matches (kgen refuses the word there).

// useKickAlt: the table has a PredKickedAlt filter.
var useKickAlt = newGenUse(tableAnyKickedAlt)

// tableAnyKickedAlt reports whether t reads the kicker-alternative byte: a
// PredKickedAlt filter, or a two-alternative Kicker carrier whose cast
// writes it and whose Count$TimesKicked (the popcount) reads it
// (kw_kickalt.go).
func tableAnyKickedAlt(t *kop.Table) bool {
	for i := range t.Cards {
		if hasKicker2(&t.Cards[i]) {
			return true
		}
	}
	for i := range t.Filters {
		if t.Filters[i].Pred == kop.PredKickedAlt {
			return true
		}
	}
	return false
}

// setKickAlt records the kicker alternatives object o's cast paid: bit0 the
// first, bit1 the second.
func (s *State) setKickAlt(o Obj, bits uint8) {
	if ka := coldMkObj.of(s); int(o) < len(ka) {
		ka[o] = bits & 3
	}
}

// clearKickAlt ends object o's alternative-kicker record.
func (s *State) clearKickAlt(o Obj) {
	if ka := coldMkObj.of(s); int(o) < len(ka) {
		ka[o] = 0
	}
}

// kickedAltPredicate is PredKickedAlt: the battlefield permanent at sl was
// cast with kicker alternative PredN (1 or 2) paid.
func (s *State) kickedAltPredicate(fl *kop.Filter, sl Slot) bool {
	ka := coldMkObj.of(s)
	o := s.perms[sl].obj
	if fl.PredN < 1 || fl.PredN > 2 || int(o) >= len(ka) {
		return false
	}
	return ka[o]&(1<<(uint(fl.PredN)-1)) != 0
}
