package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

const (
	excessSVarHit = "Name:Excess Hit\nTypes:Creature Snake\nPT:3/3\nK:Deathtouch\n" +
		"SVar:Hit:DB$ DealDamage | ValidTgts$ Creature | NumDmg$ 3 | ExcessSVar$ Excess\nOracle:x\n"
	excessSVarHitPlain = "Name:Excess Hit Plain\nTypes:Creature Snake\nPT:3/3\n" +
		"SVar:Hit:DB$ DealDamage | ValidTgts$ Creature | NumDmg$ 3 | ExcessSVar$ Excess\nOracle:x\n"
	excessSVarWall = "Name:Excess Wall\nTypes:Creature Wall\nPT:0/4\nOracle:x\n"
)

// excessSVarOf resolves the source card's Hit link against a 0/4 wall that
// already carries `marked` damage and returns the published Excess binding.
func excessSVarOf(t *testing.T, src string, marked int32) string {
	t.Helper()
	h := newHost(t, 2)
	card := mkCard(t, src)
	face := card.Faces[0]
	source := h.g.AddObject(card, 0)
	source.Zone = state.ZBattlefield
	wall := h.g.AddObject(mkCard(t, excessSVarWall), 1)
	wall.Zone = state.ZBattlefield
	wall.Damage = marked
	if got := h.Toughness(wall.ID); got != 4 {
		t.Fatalf("precondition: wall toughness %d, want 4", got)
	}
	c := &Ctx{Source: source.ID, Controller: 0, SVars: face.SVars,
		PickedTargets: []state.Target{{Obj: wall.ID}}}
	Resolve(h, c, svarSA(t, face, "Hit"))
	return c.SVars["Excess"]
}

// TestExcessSVarHonoursDeathtouch: CR 120.4a/120.10 -- a deathtouch source
// makes 1 damage lethal, so 3 damage to a 4-toughness creature is 2 excess.
// The same hit without deathtouch is 0 excess, and marked damage still counts
// toward lethality on the deathtouch path.
func TestExcessSVarHonoursDeathtouch(t *testing.T) {
	if got := excessSVarOf(t, excessSVarHitPlain, 0); got != "0" {
		t.Fatalf("no deathtouch: Excess = %q, want 0", got)
	}
	if got := excessSVarOf(t, excessSVarHit, 0); got != "2" {
		t.Fatalf("deathtouch source: Excess = %q, want 2 (lethal is 1)", got)
	}
	if got := excessSVarOf(t, excessSVarHit, 3); got != "2" {
		t.Fatalf("deathtouch, 3 marked on a 4-toughness creature: Excess = %q, want 2", got)
	}
}
