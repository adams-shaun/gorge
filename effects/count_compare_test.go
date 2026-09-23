package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// Tests for the Count$Compare head in evalCountBody. The grammar is
// `Compare <Name> <OP><threshold>.<ifTrue>.<ifFalse>`: <Name> resolves the
// compared value (SVar body, else inline expression), the threshold and the
// branches are each an operand (integer literal, SVar name, or inline
// expression). Argument-less forms (Count$Compare TronCheck) fail closed to
// zero; the two non-literal-threshold shapes (GEMePlus.3.2, LTZ.2.0) are
// covered by count_compare_threshold_test.go.

// addGraveyardSpell mints a spell fixture into seat 0's graveyard, the way
// filter_test.go's board() places its myInstant/mySorcery.
func addGraveyardSpell(t *testing.T, h *fakeHost, name string) state.ObjID {
	t.Helper()
	c, diags := cards.ParseBytes("t.txt", []byte("Name:"+name+"\nManaCost:U\nTypes:Instant\nOracle:x\n"))
	if len(diags) != 0 {
		t.Fatalf("diags: %v", diags)
	}
	c.Link()
	o := h.g.AddObject(c, 0)
	o.Zone = state.ZGraveyard
	h.g.SetZone(state.ZGraveyard, 0, append(h.g.Zone(state.ZGraveyard, 0), o.ID))
	return o.ID
}

func TestCompareHeadSelectsBranchByComparison(t *testing.T) {
	h, c := fixtureHost(t)
	c.SVars = map[string]string{
		// Nissa's Pilgrimage's own SVar table: 2 basic Forests normally, 3
		// with two or more instants/sorceries in the graveyard.
		"X": "Count$Compare Y GE2.3.2",
		"Y": "Count$ValidGraveyard Instant.YouOwn,Sorcery.YouOwn",
	}
	if got := EvalCount(h, c, "Count$Compare Y GE2.3.2"); got != 2 {
		t.Errorf("empty spell graveyard: Compare Y GE2.3.2 = %d, want 2", got)
	}
	// The corpus path: ChangeNum$ X reaches the Compare body through Num's
	// SVar indirection, not through a Count$X head (EvalCount has no head X).
	if got := Num(h, c, sa(t, "SP$ Draw | NumCards$ X"), "NumCards", 9); got != 2 {
		t.Errorf("Nissa's X via SVar indirection = %d, want 2", got)
	}
	addGraveyardSpell(t, h, "Trick")
	if got := Num(h, c, sa(t, "SP$ Draw | NumCards$ X"), "NumCards", 9); got != 2 {
		t.Errorf("one instant in graveyard: X = %d, want 2 (threshold not reached)", got)
	}
	addGraveyardSpell(t, h, "Ritual")
	if got := Num(h, c, sa(t, "SP$ Draw | NumCards$ X"), "NumCards", 9); got != 3 {
		t.Errorf("two instants in graveyard: X = %d, want 3 (mastery holds)", got)
	}
}

func TestCompareHeadOpsAndThresholds(t *testing.T) {
	h, c := fixtureHost(t)
	c.SVars = map[string]string{"N": "3"}
	cases := []struct {
		expr string
		want int32
	}{
		{"Count$Compare N GE3.7.8", 7}, // equal counts for GE
		{"Count$Compare N GT3.7.8", 8}, // equal does not count for GT
		{"Count$Compare N GT2.7.8", 7}, // strictly above counts for GT
		{"Count$Compare N EQ3.7.8", 7}, // equal counts for EQ
		{"Count$Compare N EQ4.7.8", 8}, // unequal falls to the false branch
		{"Count$Compare N LE3.7.8", 7}, // equal counts for LE
		{"Count$Compare N LE2.7.8", 8}, // above falls to the false branch
		{"Count$Compare N LT3.7.8", 8}, // equal does not count for LT
		{"Count$Compare N LT4.7.8", 7}, // strictly below counts for LT
		{"Count$Compare N GE3.0.0", 0}, // zero-valued branches are real answers
		{"Count$Compare N GE0.7.8", 7}, // zero threshold
	}
	for _, tc := range cases {
		if got := EvalCount(h, c, tc.expr); got != tc.want {
			t.Errorf("%s = %d, want %d", tc.expr, got, tc.want)
		}
	}
}

func TestCompareHeadResolvesSVarNamedBranches(t *testing.T) {
	h, c := fixtureHost(t)
	c.SVars = map[string]string{
		// apocalypse_hydra's own Y: X when X < 5, else X doubled -- both
		// branches name SVars.
		"X": "Count$xPaid",
		"Y": "Count$Compare X LT5.X.Z",
		"Z": "Count$xPaid/Twice",
	}
	c.X = 3
	if got := EvalCount(h, c, "Count$Compare X LT5.X.Z"); got != 3 {
		t.Errorf("X=3: Y (LT5.X.Z with SVar branches) = %d, want 3", got)
	}
	c.X = 5
	if got := EvalCount(h, c, "Count$Compare X LT5.X.Z"); got != 10 {
		t.Errorf("X=5: Y = %d, want 10 (X doubled)", got)
	}
	// my_wealth_will_bury_you's Y: the compared NAME is the X SVar and the
	// ifTrue branch is the same SVar.
	c.SVars["X"] = "Count$Valid Artifact.YouCtrl"
	c.SVars["Y"] = "Count$Compare X GE4.X.4"
	c.X = 0
	if got := EvalCount(h, c, "Count$Compare X GE4.X.4"); got != 4 {
		t.Errorf("Y (GE4.X.4 with no artifacts) = %d, want 4 (false branch)", got)
	}
}

