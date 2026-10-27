package effects

import "testing"

// TestYourTeamCtrlMatchesLikeYouCtrl pins the no-teams reading of Forge's
// YourTeamCtrl (engine/rules/turn.go: gorge models no Two-Headed Giant
// teams, so a player's team is that player alone): the textual matcher, the
// compiled predicate term and Player.YourTeam must all answer exactly as
// their "you" spelling does, for a permanent you control and one an opponent
// controls. Before the aliases the predicate was unknown and every Battlebond
// "your team controls" card matched nothing.
func TestYourTeamCtrlMatchesLikeYouCtrl(t *testing.T) {
	g, ids := board(t)
	// The compiled path is selected by PredicatePrograms; the plain context
	// is the textual matcher. Both must give the same answer.
	textual := SpecContext{You: 0, Source: ids["myBear"]}
	ps := CompilePredicatePrograms([]string{"Creature.YourTeamCtrl", "Creature.YouCtrl"})
	compiled := textual
	compiled.PredicatePrograms = ps
	for _, tc := range []struct {
		spec, obj string
		want      bool
	}{
		{"Creature.YourTeamCtrl", "myBear", true},
		{"Creature.YouCtrl", "myBear", true},
		{"Creature.YourTeamCtrl", "theirBig", false},
		{"Creature.YouCtrl", "theirBig", false},
	} {
		id := ids[tc.obj]
		if got := MatchesSpecCtx(g, tc.spec, id, textual); got != tc.want {
			t.Errorf("textual MatchesSpecCtx(%q, %s) = %v, want %v", tc.spec, tc.obj, got, tc.want)
		}
		if got := MatchesSpecCtx(g, tc.spec, id, compiled); got != tc.want {
			t.Errorf("compiled MatchesSpecCtx(%q, %s) = %v, want %v", tc.spec, tc.obj, got, tc.want)
		}
		// A definite compiled answer, not the Maybe fallback: the term
		// really is in the compiled table.
		if got := ps.Evaluate(tc.spec, g, g.Obj(id), compiled); got == PredicateMaybe {
			t.Errorf("compiled Evaluate(%q, %s) = Maybe; the term is not compiled", tc.spec, tc.obj)
		}
	}
	if !MatchesPlayerSpecCtx(g, "Player.YourTeam", 0, 0, PlayerSpecCtx{}) {
		t.Error("Player.YourTeam did not match the controller")
	}
	if MatchesPlayerSpecCtx(g, "Player.YourTeam", 1, 0, PlayerSpecCtx{}) {
		t.Error("Player.YourTeam matched an opponent; without teams only the player themself is on their team")
	}
}
