package effects

// This file pins two related filter capabilities on real corpus spellings:
//
//   - the bare `Attached` predicate (the candidate is attached to the source), and
//   - the resolution-time referent arguments of `AttachedTo <ref>`
//     (Targeted, ParentTarget, TriggeredCardLKICopy, TriggeredAttackerLKICopy).
//
// It complements engine/effects/attachedto_predicate_test.go, which pins the literal
// and dotted forms and the player-attachment fail-closed behaviour. Both are
// driven through the shared classifier/dispatch, so the matcher and
// UnknownPredicates cannot disagree.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestAttachedPredicate pins the bare `Attached` predicate (Forge's
// CardProperty, source.hasCardAttachment): the candidate is attached to the
// source. The corpus spells it only in filter position -- Count$Valid
// Equipment.Attached, Aura.Attached, Choices$, IsPresent$ -- so the base type
// word does the object-kind narrowing.
func TestAttachedPredicate(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	bear := corpusObject(t, reg, g, "Grizzly Bears")
	mountain := corpusObject(t, reg, g, "Mountain")

	auraOnBear := corpusObject(t, reg, g, "Unholy Strength")
	auraOnBear.AttachedTo = bear.ID
	auraOnLand := corpusObject(t, reg, g, "Wild Growth")
	auraOnLand.AttachedTo = mountain.ID
	auraFree := corpusObject(t, reg, g, "Unholy Strength")

	equipOnBear := corpusObject(t, reg, g, "Bonesplitter")
	equipOnBear.AttachedTo = bear.ID
	equipFree := corpusObject(t, reg, g, "Bonesplitter")

	// Precondition: every attachment relationship the assertions depend on is
	// really set, and the compared objects really differ.
	if g.Obj(auraOnBear.ID).AttachedTo != bear.ID {
		t.Fatalf("precondition failed: auraOnBear.AttachedTo = %d, want bear %d", g.Obj(auraOnBear.ID).AttachedTo, bear.ID)
	}
	if g.Obj(equipFree.ID).AttachedTo != 0 {
		t.Fatalf("precondition failed: equipFree.AttachedTo = %d, want 0", g.Obj(equipFree.ID).AttachedTo)
	}
	if bear.ID == mountain.ID {
		t.Fatalf("precondition failed: bear and mountain share id %d", bear.ID)
	}

	onBear := SpecContext{You: 0, Source: bear.ID}
	// Positive: an Aura/Equipment attached to the source matches its own type word + Attached.
	if !MatchesObjectCtx(g, "Aura.Attached", auraOnBear, onBear) {
		t.Errorf("Aura.Attached must match an Aura attached to the source")
	}
	if !MatchesObjectCtx(g, "Aura.Attached", auraOnLand, SpecContext{You: 0, Source: mountain.ID}) {
		t.Errorf("Aura.Attached must match an Aura attached to a land source")
	}
	if !MatchesObjectCtx(g, "Equipment.Attached", equipOnBear, onBear) {
		t.Errorf("Equipment.Attached must match an Equipment attached to the source")
	}
	// The corpus's dominant spelling: a comma list of the two attachment kinds.
	if !MatchesObjectCtx(g, "Equipment.Attached,Aura.Attached", equipOnBear, onBear) {
		t.Errorf("Equipment.Attached,Aura.Attached must match an attached Equipment")
	}
	if !MatchesObjectCtx(g, "Equipment.Attached,Aura.Attached", auraOnBear, onBear) {
		t.Errorf("Equipment.Attached,Aura.Attached must match an attached Aura")
	}

	// Negative: an attachment on something else, and an unattached one, match nothing.
	if MatchesObjectCtx(g, "Aura.Attached", auraOnLand, onBear) {
		t.Errorf("Aura.Attached must not match an Aura attached to another permanent")
	}
	if MatchesObjectCtx(g, "Aura.Attached", auraFree, onBear) {
		t.Errorf("Aura.Attached must not match an unattached Aura")
	}
	if MatchesObjectCtx(g, "Equipment.Attached", equipFree, onBear) {
		t.Errorf("Equipment.Attached must not match an unattached Equipment")
	}
	if MatchesObjectCtx(g, "Creature.Attached", bear, SpecContext{You: 0, Source: mountain.ID}) {
		t.Errorf("Creature.Attached must not match a creature that merely bears an attachment")
	}

	// A source that left the battlefield reads what was attached as it left.
	aura := g.Obj(auraOnBear.ID)
	g.Obj(bear.ID).Zone = state.ZGraveyard
	aura.AttachedTo, aura.LastBearer = 0, bear.ID
	if !MatchesObjectCtx(g, "Aura.Attached", aura, onBear) {
		t.Errorf("Aura.Attached must match what was attached to a source that left")
	}
	g.Obj(bear.ID).Zone = state.ZBattlefield
	if MatchesObjectCtx(g, "Aura.Attached", aura, onBear) {
		t.Errorf("Aura.Attached must not match a former attachment of a source still here")
	}
	aura.AttachedTo, aura.LastBearer = bear.ID, 0

	// The leading-'!' path negates the recognised positive word.
	if !MatchesObjectCtx(g, "Aura.!Attached", auraFree, onBear) {
		t.Errorf("Aura.!Attached must match an unattached Aura")
	}
	if MatchesObjectCtx(g, "Aura.!Attached", auraOnBear, onBear) {
		t.Errorf("Aura.!Attached must not match an attached Aura")
	}

	// The matcher and UnknownPredicates agree: Attached is recognised.
	for _, spec := range []string{"Aura.Attached", "Equipment.Attached", "Creature.Attached", "Aura.!Attached"} {
		if un := UnknownPredicates(spec); len(un) != 0 {
			t.Errorf("UnknownPredicates(%q) = %v, want empty (Attached is recognised)", spec, un)
		}
	}
}

