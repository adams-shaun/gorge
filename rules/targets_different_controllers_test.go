package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// protectorOfTheWastesEngine seats Protector of the Wastes on the battlefield
// under seat 0 and returns its ETB target SA plus three battlefield artifacts:
// two controlled by seat 0 (sameControllerA/B) and one by seat 1. The
// different-controllers constraint is then exercised across a real controller
// split. Preconditions are asserted so a vacuous fixture fails loudly rather
// than passing silently.
func protectorOfTheWastesEngine(t *testing.T) (e *Engine, sa *cards.SA, a0, a1, b0 state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	protector, ok := reg.Lookup("Protector of the Wastes")
	if !ok {
		t.Fatal("Protector of the Wastes missing from corpus")
	}
	artifact := func() *cards.Card {
		return &cards.Card{Faces: []*cards.Face{{Name: "Relic", Types: []string{"Artifact"}}}}
	}
	e = newSeats(t, 3)
	// Two artifacts under seat 0 and one under seat 1: the offer must group the
	// seat-0 pair together and the seat-1 artifact apart.
	a0 = e.G.AddObject(artifact(), 0).ID
	a1 = e.G.AddObject(artifact(), 0).ID
	b0 = e.G.AddObject(artifact(), 1).ID
	for _, id := range []state.ObjID{a0, a1, b0} {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})
		if z := e.G.Obj(id).Zone; z != state.ZBattlefield {
			t.Fatalf("fixture artifact %d zone = %v, want battlefield", id, z)
		}
	}
	protectorID := e.G.AddObject(protector, 0).ID
	e.emit(events.Event{Kind: events.MoveZone, Obj: protectorID, From: state.ZLibrary, To: state.ZBattlefield})
	if z := e.G.Obj(protectorID).Zone; z != state.ZBattlefield {
		t.Fatalf("fixture Protector zone = %v, want battlefield", z)
	}
	// The ETB ChangesZone trigger's Execute$ is the DB$ ChangeZone SA whose
	// ValidTgts$ Artifact,Enchantment the ask must group.
	sa = cards.ResolveSVar(protector.Faces[0].SVars, "TrigExile")
	if sa == nil {
		t.Fatal("Protector's TrigExile SVar did not resolve to a target SA")
	}
	if sa.Params["ValidTgts"] != "Artifact,Enchantment" {
		t.Fatalf("fixture precondition: TrigExile ValidTgts$ = %q, want Artifact,Enchantment", sa.Params["ValidTgts"])
	}
	if sa.Params["TargetsWithDifferentControllers"] != "True" {
		t.Fatalf("fixture precondition: TrigExile TargetsWithDifferentControllers$ = %q, want True", sa.Params["TargetsWithDifferentControllers"])
	}
	return e, sa, a0, a1, b0
}

// TestProtectorOfTheWastesDifferentControllersGroups pins the offer-time half
// of TargetsWithDifferentControllers$ on Protector of the Wastes' real corpus
// card: every target option carries the same controller-keyed Option.Group
// (the wire contract Decision.Validate enforces), so an answer reusing one
// controller is impossible to submit while one target per controller is
// accepted. It is the sibling of TestTargetsForEachPlayerUsesDecisionGroups,
// which pins the same grouping for TargetsForEachPlayer$ — both constraints
// share the one helper.
func TestProtectorOfTheWastesDifferentControllersGroups(t *testing.T) {
	e, _, a0, a1, b0 := protectorOfTheWastesEngine(t)
	e.pending = nil
	e.priorityRound()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending = %+v, want the ETB trigger's target decision", d)
	}
	if d.Min != 0 || d.Max != 2 {
		t.Fatalf("target bounds = (%d, %d), want (0, 2)", d.Min, d.Max)
	}
	if len(d.Options) != 3 {
		t.Fatalf("options = %+v, want one per battlefield artifact", d.Options)
	}
	groupOf := map[state.ObjID]string{}
	for _, o := range d.Options {
		if o.Obj != a0 && o.Obj != a1 && o.Obj != b0 {
			t.Fatalf("offered %d — only the three battlefield artifacts are targets", o.Obj)
		}
		groupOf[o.Obj] = o.Group
	}
	if groupOf[a0] == "" || groupOf[a0] != groupOf[a1] || groupOf[a0] == groupOf[b0] {
		t.Fatalf("groups = %v, want one group per controller (seat-0 pair together, seat-1 apart)", groupOf)
	}
	indexOf := func(id state.ObjID) int {
		for _, o := range d.Options {
			if o.Obj == id {
				return o.Index
			}
		}
		return -1
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{indexOf(a0), indexOf(a1)}}); err == nil {
		t.Fatal("two artifacts controlled by one player were accepted")
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{indexOf(a0), indexOf(b0)}}); err != nil {
		t.Fatalf("one artifact per controller rejected: %v", err)
	}
}

