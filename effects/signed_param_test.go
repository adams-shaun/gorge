package effects

import "testing"

// These pins cover Num's sign handling on NON-literal parameter values.
// Forge writes a stat direction as a sign on the value ("NumAtt$ +X" --
// Goblin Piledriver), and before the sign strip a signed non-literal failed
// every fallback (Atoi errors, the SVar lookup missed the sign-prefixed key,
// the X comparison was false) and degraded to zero. A signed literal was
// already correct (Atoi eats the sign) and stays so.

func TestNumStripsSignFromSVarReference(t *testing.T) {
	g, _ := board(t)
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0, SVars: map[string]string{
		"X": "Count$Valid Creature",     // the board fixture has three creatures
		"Y": "Count$PlayerCountPlayers", // two seats
	}}
	if got := Num(h, c, sa(t, "SP$ Pump | NumAtt$ +X"), "NumAtt", 0); got != 3 {
		t.Errorf("NumAtt$ +X = %d, want +3", got)
	}
	if got := Num(h, c, sa(t, "SP$ Pump | NumAtt$ -X"), "NumAtt", 0); got != -3 {
		t.Errorf("NumAtt$ -X = %d, want -3 (the sign must flip, not zero)", got)
	}
	if got := Num(h, c, sa(t, "SP$ Pump | NumDef$ +Y"), "NumDef", 0); got != 2 {
		t.Errorf("NumDef$ +Y = %d, want +2", got)
	}
	if got := Num(h, c, sa(t, "SP$ Pump | NumDef$ -Y"), "NumDef", 0); got != -2 {
		t.Errorf("NumDef$ -Y = %d, want -2", got)
	}
}

func TestNumStripsSignFromXAndInlineExpressions(t *testing.T) {
	g, _ := board(t)
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0}
	c.X = 4
	if got := Num(h, c, sa(t, "SP$ Pump | NumAtt$ +X"), "NumAtt", 0); got != 4 {
		t.Errorf("NumAtt$ +X (no SVar, Ctx.X) = %d, want +4", got)
	}
	if got := Num(h, c, sa(t, "SP$ Pump | NumDef$ -X"), "NumDef", 0); got != -4 {
		t.Errorf("NumDef$ -X (no SVar, Ctx.X) = %d, want -4", got)
	}
	if got := Num(h, c, sa(t, "SP$ Pump | NumAtt$ +Count$Valid Creature"), "NumAtt", 0); got != 3 {
		t.Errorf("NumAtt$ +Count$... = %d, want +3", got)
	}
	if got := Num(h, c, sa(t, "SP$ Pump | NumDef$ -Count$PlayerCountPlayers/Minus1"), "NumDef", 0); got != -1 {
		t.Errorf("NumDef$ -Count$.../Minus1 = %d, want -1", got)
	}
}

func TestNumSignedLiteralsUnchanged(t *testing.T) {
	h, c := fixtureHost(t)
	if got := Num(h, c, sa(t, "SP$ Pump | NumAtt$ +2"), "NumAtt", 0); got != 2 {
		t.Errorf("NumAtt$ +2 = %d, want 2", got)
	}
	if got := Num(h, c, sa(t, "SP$ Pump | NumDef$ -2"), "NumDef", 0); got != -2 {
		t.Errorf("NumDef$ -2 = %d, want -2", got)
	}
}

func TestNumSignedUnresolvableStillDegradesToZero(t *testing.T) {
	h, c := fixtureHost(t)
	c.SVars = map[string]string{"X": "Count$ThisIsNotReal"}
	if got := Num(h, c, sa(t, "SP$ Pump | NumAtt$ +X"), "NumAtt", 9); got != 0 {
		t.Errorf("signed SVar naming an unmodelled head = %d, want 0", got)
	}
	if got := Num(h, c, sa(t, "SP$ Pump | NumAtt$ +"), "NumAtt", 9); got != 0 {
		t.Errorf("a lone sign is not a value: got %d, want 0", got)
	}
	if got := Num(h, c, sa(t, "SP$ Pump | NumAtt$ -"), "NumAtt", 9); got != 0 {
		t.Errorf("a lone sign is not a value: got %d, want 0", got)
	}
}
