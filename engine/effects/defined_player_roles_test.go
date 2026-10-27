package effects

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// cr-g259: Defined$ ActivePlayer / AttackingPlayer / EnchantedController /
// EnchantedOwner are player selectors, never the source fallback.

func TestDefinedPlayerRoleSelectors(t *testing.T) {
	h := newHost(t, 3)
	g := h.g
	g.Active = 2

	auraID := g.AddObject(mkCard(t, "Name:Role Aura\nTypes:Enchantment Aura\nOracle:x\n"), 0).ID
	artifactID := g.AddObject(mkCard(t, "Name:Role Artifact\nTypes:Artifact\nOracle:x\n"), 2).ID
	// AddObject may grow g.Objs: take the pointers only after both exist.
	aura, artifact := g.Obj(auraID), g.Obj(artifactID)
	artifact.Controller = 1
	if artifact.Controller == artifact.Owner || artifact.Owner != 2 || aura.Controller != 0 {
		t.Fatalf("precondition: controller/owner must differ: ctl=%d owner=%d", artifact.Controller, artifact.Owner)
	}
	c := &Ctx{Source: aura.ID, Controller: 0}

	roles := func(spec string) []state.PlayerID {
		return definedPlayers(h, c, sa(t, "DB$ Draw | Defined$ "+spec))
	}
	want := func(spec string, exp ...state.PlayerID) {
		t.Helper()
		got := roles(spec)
		if !slices.Equal(got, exp) {
			t.Errorf("%s at step %v = %v, want %v", spec, g.Step, got, exp)
		}
	}

	g.Step = state.StepMain1
	want("ActivePlayer", 2)
	want("AttackingPlayer") // outside combat: nobody
	g.Step = state.StepDeclareAttackers
	want("AttackingPlayer", 2)
	want("ActivePlayer", 2)

	// Unattached: both name nobody, never the source's controller.
	want("EnchantedController")
	want("EnchantedOwner")

	aura.AttachedTo = artifact.ID
	want("EnchantedController", 1)
	want("EnchantedOwner", 2)
}
