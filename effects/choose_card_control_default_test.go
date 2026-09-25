package effects

// The implicit chooser-controlled default for ChooseCard's candidate pool.
// Slaughter the Strong / Destined Confrontation carry no Choices$ and no
// ControlledByPlayer$, so before this fix cardChoices offered every player's
// battlefield objects to every chooser ("each player chooses ... creatures
// they control"). These tests pin the candidate pool itself for each chooser
// (not the eventual chosen result), the one home of the default
// (chooseCardControl), and the boundary that the explicit-pool shapes --
// Choices$, DefinedCards$, ValidTgts$, AllCards$ -- keep their prior behaviour.

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// controlCreature puts one battlefield creature under seat p and returns it.
func controlCreature(t *testing.T, h *fakeHost, p state.PlayerID, name, pt string) *state.Object {
	t.Helper()
	o := h.g.AddObject(mkCard(t, "Name:"+name+"\nTypes:Creature\nPT:"+pt+"\nOracle:x\n"), p)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	return o
}

// choiceIDs projects a candidate pool to object ids in pool order.
func choiceIDs(pool []state.Target) []state.ObjID {
	out := make([]state.ObjID, 0, len(pool))
	for _, t := range pool {
		out = append(out, t.Obj)
	}
	return out
}

// TestChooseCardDefaultPoolIsChooserControlled is the real-corpus regression
// for the implicit chooser-controlled pool: with eligible permanents under
// BOTH seats, each chooser's cardChoices pool is exactly their own qualifying
// creatures, and the opponent's are excluded.
func TestChooseCardDefaultPoolIsChooserControlled(t *testing.T) {
	_, sa := corpusSA(t, "Slaughter the Strong", "")
	if sa.API != "ChooseCard" {
		t.Fatalf("Slaughter the Strong fixture changed: %+v", sa)
	}
	// Precondition: the SA is the unconstrained shape the default exists for.
	for _, p := range []string{"Choices", "ControlledByPlayer", "DefinedCards", "ValidTgts"} {
		if v, ok := sa.Params[p]; ok && v != "" {
			t.Fatalf("fixture is not the unconstrained shape: %s = %q", p, v)
		}
	}

	h := newHost(t, 2)
	mineA := controlCreature(t, h, 0, "Mine A", "3/3")
	mineB := controlCreature(t, h, 0, "Mine B", "1/1")
	theirsA := controlCreature(t, h, 1, "Theirs A", "2/2")
	theirsB := controlCreature(t, h, 1, "Theirs B", "2/2")

	// Preconditions: both seats really have qualifying battlefield creatures,
	// and each is controlled by the seat it was registered under (a vacuous
	// board would let the exclusion assertion pass silently).
	if mineA.Zone != state.ZBattlefield || mineB.Zone != state.ZBattlefield ||
		theirsA.Zone != state.ZBattlefield || theirsB.Zone != state.ZBattlefield {
		t.Fatalf("precondition: a creature is not on the battlefield: %v %v %v %v",
			mineA.Zone, mineB.Zone, theirsA.Zone, theirsB.Zone)
	}
	if mineA.Controller != 0 || mineB.Controller != 0 || theirsA.Controller != 1 || theirsB.Controller != 1 {
		t.Fatalf("precondition: controllers = [%d %d %d %d], want [0 0 1 1]",
			mineA.Controller, mineB.Controller, theirsA.Controller, theirsB.Controller)
	}

	// Each chooser's pool is their own creatures in registration order; the
	// opponent's are excluded.
	got0 := choiceIDs(cardChoices(h, &Ctx{Controller: 0}, sa, 0))
	if !sameIDs(got0, []state.ObjID{mineA.ID, mineB.ID}) {
		t.Fatalf("chooser 0 pool = %v, want own creatures [%d %d] (opponent's %d %d must be excluded)",
			got0, mineA.ID, mineB.ID, theirsA.ID, theirsB.ID)
	}
	for _, id := range []state.ObjID{theirsA.ID, theirsB.ID} {
		if containsID(got0, id) {
			t.Fatalf("chooser 0 offered opponent creature %d: %v", id, got0)
		}
	}

	got1 := choiceIDs(cardChoices(h, &Ctx{Controller: 0}, sa, 1))
	if !sameIDs(got1, []state.ObjID{theirsA.ID, theirsB.ID}) {
		t.Fatalf("chooser 1 pool = %v, want own creatures [%d %d] (opponent's %d %d must be excluded)",
			got1, theirsA.ID, theirsB.ID, mineA.ID, mineB.ID)
	}
	for _, id := range []state.ObjID{mineA.ID, mineB.ID} {
		if containsID(got1, id) {
			t.Fatalf("chooser 1 offered opponent creature %d: %v", id, got1)
		}
	}

	// The two pools must actually differ, or the test could pass on a board
	// where every creature happened to share a controller.
	if sameIDs(got0, got1) {
		t.Fatalf("chooser pools are identical (%v) -- the board did not distinguish the seats", got0)
	}
}