// TestProtectorOfTheWastesExilesOnlyDifferentControllers drives the real card
// end to end: the ETB trigger's ChangeZone exiles the two chosen artifacts
// (one per controller) and leaves the unchosen same-controller artifact on the
// battlefield. Before the key was read, the seat-0 pair was a legal answer and
// the card exiled two permanents one player controlled.
func TestProtectorOfTheWastesExilesOnlyDifferentControllers(t *testing.T) {
	e, _, a0, a1, b0 := protectorOfTheWastesEngine(t)
	e.pending = nil
	e.priorityRound()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending = %+v, want the ETB trigger's target decision", d)
	}
	indexOf := func(id state.ObjID) int {
		for _, o := range d.Options {
			if o.Obj == id {
				return o.Index
			}
		}
		return -1
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{indexOf(a0), indexOf(b0)}}); err != nil {
		t.Fatalf("submit one-per-controller answer: %v", err)
	}
	passUntilStackEmpty(t, e, 40)
	if z := e.G.Obj(a0).Zone; z != state.ZExile {
		t.Fatalf("chosen artifact %d zone = %v, want exile", a0, z)
	}
	if z := e.G.Obj(b0).Zone; z != state.ZExile {
		t.Fatalf("chosen artifact %d zone = %v, want exile", b0, z)
	}
	if z := e.G.Obj(a1).Zone; z != state.ZBattlefield {
		t.Fatalf("unchosen same-controller artifact %d zone = %v, want battlefield", a1, z)
	}
}

// TestProtectorOfTheWastesResolutionRecheckNarrowsSameController pins the
// resolution half: CR 608.2b's recheck on the real card's SA keeps one target
// per controller when the recorded set has grown to violate the constraint
// (a controller change in response). The recheck only removes; it never
// widens a list the per-target filter already narrowed, and a
// different-controller set passes through untouched.
func TestProtectorOfTheWastesResolutionRecheckNarrowsSameController(t *testing.T) {
	e, sa, a0, a1, b0 := protectorOfTheWastesEngine(t)
	// Precondition: the real SA carries the flag the recheck keys on.
	if sa.Params["TargetsWithDifferentControllers"] != "True" {
		t.Fatal("fixture precondition: the real SA does not carry TargetsWithDifferentControllers$")
	}
	same := []state.Target{{Obj: a0}, {Obj: a1}}
	got := e.legalTargets(same, sa, targetZones(sa), 0, 0, 0)
	if len(got) != 1 || got[0].Obj != a0 {
		t.Fatalf("same-controller recheck = %+v, want only the first recorded target %d", got, a0)
	}
	mixed := []state.Target{{Obj: a0}, {Obj: b0}}
	got = e.legalTargets(mixed, sa, targetZones(sa), 0, 0, 0)
	if len(got) != 2 || got[0].Obj != a0 || got[1].Obj != b0 {
		t.Fatalf("different-controller recheck = %+v, want both targets kept", got)
	}
	// The constraint must not leak onto a non-flag-bearing SA: the same
	// same-controller pair survives an ordinary Artifact,Enchantment recheck.
	plain := &cards.SA{Params: map[string]string{"ValidTgts": "Artifact,Enchantment"}}
	if got := e.legalTargets(same, plain, targetZones(plain), 0, 0, 0); len(got) != 2 {
		t.Fatalf("plain recheck = %+v, want both targets kept (no constraint)", got)
	}
}