// TestAttachedToContextReferents pins the resolution-time referent arguments
// of `AttachedTo <ref>`: Targeted and ParentTarget (the resolving ability's
// own targets), and TriggeredCardLKICopy / TriggeredAttackerLKICopy (the
// object a trigger captured). The match is the candidate's own AttachedTo
// against the live objects the referent names. Each referent is bound the way
// a real resolution binds it (SpecContext.Resolving + ResolutionTargets, or
// Remembered), and an absent binding fails closed -- matchPositive returns
// ok=false (unknown) so the leading-'!' spelling cannot invert the absence.
func TestAttachedToContextReferents(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	target := corpusObject(t, reg, g, "Grizzly Bears")
	other := corpusObject(t, reg, g, "Hill Giant")

	auraOnTarget := corpusObject(t, reg, g, "Unholy Strength")
	auraOnTarget.AttachedTo = target.ID
	auraOnOther := corpusObject(t, reg, g, "Unholy Strength")
	auraOnOther.AttachedTo = other.ID
	auraFree := corpusObject(t, reg, g, "Unholy Strength")

	// Precondition: the two attachments point at DIFFERENT live permanents.
	if g.Obj(auraOnTarget.ID).AttachedTo != target.ID || g.Obj(auraOnOther.ID).AttachedTo != other.ID {
		t.Fatalf("precondition failed: attachments %d/%d not bound to %d/%d",
			g.Obj(auraOnTarget.ID).AttachedTo, g.Obj(auraOnOther.ID).AttachedTo, target.ID, other.ID)
	}
	if target.ID == other.ID {
		t.Fatalf("precondition failed: target and other share id %d", target.ID)
	}

	resolving := SpecContext{You: 0, Resolving: true,
		ResolutionTargets: []state.Target{{Obj: target.ID}}}

	for _, ref := range []string{"Targeted", "ParentTarget"} {
		spec := "Aura.AttachedTo " + ref
		if !MatchesObjectCtx(g, spec, auraOnTarget, resolving) {
			t.Errorf("%s must match an Aura attached to the resolving ability's target", spec)
		}
		if MatchesObjectCtx(g, spec, auraOnOther, resolving) {
			t.Errorf("%s must not match an Aura attached to a permanent that is not the target", spec)
		}
		if MatchesObjectCtx(g, spec, auraFree, resolving) {
			t.Errorf("%s must not match an unattached Aura", spec)
		}
		// Unbound (no resolution): the referent names nothing, so the
		// predicate is unknown, not a silent false.
		if _, ok := matchPositive(g, spec, auraOnTarget, SpecContext{You: 0}); ok {
			t.Errorf("%s outside a resolution must be unbound (ok=false), got bound", spec)
		}
		if un := UnknownPredicates(spec); len(un) != 0 {
			t.Errorf("UnknownPredicates(%q) = %v, want empty (the referent grammar is recognised)", spec, un)
		}
	}

	// Player-only targets are unrepresentable as an attachment bearer: an
	// AttachedTo Targeted with only a player target admits nothing, and the
	// referent is still bound (the resolution exists) so it is a real false.
	playerOnly := SpecContext{You: 0, Resolving: true,
		ResolutionTargets: []state.Target{{Player: 1, IsPlayer: true}}}
	if MatchesObjectCtx(g, "Aura.AttachedTo Targeted", auraOnTarget, playerOnly) {
		t.Errorf("Aura.AttachedTo Targeted must not match when the target is a player, not an object")
	}

	// The trigger referents read the same Remembered set the Defined$ selector
	// of the same name resolves (engine/effects/context.go), so the two readings of
	// one spelling cannot disagree. Arna, Skycaptain's real source filter is
	// `Permanent.!token+AttachedTo TriggeredAttackerLKICopy`.
	triggered := SpecContext{You: 0, Resolving: true,
		Remembered: []state.Target{{Obj: other.ID}}}
	for _, ref := range []string{"TriggeredCardLKICopy", "TriggeredAttackerLKICopy"} {
		spec := "Aura.AttachedTo " + ref
		if !MatchesObjectCtx(g, spec, auraOnOther, triggered) {
			t.Errorf("%s must match an Aura attached to the remembered trigger object", spec)
		}
		if MatchesObjectCtx(g, spec, auraOnTarget, triggered) {
			t.Errorf("%s must not match an Aura attached to a different permanent", spec)
		}
		// Unbound (no remembered object): unknown, never a silent false.
		if _, ok := matchPositive(g, spec, auraOnOther, SpecContext{You: 0, Resolving: true}); ok {
			t.Errorf("%s with no remembered object must be unbound (ok=false), got bound", spec)
		}
		if un := UnknownPredicates(spec); len(un) != 0 {
			t.Errorf("UnknownPredicates(%q) = %v, want empty (the referent grammar is recognised)", spec, un)
		}
	}

	// A remembered PLAYER entry alone is not an object referent: fail closed.
	playerRemembered := SpecContext{You: 0, Resolving: true,
		Remembered: []state.Target{{Player: 1, IsPlayer: true}}}
	if _, ok := matchPositive(g, "Aura.AttachedTo TriggeredAttackerLKICopy", auraOnTarget, playerRemembered); ok {
		t.Errorf("TriggeredAttackerLKICopy with only a player remembered must be unbound, got bound")
	}
}
