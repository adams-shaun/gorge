package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// wascastspell: PredWasCastSpell, the cast provenance of a
// spell while it is on the stack or resolving. Three carriers read it:
//
//   - a SpellCast trigger's ValidCard$ (castTrigGen: matchCardTgt reads the
//     spell the cast just pushed, still on the stack);
//   - a spell ability's ConditionDefined$ Self + ConditionPresent$ Card.wasCast
//     (+!wasCastFromYourHand), read by the resolving entry;
//   - an alternative of either filter beside a type word (Lilah's
//     Instant.MultiColor+wasCastFromYourHandByYou).
//
// A mana ability's Spell.<word> RestrictValid$ is read
// off the stack too, by restrProvHolds (cast_wascastrestrict.go), not
// here. Every read here finds the spell's own entry (spellEntryOf), so the entry's prov
// IS gorge's latest PutOnStack (stackEntry.prov, zone in bits 0-2, recorded
// for a table holding the Pred: prepWasCast -> anyCastProv); nothing is
// stored per object. A stack copy is never cast (CR 707.10, gorge's IsCopy
// guard on every provenance read): the token-like ofCopy bit on the object
// makes the positive kinds false and a negated one true. A spell that has
// left the stack has no entry and reads like a copy (no kgen carrier asks).
//
// gorge's wasCastFromYourHand without ByYou is "cast from a hand by anyone"
// (WasCastFromHand: latestCastOrigin's zone == hand, the caster is not
// read); ByYou adds the reading player as the caster (WasCastFromHandByYou:
// lc.from == hand && lc.by == p). wasCastFromExile is the latest cast's
// origin == exile, any caster.

// DebugWasCastFromHandIgnored makes wasCastSpell read every spell as cast
// from its caster's hand (mutation testing only: the lockstep must catch it).
var DebugWasCastFromHandIgnored bool

// WasCastSpellStats counts the predicate's lockstep coverage
// (TestWascastspellLockstep).
type WasCastSpellStats struct {
	Reads        int // PredWasCastSpell evaluations on a cast spell
	FromHand     int // reads of a spell cast from a hand
	Elsewhere    int // reads of a spell cast from another zone
	Matched      int // reads that held
	ResGraveyard int // held reads of the resolving spell cast from a graveyard (Otterball Antics, Antiquities on the Loose)
}

// DebugWasCastSpellStats, when non-nil, accumulates the counters above.
var DebugWasCastSpellStats *WasCastSpellStats

// spellEntryOf is the stack entry of the spell object o: the resolving one,
// else the topmost on the stack; nil when o is no spell there.
func (s *State) spellEntryOf(o Obj) *stackEntry {
	if s.res.active && s.res.e.kind == seSpell && s.res.e.obj == o {
		return &s.res.e
	}
	for i := int(s.sp) - 1; i >= 0; i-- {
		if e := &s.stack[i]; e.kind == seSpell && e.obj == o {
			return e
		}
	}
	return nil
}

// wasCastSpell is PredWasCastSpell for object o, read by player you.
func (s *State) wasCastSpell(fl *kop.Filter, o Obj, you uint8) bool {
	n := fl.PredN
	cast, from, by, may, resolving, mayhem := false, kop.ZoneNone, uint8(0), false, false, false
	if e := s.spellEntryOf(o); e != nil && (s.objs[o].of&ofCopy == 0 || e.prov&7 != 0) { // a copy that was cast (prepared, CR 707.12) records its origin zone
		cast, from, by, may, resolving, mayhem = true, kop.Zone(e.prov&7), e.ctrl, s.mayPlaySpell(e), e == &s.res.e, e.trigPl == trigPlMayhem
		if DebugWasCastFromHandIgnored {
			from, by = kop.ZoneHand, you
		}
		if st := DebugWasCastSpellStats; st != nil {
			st.Reads++
			if from == kop.ZoneHand {
				st.FromHand++
			} else {
				st.Elsewhere++
			}
		}
	}
	var r bool
	switch n & 15 {
	case 0:
		r = cast
	case 1:
		r = cast && from == kop.ZoneHand
	case 2:
		r = cast && from == kop.ZoneHand && by == you
	case 3:
		r = cast && from == kop.ZoneExile
	case 4:
		r = cast && from != kop.ZoneHand
	case 5:
		r = cast && may // CastSa Spell.MayPlaySource: through a play permission (trig_mayplaystatic.go)
	case 6:
		// CastSa Spell.Mayhem: a mayhem cast marks the entry; a copy never
		// carries it
		r = (cast && mayhem) != DebugWord9Flip
	}
	r = r != (n&16 != 0)
	if st := DebugWasCastSpellStats; st != nil && r && cast {
		st.Matched++
		if resolving && from == kop.ZoneGraveyard {
			st.ResGraveyard++
		}
	}
	return r
}
