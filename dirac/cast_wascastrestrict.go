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
// the caster. The zero value fails closed. bestowed: the cast is a bestowed
// one, an Aura spell and no creature spell (CR 702.103b; gorge
// pay.Descriptor.Bestowed, bestowedSpecMatches).
type restrOrigin struct {
	from     kop.Zone
	cast     bool
	byYou    bool
	bestowed bool
	// owner is the payment spell's owner when ownSet: its controller is the
	// caster the callers pass beside it (p), so a MayPlay cast of another
	// player's card reads owner != controller and an owner filter on the
	// restriction sees it (CR 601.2g-h; Thieving Varmint's "spells you don't
	// own"). A literal that leaves owner unset defaults to the caster in
	// restrFilterMatch — the read before the owner was carried — so no literal
	// can silently mean seat 0. Build an origin with an owner through
	// restrOriginOfObj / restrOriginOfEntry.
	owner  uint8
	ownSet bool
	// obj is the spell's object when the origin names one, objSet. The
	// restriction matcher passes it to the card matcher so an object word
	// (IsCommander on a commander cast, CR 903.3) can
	// read the object; a card-only candidate leaves it NoObj.
	obj    Obj
	objSet bool
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
	case 7:
		// wasCastFromYourGraveyard: the offer reads the object's graveyard
		// zone, the payment the pushed entry's origin (castOriginAdmitsAtZone)
		r = o.from == kop.ZoneGraveyard && (o.byYou || !o.cast)
	case 8:
		// wasCastFromGraveyard, any caster: the offer reads the object's
		// graveyard zone, the payment the entry's origin (gorge
		// bareGraveyardCastAdmitsPending)
		r = o.from == kop.ZoneGraveyard
	case 9:
		// wasCastFromYourLibrary: the offer reads the library zone, the payment
		// the entry's origin cast by the reading player (gorge
		// castOriginAdmitsAtZone, castOriginTokens: byYou)
		r = o.from == kop.ZoneLibrary && (o.byYou || !o.cast)
	}
	return r != (n&16 != 0)
}

// restrOriginOfObj is the origin of the offer of object o (not on the
// stack): its current zone, its owner and the pending read.
func (s *State) restrOriginOfObj(o Obj) restrOrigin {
	return restrOrigin{from: s.objs[o].zone, owner: s.objs[o].owner(), ownSet: true, obj: o, objSet: o != NoObj}
}

// restrOriginOfEntry is the origin of the pushed spell entry e, paid for by
// its caster: the honest read. The spell's owner is its object's (a MayPlay
// push of another player's card); an entry with no object is the caster's.
func (s *State) restrOriginOfEntry(e *stackEntry) restrOrigin {
	own := e.ctrl
	if e.obj != NoObj {
		own = s.objs[e.obj].owner()
	}
	og := restrOrigin{from: kop.Zone(e.prov & 7), cast: true, byYou: true, bestowed: e.bestowedEntry(), owner: own, ownSet: true}
	if e.obj != NoObj {
		og.obj, og.objSet = e.obj, true
	}
	return og
}

// restrFilterMatch: card c, cast from origin og, matches restriction
// alternative fl of the source of player p (chosen-type words are the
// caller's). The candidate is the spell being cast: owner og.owner (the
// caster where the origin carries none), its controller p, so an owner word
// reads the caster's ownership as gorge's AvailableFor does (CR 106.6).
func (s *State) restrFilterMatch(k *prep, fl *kop.Filter, c CardID, p uint8, og restrOrigin) bool {
	own := og.owner
	if !og.ownSet {
		own = p // a literal origin carries no owner: the caster's own
	}
	prov, n := fl.Pred == kop.PredWasCastSpell, fl.PredN
	if prov {
		fl.Pred, fl.PredN = kop.PredNone, 0
	}
	if og.bestowed {
		// the bestowed spell is an Aura, not a creature (CR 702.103b): read the
		// alternative's type words against that, the rest against the card
		if ty := k.cards[c].types&^kop.TCreature | kop.TAura; fl.Types != 0 && ty&fl.Types == 0 || ty&fl.NotTypes != 0 {
			return false
		}
		fl.Types, fl.NotTypes = 0, 0
	}
	o := NoObj
	if og.objSet && s.cmdSlot(p, og.obj) >= 0 {
		// A commander cast's own object is the command-zone object, so the
		// IsCommander word can read it (CR 903.3). Every other candidate
		// stays a card-only read, as before.
		o = og.obj
	}
	if !s.matchCardOne(k, fl, c, own, p, o) {
		return false
	}
	return !prov || restrProvHolds(n, og)
}
