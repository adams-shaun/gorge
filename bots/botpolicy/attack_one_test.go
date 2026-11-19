package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestClampAttackOneAnswersValidate pins the bot half of the PlayerMustAttack
// duty (Option.AttackOne): whatever the combat heuristic proposes -- nothing,
// an unmarked pair, an over-full list -- the answer the KAttackers arm hands
// back (LegalAttackChoices, then Clamp's decision.FitRequired) passes
// Decision.Validate, the rule the engine enforces, and carries a marked pair.
func TestClampAttackOneAnswersValidate(t *testing.T) {
	d := &decision.Decision{Kind: decision.KAttackers, Player: 0, Min: 0, Max: 2, Options: []decision.Option{
		{Index: 0, Kind: "attacker", Obj: 1, Player: 1, AttackOne: true},
		{Index: 1, Kind: "attacker", Obj: 1, Player: 2},
		{Index: 2, Kind: "attacker", Obj: 2, Player: 2},
		{Index: 3, Kind: "attacker", Obj: 2, Player: 1, AttackOne: true},
	}}
	// Precondition: the empty declaration breaks the duty, so a pass-through
	// clamp would fail the check below.
	if d.AttackOneSatisfied(nil) {
		t.Fatal("fixture wrong: the empty declaration satisfies the AttackOne duty")
	}
	b := Board{Life: TableOf(map[state.PlayerID]int32{0: 20})}
	for _, proposed := range [][]int{nil, {1}, {2}, {1, 2}, {0}, {0, 3}} {
		in := Clamp(d, decision.Intent{Player: 0, Choices: LegalAttackChoices(b, d, proposed)})
		if err := d.Validate(in); err != nil {
			t.Fatalf("proposed %v: clamped answer %v failed Validate: %v", proposed, in.Choices, err)
		}
		if !d.AttackOneSatisfied(in.Choices) {
			t.Fatalf("proposed %v: clamped answer %v attacks with no marked pair", proposed, in.Choices)
		}
	}
}
