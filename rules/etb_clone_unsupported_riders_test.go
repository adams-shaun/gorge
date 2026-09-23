package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The two tests below pin the VALUE half of etbCloneWhitelist: a body whose
// parameter KEYS are all supported but whose value this build cannot carry
// faithfully is withheld, keeping the loud unimplemented-API fallback, rather
// than offering a copy election that would be empty (Mockingbird) or would
// silently drop the exception (Flesh Duplicate).

// TestMockingbirdETBWithheld pins the resolver-dependent selector case.
// Mockingbird's Choices$ Creature.Other+cmcLEY compares against
// SVar:Y:Count$CastTotalManaSpent; every matcher on the ETB route
// (etbOptions at announcement, cloneETBTemplateLegal at replacement time)
// runs through MatchesSpecFrom, which has no SVar resolver, so the predicate
// answers "recognised shape, never matches" for every creature. Offering the
// election would therefore present a list the player can only decline. The
// carrier is withheld instead.
func TestMockingbirdETBWithheld(t *testing.T) {
	e := handEngine(t, corpusAlternativeCard(t, "Mockingbird"))
	bear := e.G.AddObject(corpusAlternativeCard(t, "Grizzly Bears"), 1)
	bear.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{bear.ID})
	id := e.G.Zone(state.ZHand, 0)[0]

	// The reviewer's finding, pinned directly: the selector is unevaluable
	// without a resolver, and the eligible 2-mana creature is omitted by the
	// very matcher both ETB sites use.
	const spec = "Creature.Other+cmcLEY"
	if !effects.SpecNeedsResolver(spec) {
		t.Fatal("Mockingbird's Choices$ selector must be reported as resolver-dependent")
	}
	if effects.MatchesSpecFrom(e.G, spec, bear.ID, 0, id) {
		t.Fatal("precondition: the no-resolver matcher is expected to omit every creature here")
	}

	e.G.Players[0].Pool[state.MC] = 6
	e.G.Players[0].Pool[state.MU] = 1
	castMode(t, e, id, "")
	for {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		if d.Kind == decision.KChoose && d.Source == id {
			for _, o := range d.Options {
				if o.Kind == "clone" {
					t.Fatalf("withheld carrier offered the copy election: %+v", d.Options)
				}
			}
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	finishCast(t, e, id)
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("mockingbird did not enter as itself: %+v", o)
	}
	if hasEvent(e, events.ClonePermanent, id) {
		t.Fatal("withheld carrier must not copy")
	}
	if !hasNote(e, "unimplemented API Clone") {
		t.Fatal("loud Clone fallback note missing from the log")
	}
}

// TestFleshDuplicateETBWithheld pins the conditional-keyword case. Flesh
// Duplicate's rider is AddKeywords$ IfNew Vanishing:3 ("vanishing 3 if that
// creature doesn't have vanishing"). effClone installs an AddKeywords$ member
// verbatim as a layer-6 grant and this build implements no IfNew conditional,
// so cards.KeywordHead would read the head as "IfNew Vanishing": no
// conditional test, no vanishing ability and no entry time counters. The
// carrier is withheld rather than offering a copy that silently drops it.
func TestFleshDuplicateETBWithheld(t *testing.T) {
	e := handEngine(t, corpusAlternativeCard(t, "Flesh Duplicate"))
	dread := e.G.AddObject(corpusAlternativeCard(t, "Colossal Dreadmaw"), 1)
	dread.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{dread.ID})
	id := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MU] = 2
	castMode(t, e, id, "")
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose && d.Source == id {
		t.Fatalf("withheld carrier offered the copy election: %+v", d)
	}
	finishCast(t, e, id)
	if d := e.Pending(); d != nil && d.Kind == decision.KPriority {
		passUntilStackEmpty(t, e, 60)
	}
	if hasEvent(e, events.ClonePermanent, id) {
		t.Fatal("withheld carrier must not copy")
	}
	if !hasNote(e, "unimplemented API Clone") {
		t.Fatal("loud Clone fallback note missing from the log")
	}
	// It entered as ITSELF: still the printed 0/0 Flesh Duplicate, not the
	// 6/6 template (an SBA sweep will bin it; this harness asserts the entry,
	// which is what the withhold is about). Nothing granted the IfNew rider.
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Face().Name != "Flesh Duplicate" {
		t.Fatalf("flesh duplicate did not enter as itself: %+v", o)
	}
	if der := e.Derived(id); der.Power != 0 || der.Toughness != 0 {
		t.Fatalf("self-entry is %d/%d, want the printed 0/0", der.Power, der.Toughness)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.CounterChange && ev.Obj == id {
			t.Fatalf("withheld carrier placed counters: %+v", ev)
		}
	}
	if der := e.Derived(dread.ID); slices.ContainsFunc(der.Keywords, func(k string) bool {
		return k == "Vanishing:3" || k == "IfNew Vanishing:3"
	}) {
		t.Fatalf("template keywords leaked the unimplemented rider: %v", der.Keywords)
	}
}
