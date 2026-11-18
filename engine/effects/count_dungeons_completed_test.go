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

// DungeonsCompleted$Amount (CR 309.7) reads the resolving controller's
// completed-dungeon count: Undermountain Adventurer's "Count$Compare X GE1.6.2"
// adds six {G} once its controller has completed a dungeon, {G}{G} before.
func TestDungeonsCompletedAmountReadsTheControllersCount(t *testing.T) {
	h, c := fixtureHost(t)
	if h.g.Players[0].CompletedDungeons != 0 || h.g.Players[1].CompletedDungeons != 0 {
		t.Fatalf("precondition: the fixture's seats start with no completed dungeon")
	}
	if n, ok := EvalCountOK(h, c, "DungeonsCompleted$Amount"); !ok || n != 0 {
		t.Fatalf("no dungeon completed = (%d, %v), want (0, true)", n, ok)
	}
	h.g.Players[1].CompletedDungeons = 3 // the other seat's count never answers for seat 0
	if n, _ := EvalCountOK(h, c, "DungeonsCompleted$Amount"); n != 0 {
		t.Fatalf("seat 1's completed dungeons answered for seat 0: %d", n)
	}
	h.g.Players[0].CompletedDungeons = 2
	if n, ok := EvalCountOK(h, c, "DungeonsCompleted$Amount"); !ok || n != 2 {
		t.Fatalf("two completed = (%d, %v), want (2, true)", n, ok)
	}
	if _, ok := EvalCountOK(h, c, "DungeonsCompleted$Other"); ok {
		t.Fatalf("an unmodelled DungeonsCompleted$ property resolved")
	}
}
