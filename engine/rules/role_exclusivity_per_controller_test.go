package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Role exclusivity is CR 704.5z/303.7a: a state-based action, grouped by the
// Roles' CURRENT controller. These tests mint real corpus Role tokens
// (role_young_hero, role_cursed, role_wicked) with TokenCreate so each one
// can be given its own controller, then attach it with an Attach event. No
// state-based check runs until the test calls checkStateBased, which is the
// point: two Roles of one controller coexist until then.

// mintAttachedRole mints the corpus token key owned and controlled by p and
// attaches it to bearer. It returns the new token's id.
func mintAttachedRole(t *testing.T, e *Engine, p state.PlayerID, key string, bearer state.ObjID) state.ObjID {
	t.Helper()
	before := e.G.NextID
	e.emit(events.Event{Kind: events.TokenCreate, Player: p, Text: key})
	if e.G.NextID != before+1 {
		t.Fatalf("TokenCreate %q minted %d objects, want 1", key, e.G.NextID-before)
	}
	id := state.ObjID(before)
	e.emit(events.Event{Kind: events.Attach, Obj: id, IDs: []state.ObjID{bearer}})
	if o := e.G.Obj(id); o == nil || o.AttachedTo != bearer {
		t.Fatalf("%q token %d not attached to bearer %d: %+v", key, id, bearer, o)
	}
	return id
}

// roleMoveZoneInLog reports whether the log carries a battlefield->graveyard
// MoveZone for id with the exclusivity text.
func roleMoveZoneInLog(e *Engine, id state.ObjID) bool {
	for _, ev := range e.L.Events {
		if ev.Kind == events.MoveZone && ev.Obj == id &&
			ev.From == state.ZBattlefield && ev.To == state.ZGraveyard &&
			ev.Text == "another Role on it: the old Role goes to the graveyard" {
			return true
		}
	}
	return false
}

// TestRoleExclusivityPerController is the reported bug: A's Young Hero Role
// and B's Cursed Role on the same creature have different controllers, so CR
// 704.5z sweeps neither. The creature is 1/1 (Cursed) and still carries the
// Young Hero attack trigger.
func TestRoleExclusivityPerController(t *testing.T) {
	t.Parallel()
	reg := choiceCorpusRegistry(t)
	e := corpusEngine(t, reg, nil, nil)
	bear := battlefieldFixture(t, e, 0, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")

	young := mintAttachedRole(t, e, 0, "role_young_hero", bear)
	cursed := mintAttachedRole(t, e, 1, "role_cursed", bear)

	e.checkStateBased()

	for _, id := range []state.ObjID{young, cursed} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("Role %d left the battlefield (%+v), but its controller differs from the other Role's", id, o)
		}
	}
	if roleMoveZoneInLog(e, young) || roleMoveZoneInLog(e, cursed) {
		t.Fatal("the SBA swept a Role even though the two Roles have different controllers")
	}
	if e.Power(bear) != 1 || e.Toughness(bear) != 1 {
		t.Fatalf("bear = %d/%d, want 1/1 from the Cursed Role alone",
			e.Power(bear), e.Toughness(bear))
	}
	grants := 0
	for _, ce := range e.active() {
		if ce.Source == young && ce.AddTrigger != nil {
			grants++
		}
	}
	if grants != 1 {
		t.Fatalf("the Young Hero Role grants %d triggers, want its 1 attack trigger", grants)
	}
}

// TestRoleExclusivityIsStateBased pins the timing: two Roles of one
// controller on one bearer coexist until the state-based check, then the
// older one goes and its departure trigger fires (Wicked Role's "each
// opponent loses 1 life").
func TestRoleExclusivityIsStateBased(t *testing.T) {
	t.Parallel()
	reg := choiceCorpusRegistry(t)
	e := corpusEngine(t, reg, nil, nil)
	bear := battlefieldFixture(t, e, 0, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")

	wicked := mintAttachedRole(t, e, 0, "role_wicked", bear)
	young := mintAttachedRole(t, e, 0, "role_young_hero", bear)

	// Precondition: the attach itself does not sweep; both Roles are on the
	// battlefield before any state-based action runs.
	for _, id := range []state.ObjID{wicked, young} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("Role %d left the battlefield at attach time (%+v); the sweep must be a state-based action", id, o)
		}
	}
	if e.Power(bear) != 3 || e.Toughness(bear) != 3 {
		t.Fatalf("bear before the check = %d/%d, want 3/3 (Wicked +1/+1)",
			e.Power(bear), e.Toughness(bear))
	}
	opponentLife := e.G.Players[1].Life

	e.checkStateBased()

	if o := e.G.Obj(wicked); o == nil || (o.Zone != state.ZGraveyard && o.Zone != state.ZCeased) {
		t.Fatalf("the older Wicked Role is in zone %+v, want graveyard (then ceased)", o)
	}
	if o := e.G.Obj(young); o == nil || o.Zone != state.ZBattlefield || o.AttachedTo != bear {
		t.Fatalf("the newer Young Hero Role did not survive: %+v", o)
	}
	if !roleMoveZoneInLog(e, wicked) {
		t.Fatal("no exclusivity MoveZone for the swept Wicked Role")
	}
	e.priorityRound()
	passUntilStackEmpty(t, e, 30)
	if got := e.G.Players[1].Life; got != opponentLife-1 {
		t.Fatalf("opponent life after the swept Wicked Role's trigger = %d, want %d", got, opponentLife-1)
	}
	if e.Power(bear) != 2 || e.Toughness(bear) != 2 {
		t.Fatalf("bear after the sweep = %d/%d, want 2/2 (Young Hero gives no pump)",
			e.Power(bear), e.Toughness(bear))
	}
}

// TestRoleExclusivityControlChange pins that the grouping reads the CURRENT
// controller: two Roles controlled by different players coexist, but when a
// control change gives both the same controller, the older one is swept.
func TestRoleExclusivityControlChange(t *testing.T) {
	t.Parallel()
	reg := choiceCorpusRegistry(t)
	e := corpusEngine(t, reg, nil, nil)
	bear := battlefieldFixture(t, e, 0, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")

	young := mintAttachedRole(t, e, 0, "role_young_hero", bear)
	cursed := mintAttachedRole(t, e, 1, "role_cursed", bear)
	e.checkStateBased()
	if e.G.Obj(young).Zone != state.ZBattlefield || e.G.Obj(cursed).Zone != state.ZBattlefield {
		t.Fatal("precondition: the two Roles must coexist while their controllers differ")
	}

	e.emit(events.Event{Kind: events.ControlChange, Obj: cursed, Player: 0})
	e.checkStateBased()

	if o := e.G.Obj(young); o == nil || (o.Zone != state.ZGraveyard && o.Zone != state.ZCeased) {
		t.Fatalf("the older Young Hero Role is in zone %+v after the control change, want graveyard", o)
	}
	if o := e.G.Obj(cursed); o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("the newer Cursed Role did not survive under its new controller: %+v", o)
	}
	if !roleMoveZoneInLog(e, young) {
		t.Fatal("no exclusivity MoveZone for the swept Young Hero Role")
	}
	if e.Power(bear) != 1 || e.Toughness(bear) != 1 {
		t.Fatalf("bear after the control-change sweep = %d/%d, want 1/1",
			e.Power(bear), e.Toughness(bear))
	}
}
