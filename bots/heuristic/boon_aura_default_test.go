package heuristic

import (
	"context"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// TestHeuristicBoonAuraTargetsOwnCreature pins the default bot's cast-time
// Aura target ask: a boon hint (Curiosity is Combat Research's, Pump) goes on
// the seat's own creature, a Curse on the opponent's. The opponent's 4/4
// (option 0) outranks the own 2/2 under the plain ranking, so a bot without
// the polarity rule picks it for every hint.
func TestHeuristicBoonAuraTargetsOwnCreature(t *testing.T) {
	for _, tc := range []struct {
		logic string
		want  int // option index
	}{
		{"Curiosity", 1},
		{"Pump", 1},
		{"Curse", 0},
	} {
		d := attachRuleDecision()
		d.TargetEffect.AttachLogic = tc.logic
		got, err := NewBot(1).DecideBoard(context.Background(), attachRuleBoard(), d)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Choices) != 1 || got.Choices[0] != tc.want {
			t.Errorf("%s: choices = %v, want [%d]", tc.logic, got.Choices, tc.want)
		}
		// Precondition: with no hint the same ask takes the opponent's 4/4,
		// so the own-creature picks above are the rule, not the ranking.
		plain := attachRuleDecision()
		plain.TargetEffect = &decision.TargetEffect{API: "Attach"}
		pg, err := NewBot(1).DecideBoard(context.Background(), attachRuleBoard(), plain)
		if err != nil {
			t.Fatal(err)
		}
		if len(pg.Choices) != 1 || pg.Choices[0] != 0 {
			t.Fatalf("hintless choices = %v, want [0]; the fixture no longer discriminates", pg.Choices)
		}
	}
}
