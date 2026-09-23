package effects

// This file pins the cardinality rule of the resolution-time referent
// arguments of `AttachedTo <ref>` (review finding on agent-...27e19c88
// round 1): the supported binding is EXACTLY ONE object. A plural binding --
// a resolution with several object targets, or a trigger that remembered
// several objects -- is ambiguous, so the referent is UNBOUND and both the
// positive and the leading-'!' negated spelling fail closed as unknown
// (matchPositive returns ok=false), never an any-of guess over the plural
// list. The corpus carriers of the bare referents (Strip Bare, Hubris,
// Fiery Annihilation, Silence the Believers' single-target strive cast, Arna,
// Rhuk) bind singly; Silence the Believers at >= 2 targets is the one
// measured plural-capable carrier and it now fails closed rather than
// guessing which bearer is meant.

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func TestAttachedToReferentPluralBindingFailsClosed(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	first := corpusObject(t, reg, g, "Grizzly Bears")
	second := corpusObject(t, reg, g, "Hill Giant")

	// The aura is attached to FIRST, so under the broken any-of read the
	// plural binding WOULD match it -- the precondition that makes this test
	// fail against the old behaviour.
	auraOnFirst := corpusObject(t, reg, g, "Unholy Strength")
	auraOnFirst.AttachedTo = first.ID
	auraOnSecond := corpusObject(t, reg, g, "Unholy Strength")
	auraOnSecond.AttachedTo = second.ID

	// Precondition: two distinct live objects, and the candidate is really
	// attached to one of them.
	if first.ID == second.ID {
		t.Fatalf("precondition failed: first and second share id %d", first.ID)
	}
	if g.Obj(first.ID) == nil || g.Obj(second.ID) == nil {
		t.Fatalf("precondition failed: referent objects not live in the game")
	}
	if g.Obj(auraOnFirst.ID).AttachedTo != first.ID {
		t.Fatalf("precondition failed: auraOnFirst.AttachedTo = %d, want %d",
			g.Obj(auraOnFirst.ID).AttachedTo, first.ID)
	}

	for _, ref := range []string{"Targeted", "ParentTarget"} {
		pred := "AttachedTo " + ref
		negToken := "!AttachedTo " + ref
		spec := "Aura." + pred
		negated := "Aura." + negToken
		plural := SpecContext{You: 0, Resolving: true, ResolutionTargets: []state.Target{
			{Obj: first.ID}, {Obj: second.ID},
		}}
		// Ambiguous: the predicate is unbound, so BOTH spellings are unknown
		// and neither matches -- not an any-of match for the positive, and
		// not a "matches everything" inversion for the negation.
		// matchPredicate takes the bare predicate token (the classifier and
		// its contextPredicateBound gate live below the base split).
		if _, ok := matchPredicate(g, pred, auraOnFirst, plural); ok {
			t.Errorf("%s with two object targets must be unbound (ok=false), got bound", pred)
		}
		if _, ok := matchPredicate(g, negToken, auraOnFirst, plural); ok {
			t.Errorf("%s with two object targets must be unbound (ok=false), got bound", negToken)
		}
		if MatchesObjectCtx(g, spec, auraOnFirst, plural) {
			t.Errorf("%s must not match under an ambiguous plural binding", spec)
		}
		if MatchesObjectCtx(g, negated, auraOnFirst, plural) {
			t.Errorf("%s must not match under an ambiguous plural binding", negated)
		}
		if MatchesObjectCtx(g, negated, auraOnSecond, plural) {
			t.Errorf("%s must not match under an ambiguous plural binding either", negated)
		}
		// The grammar stays recognised -- the census still reports the token
		// as known; only the match-time binding is ambiguous.
		if un := UnknownPredicates(spec); len(un) != 0 {
			t.Errorf("UnknownPredicates(%q) = %v, want empty", spec, un)
		}

		// The contrast that pins the rule to PLURALITY alone: with exactly
		// one object target the same candidate matches as before.
		single := SpecContext{You: 0, Resolving: true, ResolutionTargets: []state.Target{
			{Obj: first.ID},
		}}
		if !MatchesObjectCtx(g, spec, auraOnFirst, single) {
			t.Errorf("%s with a single object target must still match the attached Aura", spec)
		}
		if MatchesObjectCtx(g, spec, auraOnSecond, single) {
			t.Errorf("%s with a single object target must still refuse a differently attached Aura", spec)
		}

		// A player target beside the object is dropped before the count: one
		// object plus one player is still a single, unambiguous binding.
		mixed := SpecContext{You: 0, Resolving: true, ResolutionTargets: []state.Target{
			{Obj: first.ID}, {Player: 1, IsPlayer: true},
		}}
		if !MatchesObjectCtx(g, spec, auraOnFirst, mixed) {
			t.Errorf("%s with one object and one player target must still match (the player is dropped, one object remains)", spec)
		}
	}

	// The trigger referents: two remembered objects is an ambiguous binding.
	for _, ref := range []string{"TriggeredCardLKICopy", "TriggeredAttackerLKICopy"} {
		pred := "AttachedTo " + ref
		negToken := "!AttachedTo " + ref
		spec := "Aura." + pred
		negated := "Aura." + negToken
		plural := SpecContext{You: 0, Resolving: true, Remembered: []state.Target{
			{Obj: first.ID}, {Obj: second.ID},
		}}
		if _, ok := matchPredicate(g, pred, auraOnFirst, plural); ok {
			t.Errorf("%s with two remembered objects must be unbound (ok=false), got bound", pred)
		}
		if _, ok := matchPredicate(g, negToken, auraOnFirst, plural); ok {
			t.Errorf("%s with two remembered objects must be unbound (ok=false), got bound", negToken)
		}
		if MatchesObjectCtx(g, spec, auraOnFirst, plural) {
			t.Errorf("%s must not match under an ambiguous plural remembered set", spec)
		}
		if MatchesObjectCtx(g, negated, auraOnFirst, plural) {
			t.Errorf("%s must not match under an ambiguous plural remembered set", negated)
		}
		if un := UnknownPredicates(spec); len(un) != 0 {
			t.Errorf("UnknownPredicates(%q) = %v, want empty", spec, un)
		}

		// Single remembered object keeps the round-1 behaviour.
		single := SpecContext{You: 0, Resolving: true, Remembered: []state.Target{
			{Obj: second.ID},
		}}
		if !MatchesObjectCtx(g, spec, auraOnSecond, single) {
			t.Errorf("%s with a single remembered object must still match the attached Aura", spec)
		}
		if MatchesObjectCtx(g, spec, auraOnFirst, single) {
			t.Errorf("%s with a single remembered object must still refuse a differently attached Aura", spec)
		}
	}
}