func TestCompareHeadMissingSVarDegradesToZero(t *testing.T) {
	h, c := fixtureHost(t)
	c.SVars = map[string]string{"N": "2"}
	// The compared name is neither an SVar nor an expression: zero, which
	// fails every positive comparison and passes EQ0.
	if got := EvalCount(h, c, "Count$Compare Ghost GE1.5.6"); got != 6 {
		t.Errorf("missing compared SVar: X = %d, want 6 (false branch)", got)
	}
	if got := EvalCount(h, c, "Count$Compare Ghost EQ0.5.6"); got != 5 {
		t.Errorf("missing compared SVar vs EQ0: X = %d, want 5 (true branch)", got)
	}
	// A branch naming a missing SVar degrades to zero.
	if got := EvalCount(h, c, "Count$Compare N GE1.MissingSVar.7"); got != 0 {
		t.Errorf("missing branch SVar: X = %d, want 0", got)
	}
}

func TestCompareHeadExoticFormsFailClosed(t *testing.T) {
	h, c := fixtureHost(t)
	c.SVars = map[string]string{
		// The corpus's argument-less forms and the compare-head-with-no-
		// argument shapes must keep degrading to zero -- no invented
		// semantics. (The two non-literal-threshold forms GEMePlus.3.2 and
		// LTZ.2.0 now EVALUATE; see count_compare_threshold_test.go.)
		"MePlus": "SVar$Me/Plus.4",
		"Z":      "Count$ValidHand Card.YouOwn",
	}
	cases := []string{
		"Count$Compare TronCheck", // argument-less
		"Count$Compare W",         // argument-less
		"Count$Compare ReplacedCard$CardManaCost",
		"Count$Compare", // bare head, no argument at all
	}
	for _, expr := range cases {
		if got := EvalCount(h, c, expr); got != 0 {
			t.Errorf("%s = %d, want 0 (fail closed)", expr, got)
		}
	}
	// The named-branch forms with a parseable threshold still resolve
	// through SVar recursion even when an inner head is unsupported --
	// the_biblioplex's X resolves via its nested Y.
	c.SVars["Y"] = "Count$Compare Z EQ0.1.0"
	if got := EvalCount(h, c, "Count$Compare Z EQ7.1.Y"); got != 1 {
		t.Errorf("biblioplex X = %d, want 1 (nested Y's true branch)", got)
	}
}

func TestCompareHeadSelfReferenceIsDepthCapped(t *testing.T) {
	h, c := fixtureHost(t)
	// A self-referential SVar table must not blow the stack: the depth cap
	// terminates the recursion -- the innermost evaluation degrades to zero
	// (its compared value reads as the cap's zero) and every outer level
	// computes on that deterministically, so the whole table still yields
	// ONE fixed answer rather than diverging. Pinned as deterministic across
	// calls rather than to a specific parity value.
	c.SVars = map[string]string{
		"X": "Count$Compare Y GE1.5.6",
		"Y": "Count$Compare X GE1.5.6",
	}
	got := EvalCount(h, c, "Count$Compare Y GE1.5.6")
	if got != EvalCount(h, c, "Count$Compare Y GE1.5.6") {
		t.Errorf("cyclic SVar table not deterministic: got %d", got)
	}
}

// The PlayerCountPropertyYou$HasPropertyActive head -- Starting Town's ETB
// gate reads SVar:Y:PlayerCountPropertyYou$HasPropertyActive and feeds it to
// Count$Compare Y GE1.Z.4, so X is YourTurns while the controller is the
// active player and 4 off it (tapped only when X > 3). Before the fix the
// head matched no dispatch branch, Y degraded to 0, the Compare took the
// ifFalse branch "4" and the gate held on EVERY turn.
func TestPlayerCountPropertyYouHasPropertyActive(t *testing.T) {
	h, c := fixtureHost(t)

	// The head itself: 1 when the resolving controller is the active player.
	if got, ok := EvalCountOK(h, c, "PlayerCountPropertyYou$HasPropertyActive"); !ok || got != 1 {
		t.Errorf("controller active: head = (%d, %v), want (1, true)", got, ok)
	}
	h.g.Active = 1
	if got, ok := EvalCountOK(h, c, "PlayerCountPropertyYou$HasPropertyActive"); !ok || got != 0 {
		t.Errorf("controller not active: head = (%d, %v), want (0, true)", got, ok)
	}

	// Every other property on the You group, and every other group, stays
	// unresolvable (0, false) -- fail closed, not a fake zero.
	h.g.Active = 0
	for _, head := range []string{
		"PlayerCountPropertyYou$LifeLostThisTurn",
		"PlayerCountPropertyYou$LandsPlayed",
		"PlayerCountPropertyOpponent$HasPropertyActive",
		"PlayerCountPropertywithAtLeast2MoreLandsThanYou$Amount",
	} {
		if got, ok := EvalCountOK(h, c, head); ok {
			t.Errorf("%s resolved to %d, want unresolvable (0, false)", head, got)
		}
	}

	// The mechanical chain through the Compare: on your turn X is Z (the
	// fakeHost's TurnsTaken is 0); off your turn X takes the ifFalse branch 4.
	c.SVars = map[string]string{
		"X": "Count$Compare Y GE1.Z.4",
		"Y": "PlayerCountPropertyYou$HasPropertyActive",
		"Z": "Count$YourTurns",
	}
	if got := EvalCount(h, c, "Count$Compare Y GE1.Z.4"); got != 0 {
		t.Errorf("active controller: X chain = %d, want Z = 0", got)
	}
	h.g.Active = 1
	if got := EvalCount(h, c, "Count$Compare Y GE1.Z.4"); got != 4 {
		t.Errorf("inactive controller: X chain = %d, want the ifFalse branch 4", got)
	}
}
