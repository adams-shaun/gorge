package effects

import (
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// Tests for the Count$Compare THRESHOLD being an operand rather than a
// literal-only field. The grammar is
// `Compare <Name> <OP><threshold>.<ifTrue>.<ifFalse>`: <Name>, the threshold
// and each branch all resolve through the same evalCountOperand (an integer
// literal, an SVar name, or an inline expression). The corpus carries
// exactly two non-literal thresholds -- teachings_of_the_archaics'
// `X:Count$Compare Opp GEMePlus.3.2` and anchor_to_reality's
// `X:Count$Compare Y LTZ.2.0` -- and both used to fail the whole head closed
// to zero. The SVar tables below are copied from the compiled corpus
// (`reg.Lookup`, asserted) so the tests run against the real card scripts,
// not a reconstructed approximation of them.

// corpusCompareSVars loads a card's compiled first-face SVar table and fails
// the test if the card or its Compare body is absent or shaped differently,
// so a corpus move cannot silently turn the test into a no-op.
func corpusCompareSVars(t *testing.T, cardName, name, wantBody string) map[string]string {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	c, ok := reg.Lookup(cardName)
	if !ok {
		t.Fatalf("corpus has no %s", cardName)
	}
	if len(c.Faces) == 0 {
		t.Fatalf("%s has no faces", cardName)
	}
	sv := c.Faces[0].SVars
	if got := sv[name]; got != wantBody {
		t.Fatalf("precondition: %s SVar %s = %q, want %q", cardName, name, got, wantBody)
	}
	clone := make(map[string]string, len(sv))
	for k, v := range sv {
		clone[k] = v
	}
	return clone
}

// TestCompareHeadResolvesSVarThresholdAnchorToReality pins the LTZ form: the
// threshold names an SVar (Z, the sacrificed permanent's mana value) rather
// than a literal. Anchor to Reality's X is 2 ("scry 2") when the remembered
// card's mana value is LESS than the sacrificed permanent's, else 0.
func TestCompareHeadResolvesSVarThresholdAnchorToReality(t *testing.T) {
	sv := corpusCompareSVars(t, "Anchor to Reality", "X", "Count$Compare Y LTZ.2.0")
	// The threshold really is an SVar name (not a literal): assert the
	// precondition the fix depends on, so a future rewrite back to
	// literal-only parsing fails this test at the precondition.
	if _, err := strconv.Atoi(sv["Z"]); err == nil {
		t.Fatalf("precondition: Z = %q parses as a literal -- LTZ is no longer the exotic form", sv["Z"])
	}
	g, ids := board(t)
	h := &fakeHost{g: g}
	// Y is Remembered$CardManaCost: myBear has mana value 2.
	c := &Ctx{
		Controller: 0,
		SVars:      sv,
		Remembered: []state.Target{{Obj: ids["myBear"]}},
	}
	// Precondition: the two compared values actually differ.
	if y := EvalCount(h, c, "Remembered$CardManaCost"); y != 2 {
		t.Fatalf("precondition: Y = %d, want 2 (myBear's mana value)", y)
	}
	// Sacrificed permanent's mana value 1 < Y=2: `Y < Z` is false, ifFalse 0.
	c.Sacrificed = []state.SacrificedInfo{{ManaValue: 1}}
	if z := EvalCount(h, c, "Sacrificed$CardManaCost"); z != 1 {
		t.Fatalf("precondition: Z = %d, want 1", z)
	}
	if got := EvalCount(h, c, "Count$Compare Y LTZ.2.0"); got != 0 {
		t.Fatalf("Y=2, Z=1: LTZ.2.0 = %d, want 0 (2 < 1 is false)", got)
	}
	// Sacrificed mana value 3 > Y=2: `Y < Z` is true, ifTrue 2.
	c.Sacrificed = []state.SacrificedInfo{{ManaValue: 3}}
	if got := EvalCount(h, c, "Count$Compare Y LTZ.2.0"); got != 2 {
		t.Fatalf("Y=2, Z=3: LTZ.2.0 = %d, want 2 (2 < 3 is true)", got)
	}
	// The corpus path: the ChangeNum$ X read reaches the same body through
	// Num's SVar indirection.
	if got := Num(h, c, sa(t, "SP$ Scry | ScryNum$ X"), "ScryNum", 9); got != 2 {
		t.Fatalf("ScryNum$ X via SVar indirection = %d, want 2", got)
	}
}

// TestCompareHeadResolvesSVarThresholdTeachings pins the GEMePlus form: the
// threshold names an SVar (MePlus = SVar$Me/Plus.4) whose inner head
// (CardsInYourHand) this build does not model. By the evaluator's
// degrade-to-0 convention Me reads 0, so the threshold resolves to 4, and
// the comparison still runs instead of the whole head vanishing.
func TestCompareHeadResolvesSVarThresholdTeachings(t *testing.T) {
	sv := corpusCompareSVars(t, "Teachings of the Archaics", "X", "Count$Compare Opp GEMePlus.3.2")
	if _, err := strconv.Atoi(sv["MePlus"]); err == nil {
		t.Fatalf("precondition: MePlus = %q parses as a literal -- GEMePlus is no longer the exotic form", sv["MePlus"])
	}
	h, c := fixtureHost(t)
	c.SVars = sv
	// The threshold is an OPERAND: MePlus resolves to a NONZERO value (4)
	// even though its inner head is unmodelled, which the old literal-only
	// parse could never produce.
	if got := EvalCount(h, c, "SVar$Me/Plus.4"); got != 4 {
		t.Fatalf("precondition: MePlus = %d, want 4 (Me degrades to 0, Plus.4)", got)
	}
	// Opp (PlayerCountOpponents$HighestCardsInHand) is unmodelled too and
	// degrades to 0; 0 >= 4 is false, so the corpus card draws its default 2.
	if got := EvalCount(h, c, "Count$Compare Opp GEMePlus.3.2"); got != 2 {
		t.Fatalf("Opp=0 vs MePlus=4: GEMePlus.3.2 = %d, want 2 (the documented default)", got)
	}
	// Control: point the compared name at the SAME operand as the threshold
	// (a modelled SVar, not the unmodelled corpus Opp), so the comparison's
	// two sides are 4 and 4 and the ifTrue branch is selected. This proves
	// the comparison actually reads the resolved threshold value.
	c.SVars["Opp"] = "SVar$Me/Plus.4"
	if got := EvalCount(h, c, "Count$Compare Opp GEMePlus.3.2"); got != 3 {
		t.Fatalf("Opp=4 vs MePlus=4: GEMePlus.3.2 = %d, want 3 (4 >= 4 holds)", got)
	}
}

// TestCompareHeadRejectsEmptyThreshold preserves the fail-closed grammar for
// a syntactically split comparison with no threshold between the operator and
// its first dot. Empty is neither a literal nor an SVar operand.
func TestCompareHeadRejectsEmptyThreshold(t *testing.T) {
	h, c := fixtureHost(t)
	c.SVars = map[string]string{"N": "2"}
	if got := EvalCount(h, c, "SVar$N"); got != 2 {
		t.Fatalf("precondition: N = %d, want 2", got)
	}
	if trueBranch, falseBranch := EvalCount(h, c, "7"), EvalCount(h, c, "8"); trueBranch == falseBranch {
		t.Fatal("precondition: comparison branches must differ")
	}
	if got := EvalCount(h, c, "Count$Compare N GE.7.8"); got != 0 {
		t.Fatalf("empty threshold = %d, want 0 (fail closed)", got)
	}
}

// TestCompareHeadThresholdOperandAcrossOps is the minimal grammar pin: an
// operand-named threshold is compared at its resolved value, for EVERY one
// of the five ops, with both sides modelled so the branch selected is the
// values', not a constant's.
func TestCompareHeadThresholdOperandAcrossOps(t *testing.T) {
	g, _ := board(t)
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0}
	// Board: 2 creatures controlled by seat 0 (filter_test.go board).
	c.SVars = map[string]string{
		// Threshold operand: seat 0's creatures = 2.
		"Th": "Count$Valid Creature.YouCtrl",
		// Compared value, set per case below.
		"V": "2",
	}
	if got := EvalCount(h, c, "SVar$Th"); got != 2 {
		t.Fatalf("precondition: Th = %d, want 2 (myBear + myFlier)", got)
	}
	cases := []struct {
		value string
		expr  string
		want  int32
	}{
		{"2", "Count$Compare V GETh.7.8", 7}, // 2>=2 true
		{"2", "Count$Compare V GTTh.7.8", 8}, // 2>2 false
		{"2", "Count$Compare V EQTh.7.8", 7}, // 2==2 true
		{"2", "Count$Compare V LETh.7.8", 7}, // 2<=2 true
		{"2", "Count$Compare V LTTh.7.8", 8}, // 2<2 false
		{"3", "Count$Compare V GTTh.7.8", 7}, // 3>2 true
		{"3", "Count$Compare V LTTh.7.8", 8}, // 3<2 false
	}
	for _, tc := range cases {
		c.SVars["V"] = tc.value
		if got := EvalCount(h, c, tc.expr); got != tc.want {
			t.Errorf("%s with V=%s: = %d, want %d", tc.expr, tc.value, got, tc.want)
		}
	}
}
