package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// wascastenter: PredWasCastEnter, a permanent's cast
// provenance read as it enters, from the spell that is resolving into it.
//
// gorge's WasCast holds from the PutOnStack until a zone change other than
// the resolution's own stack -> battlefield entry (host_cast_ledger.go), and
// ObjectWasCastFromGraveyard adds the keyword flags; a copy is never cast.
// Every carrier reads it at the moment the permanent enters, so the
// resolving entry s.res.e IS the provenance record (stackEntry.prov, zone in
// bits 0-2, recorded for a table holding the Pred: prepWasCast); nothing is
// stored per slot. The three reads run inside enterGen / the ETB walk that
// follows it, with s.res.e still the entering spell:
//
//   - the ETB trigger match (fireObjGen -> objEventMatch -> matchPermGen),
//     run by the core's enter before the resolution pops the entry;
//   - a ChangesZoneAll batch line is the same ETB event with Tf2Batch, one
//     match per entering object;
//   - the enters replacement program's condition (enterGen's condEval, ctx.e
//     = &s.res.e, the self RefMatches filter).
//
// An object that enters any other way (a token, a reanimation or blink
// resolving as an ability, a batch return by another spell) is not the
// resolving spell's object, so it reads not cast, as in gorge where the
// provenance died with the earlier zone change.

// DebugWasCastAlways makes enterCast read true for every object (mutation
// testing only: the lockstep must catch it).
var DebugWasCastAlways bool

// enterCast reports whether o is the permanent the spell entry resolving
// right now is becoming, and the zone (kop.Zone) that spell was cast from.
func (s *State) enterCast(o Obj) (cast bool, from uint8) {
	if DebugWasCastAlways {
		return true, uint8(kop.ZoneHand)
	}
	e := s.resSpellOf(o)
	if e == nil || s.objs[o].of&(ofCopy|ofToken) != 0 {
		return false, 0 // a copy is never cast (CR 707.10); a permanent copy is already a token when its enter triggers match (permCopyEnters)
	}
	return true, e.prov & 7
}

// WasCastStats counts the predicate's lockstep coverage (TestWascastLockstep).
type WasCastStats struct {
	Reads    int // PredWasCastEnter evaluations
	Cast     int // reads of an object that was the resolving spell
	NotCast  int // reads of an object that arrived otherwise (a flicker, a reanimation, a token)
	Matched  int // reads that held
	FromHand int // reads of a spell cast from its owner's hand by its controller
}

// DebugWasCastStats, when non-nil, accumulates the counters above.
var DebugWasCastStats *WasCastStats

// wasCastPred is PredWasCastEnter for object o controlled by ctrl.
func (s *State) wasCastPred(fl *kop.Filter, o Obj, ctrl uint8) bool {
	r := s.wasCastRead(fl, o, ctrl)
	if st := DebugWasCastStats; st != nil {
		st.Reads++
		if cast, from := s.enterCast(o); cast {
			st.Cast++
			if kop.Zone(from) == kop.ZoneHand && s.res.e.ctrl == ctrl {
				st.FromHand++
			}
		} else {
			st.NotCast++
		}
		if r {
			st.Matched++
		}
	}
	return r
}

func (s *State) wasCastRead(fl *kop.Filter, o Obj, ctrl uint8) bool {
	cast, from := s.enterCast(o)
	if cast {
		switch fl.PredN & 3 {
		case 1: // wasCastFromGraveyard: a graveyard-origin keyword cast or the cast zone
			cast = kop.Zone(from) == kop.ZoneGraveyard || s.res.e.flags&sfFlashback != 0
		case 3: // wasCastFromYourGraveyard: the cast zone only, by the reading player
			cast = (kop.Zone(from) == kop.ZoneGraveyard || DebugWasCastYourGraveAnyZone) && s.res.e.ctrl == ctrl
		case 2: // wasCastFromYourHandByYou
			cast = kop.Zone(from) == kop.ZoneHand && s.res.e.ctrl == ctrl && s.objs[o].owner() == ctrl
		}
	}
	if fl.PredN&wasCastSpentZero != 0 {
		// CastSa Spell.ManaSpent EQ0: the cast spent no mana
		cast = (cast && s.spentOfSpell(&s.res.e) == 0) != DebugWord9Flip
	}
	if fl.PredN&wasCastWebSling != 0 {
		// webSlinged: the entry's web-slinging mark (a copy's is cleared,
		// CR 707.10; mintSpellCopy)
		cast = cast && webSlingEntry(&s.res.e)
	}
	return cast != (fl.PredN&4 != 0)
}

// prepWasCast makes the spell entries record their cast zone (genPrep.
// anyCastProv) for a table holding PredWasCastEnter or PredWasCastSpell
// (cast_wascastspell.go).
func prepWasCast(t *kop.Table, g *genPrep) {
	prepWord9(t, g) // filter_props.go
	for i := range t.Filters {
		if p := t.Filters[i].Pred; p == kop.PredWasCastEnter || p == kop.PredWasCastSpell {
			g.anyCastProv = true
			return
		}
	}
}
