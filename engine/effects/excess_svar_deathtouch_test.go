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
	// already at lethal (toughness 4, 4 marked): lethal is 0, not raised to 1,
	// so the whole 3 is excess -- marked damage still counts on this path
	if got := excessSVarOf(t, excessSVarHit, 4); got != "3" {
		t.Fatalf("deathtouch, 4 marked on a 4-toughness creature: Excess = %q, want 3", got)
	}
}

// excessSVarFight resolves Rhino's Rampage's Fight link shape: `fighter` (the
// parent's target) fights the 0/4 wall, with the corpus ExcessSVarCondition$.
func excessSVarFight(t *testing.T, keywords string) string {
	t.Helper()
	h := newHost(t, 2)
	fighter := putCreature(t, h, 0, "Fighter", "3/3", keywords)
	wall := putCreature(t, h, 1, "Wall", "0/4", "")
	if p, tough := h.Power(fighter), h.Toughness(wall); p != 3 || tough != 4 {
		t.Fatalf("precondition: power %d toughness %d, want 3 and 4", p, tough)
	}
	c := &Ctx{Source: fighter, Controller: 0,
		Targets:       []state.Target{{Obj: fighter}},
		PickedTargets: []state.Target{{Obj: wall}}}
	Resolve(h, c, sa(t, "DB$ Fight | Defined$ ParentTarget | ValidTgts$ Creature.OppCtrl | "+
		"ExcessSVar$ Excess | ExcessSVarCondition$ Creature.targetedBy+OppCtrl"))
	return c.SVars["Excess"]
}

// TestExcessSVarFightHonoursDeathtouch: a deathtouch fighter's 3 damage to a
// 0/4 is 2 excess (CR 120.4a); without deathtouch it is 0.
func TestExcessSVarFightHonoursDeathtouch(t *testing.T) {
	if got := excessSVarFight(t, ""); got != "0" {
		t.Fatalf("no deathtouch: fight Excess = %q, want 0", got)
	}
	if got := excessSVarFight(t, "Deathtouch"); got != "2" {
		t.Fatalf("deathtouch fighter: fight Excess = %q, want 2 (lethal is 1)", got)
	}
}

// TestExcessDamageRedirectHonoursDeathtouch: Ram Through with a deathtouch
// trampler: 1 damage is lethal to the 2/2, so the creature takes 1 and its
// controller the other 4 (CR 120.4a, 702.19c).
func TestExcessDamageRedirectHonoursDeathtouch(t *testing.T) {
	victim, life, _ := ramThroughDamage(t,
		"Name:Excess DT Trampler\nTypes:Creature Beast\nPT:5/5\nK:Trample\nK:Deathtouch\nOracle:x\n")
	if victim.Damage != 1 {
		t.Fatalf("the creature took %d damage, want exactly lethal 1", victim.Damage)
	}
	if life != 16 {
		t.Fatalf("controller life = %d, want 16 (lost the 4 excess)", life)
	}
}
