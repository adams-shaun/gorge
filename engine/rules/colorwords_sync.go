package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The CR 612.5 colour-word change's reader-to-behaviour bridge. The static
// walk (layers.go) emits one LText substitution effect per replaced colour
// word from a Mode$ Continuous ChangeColorWordsTo$ static (Swirl the Mists);
// chars/derive.go's layer-3 arm substitutes them into the derived text, and
// this file keeps each affected object's Object.TextChanges carrying the same
// live set, in active-effect order. The rewritten Face() is what makes the
// change behavioural: a target spec's nonBlack clause reads nonRed, a
// protection-from-black keyword reads red, every param the object's text
// carries.
//
// The control is the shape reconcileControlStatics takes (control_static.go):
// a derived wanted set, recomputed at the emit tail, realized through events
// the one way events.Apply can write the field -- here events.ColorWordsSync,
// whose fold replaces exactly the Static-marked subs (a resolving
// api:ChangeText's own substitution on the same object is never lost). The
// re-derivation is a pure function of the live statics and the board, so a
// replayed log rebuilds the same TextChanges; the sync emits only when the
// live set moves.

// colourWordLetters spells the five colour words' WUBRG letters, the form
// events.Choose records on the entering object (state.Object.ChosenColor).
const colourWordLetters = "WUBRG"

// colourWordNames spells the colour words in the same WUBRG order, indexed by
// strings.IndexByte(colourWordLetters, letter): the substitution target's
// capitalised form, the one SubstituteTextWord writes ("nonblack" reads
// "nonred").
var colourWordNames = [5]string{"White", "Blue", "Black", "Red", "Green"}

// allZones is every game zone an AffectedZone$ "All" reaches (the zone enum
// up through ZCommand; ZCeased holds no objects).
var allZones = [...]state.Zone{state.ZLibrary, state.ZHand, state.ZBattlefield, state.ZGraveyard, state.ZExile, state.ZStack, state.ZCommand}

// reconcileColorWords keeps every object in a zone the live colour-word
// statics reach carrying exactly the substitutions those statics currently
// derive, in active-effect order. Runs at the emit tail, next to
// reconcileControlStatics; gated on colorWordsInPool so a match with no
// colour-word carrier in its pool pays one predictable branch.
func (e *Engine) reconcileColorWords() {
	if !e.colorWordsInPool || e.reconcilingColorWords {
		return
	}
	act := e.active()
	var live []ContinuousEffect
	for i := range act {
		if ce := &act[i]; ce.Layer == LText && ce.TextFrom != "" {
			live = append(live, *ce)
		}
	}
	if len(live) == 0 && !anyStaticTextChanges(e.G) {
		return
	}
	e.reconcilingColorWords = true
	defer func() { e.reconcilingColorWords = false }()
	// One zone admission per live effect, parsed once (the same ParseZones
	// read the derive walk's AffectedZone narrowing makes): the static's
	// AffectedZone$ names the zones its effect reaches, so an object in one
	// of them carries the substitutions and nothing else does. A value that
	// cannot parse admits nothing (fail closed, the direction the derive
	// walk takes); an omitted AffectedZone$ narrows through PermanentsOnly
	// alone (the derive walk skips the parse for the empty string), so it
	// admits everywhere the PermanentsOnly check allows.
	type zoneAdmission struct {
		zones []state.Zone
		all   bool
	}
	admits := make([]zoneAdmission, len(live))
	reach := map[state.Zone]bool{}
	if len(live) == 0 {
		// No live effect: the only work is the retraction, which visits
		// every zone the changed objects can sit in.
		for _, z := range allZones {
			reach[z] = true
		}
	}
	for i := range live {
		az := live[i].AffectedZone
		if az == "" {
			admits[i].all = true
			for _, z := range allZones {
				reach[z] = true
			}
			continue
		}
		zones, all, ok := effects.ParseZones(az)
		if !ok {
			continue
		}
		admits[i].zones, admits[i].all = zones, all
		if all {
			for _, z := range allZones {
				reach[z] = true
			}
		} else {
			for _, z := range zones {
				reach[z] = true
			}
		}
	}
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if !reach[o.Zone] {
			continue
		}
		var want []cards.TextSub
		for j := range live {
			ce := &live[j]
			if ce.PermanentsOnly && o.Zone != state.ZBattlefield {
				continue
			}
			if adm := &admits[j]; !adm.all && !zoneAdmitted(adm.zones, o.Zone) {
				continue
			}
			if !e.matchesSpecFrom(ce.Affects, o.ID, ce.Controller, ce.Source) {
				continue
			}
			want = append(want, cards.TextSub{From: ce.TextFrom, To: ce.TextTo, Static: true})
		}
		if sameTextSubs(staticSubsOf(o.TextChanges), want) {
			continue
		}
		e.emit(events.Event{Kind: events.ColorWordsSync, Obj: o.ID, Text: events.ColorWordsSyncText(want)})
	}
}

// staticSubsOf is the Static-marked portion of the object's text changes, in
// list order: what events.ColorWordsSync's fold replaces. A fresh slice, so
// the caller never aliases the object's own backing array (the fold rebuilds
// the list, never in place).
func staticSubsOf(subs []cards.TextSub) []cards.TextSub {
	var out []cards.TextSub
	for _, s := range subs {
		if s.Static {
			out = append(out, s)
		}
	}
	return out
}

// zoneAdmitted is zone.go's zoneIn, local because effects.zoneIn is private.
func zoneAdmitted(zones []state.Zone, want state.Zone) bool {
	for _, z := range zones {
		if z == want {
			return true
		}
	}
	return false
}

// sameTextSubs compares two substitution lists pair-wise (From/To; the
// UntilEOT and Static marks are construction-invariant for the two sides).
func sameTextSubs(a, b []cards.TextSub) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].From != b[i].From || a[i].To != b[i].To {
			return false
		}
	}
	return true
}

// anyStaticTextChanges reports whether any object still carries a
// Static-marked substitution, so the sync retracts it even when the live set
// has just gone empty.
func anyStaticTextChanges(g *state.Game) bool {
	for i := range g.Objs {
		for _, s := range g.Objs[i].TextChanges {
			if s.Static {
				return true
			}
		}
	}
	return false
}
