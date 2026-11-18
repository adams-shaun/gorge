package effects

import "testing"

// DungeonsCompleted$Amount reads the resolving controller's completed
// dungeons (CR 309.7); Undermountain Adventurer's Count$Compare X GE1.6.2
// turns it into "six {G} instead of {G}{G}".
func TestDungeonsCompletedAmount(t *testing.T) {
	h, c := fixtureHost(t)
	c.SVars = map[string]string{
		"X": "DungeonsCompleted$Amount",
		"Y": "Count$Compare X GE1.6.2",
	}
	if got, ok := EvalCountOK(h, c, "DungeonsCompleted$Amount"); !ok || got != 0 {
		t.Fatalf("no dungeon completed: (%d, %v), want (0, true)", got, ok)
	}
	if got := EvalCount(h, c, "Count$Compare X GE1.6.2"); got != 2 {
		t.Fatalf("no dungeon completed: Y = %d, want 2", got)
	}
	h.g.Players[c.Controller].CompletedDungeons = 1
	if got := EvalCount(h, c, "Count$Compare X GE1.6.2"); got != 6 {
		t.Fatalf("one dungeon completed: Y = %d, want 6", got)
	}
	if _, ok := EvalCountOK(h, c, "DungeonsCompleted$Other"); ok {
		t.Fatal("an unmodelled DungeonsCompleted property must stay unresolvable")
	}
}