// TestChooseCardControlDefaultBoundary pins the one home of the default: the
// unconstrained shape resolves to Chooser, and each explicit-pool shape stays
// unrestricted (empty = admit every candidate).
func TestChooseCardControlDefaultBoundary(t *testing.T) {
	_, stt := corpusSA(t, "Slaughter the Strong", "")
	if stt.Params["Choices"] != "" || stt.Params["ControlledByPlayer"] != "" || stt.Params["DefinedCards"] != "" {
		t.Fatalf("Slaughter the Strong is no longer the unconstrained shape: %+v", stt.Params)
	}
	if got := chooseCardControl(stt); got != "Chooser" {
		t.Fatalf("unconstrained ChooseCard control = %q, want the implicit Chooser default", got)
	}

	_, lastOne := corpusSA(t, "Last One Standing", "")
	if lastOne.Params["Choices"] == "" {
		t.Fatalf("Last One Standing no longer carries Choices$: %+v", lastOne.Params)
	}
	if got := chooseCardControl(lastOne); got != "" {
		t.Fatalf("Choices$-bearing ChooseCard control = %q, want unchanged empty (preserve explicit Choices$)", got)
	}

	_, wild := corpusSA(t, "Wild Swing", "DBChooseRandom")
	if wild.Params["DefinedCards"] != "Targeted" {
		t.Fatalf("Wild Swing fixture changed: %+v", wild.Params)
	}
	if got := chooseCardControl(wild); got != "" {
		t.Fatalf("DefinedCards$ ChooseCard control = %q, want unchanged empty (preserve DefinedCards$)", got)
	}

	targeted := sa(t, "SP$ ChooseCard | ValidTgts$ Creature")
	if got := chooseCardControl(targeted); got != "" {
		t.Fatalf("ValidTgts$ ChooseCard control = %q, want unchanged empty (preserve target pool)", got)
	}

	all := sa(t, "SP$ ChooseCard | AllCards$ True")
	if got := chooseCardControl(all); got != "" {
		t.Fatalf("AllCards$ ChooseCard control = %q, want unchanged empty (preserve all-card pool)", got)
	}

	explicit := sa(t, "SP$ ChooseCard | ControlledByPlayer$ Left")
	if got := chooseCardControl(explicit); got != "Left" {
		t.Fatalf("explicit ControlledByPlayer$ = %q, want Left as written", got)
	}
}

// TestChooseCardAllCardsPoolKeepsOpponentObject pins the AllCards$ boundary:
// this explicit pool remains cross-controller when Choices$ is absent.
func TestChooseCardAllCardsPoolKeepsOpponentObject(t *testing.T) {
	all := sa(t, "SP$ ChooseCard | AllCards$ True")
	h := newHost(t, 2)
	mine := controlCreature(t, h, 0, "My AllCard", "2/2")
	theirs := controlCreature(t, h, 1, "Their AllCard", "2/2")
	if mine.Zone != state.ZBattlefield || theirs.Zone != state.ZBattlefield || mine.Controller == theirs.Controller {
		t.Fatalf("precondition: expected distinct controllers' battlefield objects, got zones %v/%v controllers %d/%d",
			mine.Zone, theirs.Zone, mine.Controller, theirs.Controller)
	}
	got := choiceIDs(cardChoices(h, &Ctx{Controller: 0}, all, 0))
	if !sameIDs(got, []state.ObjID{mine.ID, theirs.ID}) {
		t.Fatalf("AllCards$ pool = %v, want both controllers' objects [%d %d]", got, mine.ID, theirs.ID)
	}
}

// TestChooseCardDefinedCardsPoolKeepsOpponentTarget is the boundary at the
// pool level: a DefinedCards$ set the chooser does not control must still be
// offered. Wild Swing's chooser is the spell's controller, and its target may
// be an opponent's permanent; the implicit default must not reach it.
func TestChooseCardDefinedCardsPoolKeepsOpponentTarget(t *testing.T) {
	_, wild := corpusSA(t, "Wild Swing", "DBChooseRandom")
	if wild.Params["DefinedCards"] != "Targeted" || wild.Params["ControlledByPlayer"] != "" {
		t.Fatalf("Wild Swing fixture changed: %+v", wild.Params)
	}
	h := newHost(t, 2)
	opponent := controlCreature(t, h, 1, "Their Target", "2/2")
	if opponent.Controller == 0 {
		t.Fatalf("precondition: the DefinedCards$ target must be controlled by someone other than the chooser")
	}
	c := &Ctx{Controller: 0, Targets: []state.Target{{Obj: opponent.ID}}}
	got := choiceIDs(cardChoices(h, c, wild, 0))
	if !sameIDs(got, []state.ObjID{opponent.ID}) {
		t.Fatalf("Wild Swing pool = %v, want the opponent-controlled target [%d] (DefinedCards$ must not be chooser-filtered)",
			got, opponent.ID)
	}
}
