package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
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
	// The static is active and scoped to its controller, so two attacks at
	// opponent 1 remain legal while two attacks at controller 0 are not.
	one := onBoardReady(t, e, 0, "Name:Attacker One\nTypes:Creature\nPT:2/2\nOracle:x\n")
	two := onBoardReady(t, e, 0, "Name:Attacker Two\nTypes:Creature\nPT:2/2\nOracle:x\n")
	if sv := e.activeStatics("AttackRestrict"); len(sv) == 0 {
		t.Fatal("precondition: AttackRestrict static not active")
	}
	if e.maxAttackers() < 2 {
		t.Fatal("ValidDefender restriction incorrectly became a global ceiling")
	}
	d := &decision.Decision{Max: 4, Options: []decision.Option{
		{Index: 0, Kind: "attacker", Obj: one, Player: 1},
		{Index: 1, Kind: "attacker", Obj: two, Player: 1},
		{Index: 2, Kind: "attacker", Obj: one, Player: 0, Group: "attack-restrict:0"},
		{Index: 3, Kind: "attacker", Obj: two, Player: 0, Group: "attack-restrict:0"},
	}}
	if err := d.Validate(decision.Intent{Choices: []int{0, 1}}); err != nil {
		t.Fatalf("shared decision rule rejected attacks at other defender: %v", err)
	}
	if err := d.Validate(decision.Intent{Choices: []int{2, 3}}); err == nil {
		t.Fatal("decision accepted two attacks at restricted defender")
	}
	if err := e.validateAttackDeclaration(d, decision.Intent{Choices: []int{0, 1}}); err != nil {
		t.Fatalf("attacks at other defender were rejected: %v", err)
	}
	if err := e.validateAttackDeclaration(d, decision.Intent{Choices: []int{2, 3}}); err == nil {
		t.Fatal("two attackers declared at restricted defender, want rejection")
	}
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
