package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func TestAttackRestrictValidDefenderIsScoped(t *testing.T) {
	e := combatEngine(t)
	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers
	judoon := onBoard(t, e, 0, "Name:Judoon Enforcers\nTypes:Creature\nPT:2/2\nS:Mode$ AttackRestrict | MaxAttackers$ 1 | ValidDefender$ You\nOracle:x\n")
	if o := e.G.Obj(judoon); o == nil || o.Zone != state.ZBattlefield {
		t.Fatal("precondition: restriction source not on battlefield")
	}
	// The active static is present and its scoped gate must not become a
	// table-wide Max value.
	one := onBoardReady(t, e, 0, "Name:Attacker One\nTypes:Creature\nPT:2/2\nOracle:x\n")
	two := onBoardReady(t, e, 0, "Name:Attacker Two\nTypes:Creature\nPT:2/2\nOracle:x\n")
	sv := e.activeStatics("AttackRestrict")
	if len(sv) == 0 {
		t.Fatal("precondition: AttackRestrict static not active")
	}
	if e.maxAttackers() < 2 {
		t.Fatal("ValidDefender restriction incorrectly became a global ceiling")
	}
	_ = one
	_ = two
}

func TestAttackRestrictSilentArbiterRemainsGlobal(t *testing.T) {
	e := combatEngine(t)
	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers
	onBoard(t, e, 0, "Name:Silent Arbiter\nTypes:Artifact Creature\nPT:1/5\nS:Mode$ AttackRestrict | MaxAttackers$ 1\nOracle:x\n")
	if got := e.maxAttackers(); got != 1 {
		t.Fatalf("global ceiling = %d, want 1", got)
	}
}
