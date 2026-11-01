package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// wascastrestrict: PredWasCastSpell on a mana
// ability's spend restriction (Interdimensional Web Watch's
// Spell.wasCastFromExile, Mm'menon's granted Spell.!wasCastFromYourHand).
//
// The restricted mana is matched against a spell that is often not on the
// stack yet (the offer, the witness plan), so wasCastSpell's entry lookup
// (spellEntryOf(NoObj)) finds nothing. The restriction matchers strip the
// predicate and read the cast through restrProvHolds with the origin the
// payment carries (payDesc.from / cast / byYou). gorge's split
// (castProvenanceAdmitsPending): an object not on the stack is priced as the
// cast it would be, every family reading the object's current zone with the
// offering player as its caster; once the entry is pushed the latest
// PutOnStack is the honest read.

// DebugRestrProvIgnored makes every provenance restriction hold (mutation
// testing only: the lockstep must catch it). It reads only in the
// restriction helper, never on the SpellCast / condition path.
var DebugRestrProvIgnored bool

// RestrictGrantStats counts the grant prefix search's reads of a restricted
// granted source (TestWascastrestrictLockstep's coverage floor: gorge never
// spends Mm'menon's mana, so the log shows no spend to count).
type RestrictGrantStats struct {
	Reads  int // restricted grants a search considered for a play
	Denied int // ... that the play's origin did not admit
}

// DebugRestrGrantStats, when non-nil, accumulates the counters above.
var DebugRestrGrantStats *RestrictGrantStats

// restrOrigin is the cast the restriction is read for: the zone it comes
// from (the current zone before the push, the entry's prov after), whether
// the entry is pushed (the honest read), and whether the reading player is
// the caster. The zero value fails closed.
type restrOrigin struct {
	from  kop.Zone
	cast  bool
	byYou bool
}

// restrProvHolds is PredWasCastSpell n for a restriction match.
func restrProvHolds(n int16, o restrOrigin) bool {
	if DebugRestrProvIgnored {
		return true
	}
	if !o.cast && o.from == kop.ZoneNone {
		return false
	}
	var r bool
	switch n & 15 {
	case 0:
		r = true
	case 1:
		r = o.from == kop.ZoneHand
	case 2:
		r = o.from == kop.ZoneHand && (o.byYou || !o.cast)
	case 3:
		r = o.from == kop.ZoneExile
	case 4:
		r = o.from != kop.ZoneHand
	}
	return r != (n&16 != 0)
}

// restrOriginOfObj is the origin of the offer of object o (not on the
// stack): its current zone, the pending read.
func (s *State) restrOriginOfObj(o Obj) restrOrigin {
	return restrOrigin{from: s.objs[o].zone}
}

// restrOriginOfEntry is the origin of the pushed spell entry e, paid for by
// its caster: the honest read.
func restrOriginOfEntry(e *stackEntry) restrOrigin {
	return restrOrigin{from: kop.Zone(e.prov & 7), cast: true, byYou: true}
}

// restrFilterMatch: card c, cast from origin og, matches restriction
// alternative fl of the source of player p (chosen-type words are the
// caller's).
func (s *State) restrFilterMatch(k *prep, fl *kop.Filter, c CardID, p uint8, og restrOrigin) bool {
	prov, n := fl.Pred == kop.PredWasCastSpell, fl.PredN
	if prov {
		fl.Pred, fl.PredN = kop.PredNone, 0
	}
	if !s.matchCardOne(k, fl, c, p, p, NoObj) {
		return false
	}
	return !prov || restrProvHolds(n, og)
}
