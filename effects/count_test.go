package effects

import (
	"math"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

func TestNumReadsLiterals(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{}
	if got := Num(h, c, sa(t, "SP$ DealDamage | NumDmg$ 3"), "NumDmg", 1); got != 3 {
		t.Errorf("NumDmg = %d", got)
	}
	if got := Num(h, c, sa(t, "SP$ DealDamage"), "NumDmg", 7); got != 7 {
		t.Errorf("missing key should return the default, got %d", got)
	}
	if got := Num(h, c, sa(t, "SP$ DealDamage | NumDmg$ -2"), "NumDmg", 0); got != -2 {
		t.Errorf("negative literal = %d", got)
	}
}

func TestNumFollowsSVarIndirection(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{SVars: map[string]string{"Y": "Count$ThisIsNotReal"}}
	// An SVar naming an unmodelled Count$ head resolves to zero, not garbage.
	if got := Num(h, c, sa(t, "SP$ Draw | NumCards$ Y"), "NumCards", 9); got != 0 {
		t.Errorf("unknown Count$ head = %d, want 0", got)
	}
	c.SVars["Z"] = "Count$xPaid"
	c.X = 4
	if got := Num(h, c, sa(t, "SP$ Draw | NumCards$ Z"), "NumCards", 0); got != 4 {
		t.Errorf("Count$xPaid = %d, want 4", got)
	}
}

// TestNumEvaluatesInlineCountExpression locks in Task 17's Num extension:
// Storm's expansion puts Count$ThisTurnCast/Minus1 inline in the Amount$
// param (cards/keywords.go), not behind an SVar name -- before the extension
// Num mistook that literal for an SVar lookup, failed it, and judged the
// whole amount zero, silencing every Storm copy.
func TestNumEvaluatesInlineCountExpression(t *testing.T) {
	h, c := fixtureHost(t)
	got := Num(h, c, sa(t, "SP$ Draw | Amount$ Count$PlayerCountPlayers/Minus1"), "Amount", 9)
	if got != 1 {
		t.Errorf("Num inline Count$ = %d, want 1 (2 players minus 1)", got)
	}
}

func TestEvalCountValidCountsTheBattlefield(t *testing.T) {
	g, _ := board(t)
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0}
	if got := EvalCount(h, c, "Count$Valid Creature"); got != 3 {
		t.Errorf("Valid Creature = %d, want 3", got)
	}
	if got := EvalCount(h, c, "Count$Valid Creature.YouCtrl"); got != 2 {
		t.Errorf("Valid Creature.YouCtrl = %d, want 2", got)
	}
	if got := EvalCount(h, c, "Count$Valid Land.YouCtrl"); got != 1 {
		t.Errorf("Valid Land.YouCtrl = %d, want 1", got)
	}
}

func TestEvalCountValidSumsAPropertySuffix(t *testing.T) {
	g, _ := board(t)
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0}
	// You control the Bear (2/2, MV 2), the Flier (1/1, MV 2) and the
	// Giant (5/5, MV 5) is the opponent's: the sum property suffix sums
	// over the MATCHES only.
	if got := EvalCount(h, c, "Count$Valid Creature.YouCtrl$CardPower"); got != 3 {
		t.Errorf("Creature.YouCtrl$CardPower = %d, want 3", got)
	}
	if got := EvalCount(h, c, "Count$Valid Creature.YouCtrl$CardToughness"); got != 3 {
		t.Errorf("Creature.YouCtrl$CardToughness = %d, want 3", got)
	}
	if got := EvalCount(h, c, "Count$Valid Creature$CardManaCost"); got != 9 {
		t.Errorf("Creature$CardManaCost = %d, want 9 (Bear+Giant+Flier: 2+5+2)", got)
	}
	if got := EvalCount(h, c, "Count$Valid Creature.YouCtrl$CardManaCost"); got != 4 {
		t.Errorf("Creature.YouCtrl$CardManaCost = %d, want 4", got)
	}
	// An unrecognised property keeps the old whole-token spec read: it
	// never matched anything, so it stays a zero count, not a widening.
	// (The Greatest/Least reductions ARE read now -- see
	// TestEvalCountValidExtremeProperties below.)
	if got := EvalCount(h, c, "Count$Valid Creature$DifferentCardPower"); got != 0 {
		t.Errorf("DifferentCardPower token = %d, want 0 (out of scope, fail closed)", got)
	}
}

// TestEvalCountValidExtremeProperties pins the four extreme-reduction
// property suffixes: the MAXIMUM (Greatest*) or MINIMUM (Least*) of the
// property over the matches, not a sum. Several different values, because
// the defect being fixed is a silent zero and a single-value test can pass
// by coincidence.
func TestEvalCountValidExtremeProperties(t *testing.T) {
	g, _ := board(t)
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0}
	// Board: controller 0 has Bear 2/2 (MV 2) and Flier 1/1 (MV 2);
	// controller 1 has the Giant 5/5 (MV 5).
	cases := []struct {
		expr string
		want int32
	}{
		{"Count$Valid Creature$GreatestCardPower", 5},
		{"Count$Valid Creature.YouCtrl$GreatestCardPower", 2},
		{"Count$Valid Creature.YouCtrl$LeastCardPower", 1},
		{"Count$Valid Creature$LeastCardPower", 1},
		{"Count$Valid Creature$GreatestCardToughness", 5},
		{"Count$Valid Creature.YouCtrl$GreatestCardToughness", 2},
		// Bear MV 2, Giant MV 5: the greatest mana value, not the sum (9).
		{"Count$Valid Creature$GreatestCardManaCost", 5},
		{"Count$Valid Creature.YouCtrl$GreatestCardManaCost", 2},
		// Zero matches -> 0, never an int-min/max sentinel.
		{"Count$Valid Creature.YouCtrl+NonExistent$GreatestCardPower", 0},
		{"Count$Valid Creature.YouCtrl+NonExistent$LeastCardPower", 0},
	}
	for _, tc := range cases {
		if got := EvalCount(h, c, tc.expr); got != tc.want {
			t.Errorf("%s = %d, want %d", tc.expr, got, tc.want)
		}
	}
	// A second board strength so a hard-coded 5 cannot pass both: the
	// DERIVED read must follow +1/+1 counters, not the printed face.
	for _, id := range g.Zone(state.ZBattlefield, 0) {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Bear" {
			o.AddCounter("P1P1", 5)
		}
	}
	if got := EvalCount(h, c, "Count$Valid Creature.YouCtrl$GreatestCardPower"); got != 7 {
		t.Errorf("Creature.YouCtrl$GreatestCardPower after counters = %d, want 7", got)
	}
}

func TestEvalCountValidCountsDistinctColors(t *testing.T) {
	g, _ := board(t)
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0}
	// Controller 0's board: Bear (G), Flier (U), Aura (W), Walker (U),
	// Siege (R) plus a colourless Mountain and a colourless Relic; the
	// Giant (R) is the opponent's. DISTINCT colours among the matches,
	// not a sum over permanents: 4 (W,U,R,G), the Walker's second U
	// counted once.
	if got := EvalCount(h, c, "Count$Valid Permanent.YouCtrl$Colors"); got != 4 {
		t.Errorf("Permanent.YouCtrl$Colors = %d, want 4", got)
	}
	if got := EvalCount(h, c, "Count$Valid Creature.YouCtrl$Colors"); got != 2 {
		t.Errorf("Creature.YouCtrl$Colors = %d, want 2 (G+U)", got)
	}
	// The opponent's permanents never enter a YouCtrl count (their Giant
	// is also R, so an unscoping bug would still show 4).
	if got := EvalCount(h, c, "Count$Valid Permanent.YouCtrl$Colors"); got != 4 {
		t.Errorf("re-read Permanent.YouCtrl$Colors = %d, want 4", got)
	}
	// A spec matching only colourless permanents counts zero colours.
	if got := EvalCount(h, c, "Count$Valid Land$Colors"); got != 0 {
		t.Errorf("Land$Colors = %d, want 0 (colourless Mountain)", got)
	}
	// A compound spec still matches; the Legendary Walker alone is U.
	if got := EvalCount(h, c, "Count$Valid Permanent.YouCtrl+Legendary$Colors"); got != 1 {
		t.Errorf("Permanent.YouCtrl+Legendary$Colors = %d, want 1 (U)", got)
	}
	// The corpus's one op suffix (happily_ever_after) parses and does not
	// bind at 5 on a four-colour board...
	if got := EvalCount(h, c, "Count$Valid Permanent.YouCtrl$Colors/LimitMax.5"); got != 4 {
		t.Errorf("Colors/LimitMax.5 = %d, want 4", got)
	}
	// ...and clamps when it does bind.
	if got := EvalCount(h, c, "Count$Valid Permanent.YouCtrl$Colors/LimitMax.2"); got != 2 {
		t.Errorf("Colors/LimitMax.2 = %d, want 2 (clamped)", got)
	}
	// An op suffix Colors does not READ (Bogus.3) follows applyCountOp's
	// convention for every unknown op -- ignored, so the plain Colors count
	// stands -- and the still-out-of-scope properties keep the whole-token
	// fail-closed read.
	if got := EvalCount(h, c, "Count$Valid Permanent.YouCtrl$Colors/Bogus.3"); got != 4 {
		t.Errorf("Colors/Bogus.3 = %d, want 4 (unknown op ignored, plain Colors)", got)
	}
	if got := EvalCount(h, c, "Count$Valid Creature$DifferentCardPower"); got != 0 {
		t.Errorf("DifferentCardPower token = %d, want 0 (out of scope, fail closed)", got)
	}
}

func TestEvalCountZoneScopedForms(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	// Move a creature to the graveyard and one to hand.
	moveTo(g, ids["myBear"], state.ZGraveyard)
	moveTo(g, ids["myFlier"], state.ZHand)
	c := &Ctx{Controller: 0}
	if got := EvalCount(h, c, "Count$ValidGraveyard Creature.YouOwn"); got != 1 {
		t.Errorf("graveyard creatures = %d, want 1", got)
	}
	if got := EvalCount(h, c, "Count$ValidHand Card.YouOwn"); got != 1 {
		t.Errorf("hand cards = %d, want 1", got)
	}
}

func TestEvalCountPlayerAndLifeForms(t *testing.T) {
	g, _ := board(t)
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0}
	g.Players[0].Life = 13
	if got := EvalCount(h, c, "Count$YourLifeTotal"); got != 13 {
		t.Errorf("YourLifeTotal = %d", got)
	}
	if got := EvalCount(h, c, "Count$PlayerCountPlayers"); got != 2 {
		t.Errorf("PlayerCountPlayers = %d", got)
	}
	if got := EvalCount(h, c, "Count$PlayerCountOpponents"); got != 1 {
		t.Errorf("PlayerCountOpponents = %d", got)
	}
}

func TestEvalCountArithmeticSuffix(t *testing.T) {
	g, _ := board(t)
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0}
	// Forge appends ".Plus1", ".Minus1", ".Twice" and similar to a count.
	if got := EvalCount(h, c, "Count$Valid Creature.YouCtrl/Plus1"); got != 3 {
		t.Errorf("Plus1 = %d, want 3", got)
	}
	if got := EvalCount(h, c, "Count$Valid Creature.YouCtrl/Minus1"); got != 1 {
		t.Errorf("Minus1 = %d, want 1", got)
	}
	if got := EvalCount(h, c, "Count$Valid Creature.YouCtrl/Twice"); got != 4 {
		t.Errorf("Twice = %d, want 4", got)
	}
}

// TestCountOpsDoNotOverflow is Task 20's hygiene guard for applyCountOp: the
// /Op arithmetic (Plus/Minus/Times/Twice/Negative) was computed in raw int32,
// where MaxInt32 doubled, negated or nudged silently wrapped. It must now run
// in int64 and clamp to the int32 range so a huge Count$ can neither overflow
// to a sign-flipped value nor panic.
func TestCountOpsDoNotOverflow(t *testing.T) {
	if applyCountOp(math.MaxInt32, "Twice") != math.MaxInt32 ||
		applyCountOp(math.MinInt32, "Negative") != math.MaxInt32 ||
		applyCountOp(math.MaxInt32, "Plus5") != math.MaxInt32 {
		t.Fatal("count ops overflow")
	}
}

// TestCountOpsDivisionRoundings is the table guard for Forge's AmountOperators
// division family. Every spelling the corpus carries must divide by its named
// divisor with the right rounding direction; before the Divide arm landed
// DivideEvenlyUp (and bare Divide) fell through applyCountOp untouched, so
// Legate Lanius's "a tenth, rounded up" read as the whole count.
func TestCountOpsDivisionRoundings(t *testing.T) {
	cases := []struct {
		in   int32
		op   string
		want int32
	}{
		// DivideEvenlyUp = ceil (Legate Lanius's real corpus suffix).
		{0, "DivideEvenlyUp.10", 0},
		{1, "DivideEvenlyUp.10", 1},
		{9, "DivideEvenlyUp.10", 1},
		{10, "DivideEvenlyUp.10", 1},
		{11, "DivideEvenlyUp.10", 2},
		{20, "DivideEvenlyUp.10", 2},
		{21, "DivideEvenlyUp.10", 3},
		{7, "DivideEvenlyUp.2", 4},
		// DivideEvenlyDown = floor (the pre-existing arm's real suffixes).
		{0, "DivideEvenlyDown.2", 0},
		{1, "DivideEvenlyDown.2", 0},
		{2, "DivideEvenlyDown.2", 1},
		{3, "DivideEvenlyDown.2", 1},
		{35, "DivideEvenlyDown.5", 7},
		{34, "DivideEvenlyDown.7", 4},
		// DivideEvenly / bare Divide = Forge's default, floor.
		{9, "DivideEvenly.4", 2},
		{8, "DivideEvenly.4", 2},
		{9, "Divide.4", 2},
		{8, "Divide.4", 2},
		// A negative operand floors toward negative infinity, not toward zero
		// (the truncation note the old DivideEvenlyDown arm carried).
		{-3, "DivideEvenlyDown.2", -2},
		{-3, "DivideEvenlyUp.2", -1},
		// A missing / non-numeric / non-positive divisor leaves the value
		// unchanged rather than dividing by zero.
		{7, "DivideEvenlyUp", 7},
		{7, "DivideEvenlyDown.0", 7},
		{7, "DivideEvenlyDown.-2", 7},
		{7, "DivideEvenlyDown.NumOpps", 7},
		// The non-division ops are untouched by the new arm.
		{3, "Plus1", 4},
		{3, "Minus1", 2},
		{3, "Twice", 6},
		{3, "HalfUp", 2},
		{3, "HalfDown", 1},
	}
	for _, tc := range cases {
		if got := applyCountOp(tc.in, tc.op); got != tc.want {
			t.Errorf("applyCountOp(%d, %q) = %d, want %d", tc.in, tc.op, got, tc.want)
		}
	}
}

func TestNumGuardsNilCtx(t *testing.T) {
	h := newHost(t, 2)
	// Nil Ctx should not panic; it's treated as an empty Ctx.
	if got := Num(h, nil, sa(t, "SP$ X | NumDmg$ 5"), "NumDmg", 9); got != 5 {
		t.Errorf("literal with nil Ctx = %d, want 5", got)
	}
	if got := Num(h, nil, sa(t, "SP$ X | NumDmg$ Y"), "NumDmg", 7); got != 0 {
		t.Errorf("SVar reference with nil Ctx = %d, want 0 (default)", got)
	}
}

func TestEvalCountGuardsNilHostAndCtx(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{Controller: 0}
	// Nil Host should not panic; it returns the default.
	if got := EvalCount(nil, c, "Count$YourLifeTotal"); got != 0 {
		t.Errorf("nil Host = %d, want 0", got)
	}
	// Nil Ctx should not panic; it returns the default.
	if got := EvalCount(h, nil, "Count$YourLifeTotal"); got != 0 {
		t.Errorf("nil Ctx = %d, want 0", got)
	}
}

func TestEvalCountGuardsOutOfRangeController(t *testing.T) {
	g, _ := board(t)
	h := &fakeHost{g: g}
	// Controller out of range (>= len(Players)) should return 0, not panic.
	c := &Ctx{Controller: state.PlayerID(len(g.Players))}
	if got := EvalCount(h, c, "Count$YourLifeTotal"); got != 0 {
		t.Errorf("out-of-range Controller = %d, want 0", got)
	}
	// Large out-of-range Controller should also return 0.
	c.Controller = state.PlayerID(255)
	if got := EvalCount(h, c, "Count$YourLifeTotal"); got != 0 {
		t.Errorf("large out-of-range Controller = %d, want 0", got)
	}
}

func TestSetSVarsCopiesTheMap(t *testing.T) {
	c := &Ctx{}
	original := map[string]string{"A": "Count$xPaid", "B": "Count$YourLifeTotal"}
	SetSVars(c, original)

	// Verify the SVar was copied, not aliased.
	if c.SVars["A"] != "Count$xPaid" {
		t.Errorf("SVars copy failed: A = %q", c.SVars["A"])
	}

	// Mutate the original map after the call.
	original["A"] = "Count$ModifiedAfter"
	original["C"] = "Count$NewEntry"

	// The copy in c.SVars should be unaffected.
	if c.SVars["A"] != "Count$xPaid" {
		t.Errorf("original mutation affected the copy: A = %q", c.SVars["A"])
	}
	if _, ok := c.SVars["C"]; ok {
		t.Errorf("new entry in original appeared in copy")
	}

	// Test that nil input leaves SVars nil (not an empty map).
	c2 := &Ctx{}
	SetSVars(c2, nil)
	if c2.SVars != nil {
		t.Errorf("nil input should leave SVars nil, got %v", c2.SVars)
	}
}

func TestNumSelfReferencingSVar(t *testing.T) {
	h := newHost(t, 2)
	// An SVar that refers to itself should terminate and return 0.
	c := &Ctx{SVars: map[string]string{"Self": "Count$Self"}}
	if got := Num(h, c, sa(t, "SP$ X | NumDmg$ Self"), "NumDmg", 9); got != 0 {
		t.Errorf("self-referencing SVar = %d, want 0", got)
	}
}

func TestNumCyclicSVars(t *testing.T) {
	h := newHost(t, 2)
	// Two SVars referring to each other should terminate and return 0.
	c := &Ctx{SVars: map[string]string{
		"A": "Count$B",
		"B": "Count$A",
	}}
	if got := Num(h, c, sa(t, "SP$ X | NumDmg$ A"), "NumDmg", 9); got != 0 {
		t.Errorf("cyclic SVar A = %d, want 0", got)
	}
	if got := Num(h, c, sa(t, "SP$ X | NumDmg$ B"), "NumDmg", 9); got != 0 {
		t.Errorf("cyclic SVar B = %d, want 0", got)
	}
}

func TestCountCardCountersKickedAndTimes(t *testing.T) {
	h, c := fixtureHost(t)
	src := h.Game().Obj(c.Source)
	src.AddCounter("CHARGE", 3)
	if n := EvalCount(h, c, "Count$CardCounters.CHARGE"); n != 3 {
		t.Fatalf("CardCounters.CHARGE = %d", n)
	}
	if n := EvalCount(h, c, "Count$CardCounters.P1P1"); n != 0 {
		t.Fatalf("absent counter kind = %d", n)
	}
	if n := EvalCount(h, c, "Count$Kicked.4.0"); n != 0 {
		t.Fatalf("not kicked = %d", n)
	}
	src.CastFlags = state.FlagKicked
	if n := EvalCount(h, c, "Count$Kicked.4.0"); n != 4 {
		t.Fatalf("kicked = %d", n)
	}
	if n := EvalCount(h, c, "Count$CardCounters.CHARGE/Times.2"); n != 6 {
		t.Fatalf("Times.2 = %d", n)
	}
}

// moveTo is a test helper: relocate an object without going through events.
func moveTo(g *state.Game, id state.ObjID, z state.Zone) {
	o := g.Obj(id)
	src := g.Zone(o.Zone, o.Owner)
	out := src[:0:0]
	for _, x := range src {
		if x != id {
			out = append(out, x)
		}
	}
	g.SetZone(o.Zone, o.Owner, out)
	g.SetZone(z, o.Owner, append(g.Zone(z, o.Owner), id))
	o.Zone = z
}

var _ = cards.Card{}

// TestPlayerCountExtremePropertiesFailUnresolvable pins the r2 review's
// fail-direction fix on the PlayerCount wrapper: the two life extremes
// (LowestLifeTotal/HighestLifeTotal) and the count/counted-quantity extremes
// (HighestValid/LowestValid over any Count$ zone, plus the
// HighestLifeLostThisTurn pair) are evaluated; ANY other property — e.g.
// HighestCardsInHand — reports (0, false), so a gate over one fails per its
// caller's documented direction instead of silently enforcing a fake zero.
func TestPlayerCountExtremePropertiesFailUnresolvable(t *testing.T) {
	g, _ := board(t)
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0}
	g.Players[0].Life = 13
	g.Players[1].Life = 7
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountOpponents$LowestLifeTotal"); !ok || got != 7 {
		t.Fatalf("LowestLifeTotal = (%d, %v), want (7, true)", got, ok)
	}
	if got, ok := EvalCountOK(h, c, "Count$PlayerCountPlayers$HighestLifeTotal"); !ok || got != 13 {
		t.Fatalf("HighestLifeTotal = (%d, %v), want (13, true)", got, ok)
	}
	for _, body := range []string{
		"Count$PlayerCountOpponents$HighestCardsInHand",
		"Count$PlayerCountPlayers$LowestCardsInHand",
	} {
		if got, ok := EvalCountOK(h, c, body); ok {
			t.Fatalf("%s reported EVALUATED as %d -- an unmodelled property must fail unresolvable, not enforce a fake zero", body, got)
		}
	}
}

// TestChosenNumberHeadReadsTheFrozenBinding locks the Count$ChosenNumber
// head (task wildgrowth1): the head reads Ctx.ChosenNumber -- the
// Effect-created replacement's SetChosenNumber$ binding rules' replCtx
// threads in -- and its VERDICT is the bound flag. A bound context evaluates
// (including a bound zero, torgal with no Dogs); an UNBOUND context stays
// unresolved, so the Choose-event population (whose binding lives on
// state.Object.ChosenNumber, never on Ctx) keeps its pre-wildgrowth fail
// direction at every EvalCountOK consumer instead of enforcing a meaningless
// zero.
func TestChosenNumberHeadReadsTheFrozenBinding(t *testing.T) {
	h := newHost(t, 2)
	bound := &Ctx{ChosenNumber: 5, ChosenNumberBound: true}
	if n, ok := EvalCountOK(h, bound, "Count$ChosenNumber"); !ok || n != 5 {
		t.Errorf("bound Count$ChosenNumber = (%d, %v), want (5, true)", n, ok)
	}
	// A bound ZERO is a legitimate binding, not a failed one.
	boundZero := &Ctx{ChosenNumberBound: true}
	if n, ok := EvalCountOK(h, boundZero, "Count$ChosenNumber"); !ok || n != 0 {
		t.Errorf("bound-zero Count$ChosenNumber = (%d, %v), want (0, true)", n, ok)
	}
	// Unbound: UNRESOLVED. The value-true verdict of the first draft flipped
	// every EvalCountOK consumer for the Choose-event cards (void's
	// Artifact.cmcEQX DestroyAll matched MV-0; plague_of_vermin's GE1 SVar
	// gate enforced 0 fail-closed) -- the verdict must stay false here.
	unbound := &Ctx{}
	if n, ok := EvalCountOK(h, unbound, "Count$ChosenNumber"); ok {
		t.Errorf("unbound Count$ChosenNumber = (%d, %v), want unresolved (0, false)", n, ok)
	}
}

// TestThisTurnEnteredGraveyardCountsPermanentBase pins the Defect-2 fix in
// countEntered: a spec's filter must be evaluated in the entry's DESTINATION
// zone. A Grizzly Bears moved battlefield->graveyard this turn is no longer
// on the battlefield, so the ordinary matcher's `Permanent` base
// (o.Zone == ZBattlefield) rejects it and the count came back 0. The
// zone-aware matcher reads a non-battlefield `Permanent` base as a permanent
// CARD (Forge's Card.isPermanent()), which is what Gravestorm's
// Count$ThisTurnEntered_Graveyard_from_Battlefield_Permanent needs.
func TestThisTurnEnteredGraveyardCountsPermanentBase(t *testing.T) {
	h := newHost(t, 2)
	card := mkCard(t, "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	bear := h.g.AddObject(card, 0)
	g := h.g
	// Move the object out of the battlefield for real, mirroring events.Move:
	// update the live zone lists, the object's Zone, and the per-add entry
	// list the ThisTurnEntered heads fold.
	g.SetZone(state.ZBattlefield, 0, nil)
	g.SetZone(state.ZGraveyard, 0, append(g.Zone(state.ZGraveyard, 0), bear.ID))
	bear.Zone = state.ZGraveyard
	g.Entered = append(g.Entered, state.ZoneEntry{Obj: bear.ID, To: state.ZGraveyard, From: state.ZBattlefield})

	c := &Ctx{Controller: 0}
	for _, tc := range []struct {
		body string
		want int32
	}{
		// Permanent is the Defect-2 carrier: 0 before the fix.
		{"Count$ThisTurnEntered_Graveyard_from_Battlefield_Permanent", 1},
		// Creature/Card are type/identity tests and were already 1; they
		// must stay 1 so the fix does not regress the common shape.
		{"Count$ThisTurnEntered_Graveyard_from_Battlefield_Creature", 1},
		{"Count$ThisTurnEntered_Graveyard_from_Battlefield_Card", 1},
	} {
		got, ok := EvalCountOK(h, c, tc.body)
		if !ok || got != tc.want {
			t.Errorf("%s = (%d, %v), want (%d, true)", tc.body, got, ok, tc.want)
		}
	}
	// An unrelated permanent that never entered the graveyard must not
	// change the count -- the fix must scope to the entry list, not widen.
	h.g.AddObject(mkCard(t, "Name:Rock\nManaCost:1\nTypes:Artifact\nOracle:x\n"), 0)
	if got, _ := EvalCountOK(h, c, "Count$ThisTurnEntered_Graveyard_from_Battlefield_Creature"); got != 1 {
		t.Errorf("unrelated object changed the count to %d", got)
	}
}

// TestThisTurnEnteredExileSkipsSpellCopies pins the CR 707.10h half of the
// zone-aware matcher: a spell copy the engine parks in exile when it resolves
// has ceased to exist (a copy that leaves the stack is a transient reference,
// not a card), so every Count$ThisTurnEntered_<off-battlefield zone> head must
// skip it. The Ennis, Debate Moderator end step reads
// Count$ThisTurnEntered_Exile_Card.!token and was counting exiled Storm/
// Gravestorm copies before the gate went into matchesZoneSpecCtx. A copy that
// resolved onto the BATTLEFIELD (CR 707.10g) is a real permanent and must keep
// counting.
func TestThisTurnEnteredExileSkipsSpellCopies(t *testing.T) {
	h := newHost(t, 2)
	g := h.g

	// A real card exiled this turn: the one honest entry.
	real := g.AddObject(mkCard(t, "Name:Trick\nManaCost:U\nTypes:Instant\nOracle:x\n"), 0)
	real.Zone = state.ZExile
	g.SetZone(state.ZExile, 0, append(g.Zone(state.ZExile, 0), real.ID))
	g.Entered = append(g.Entered, state.ZoneEntry{Obj: real.ID, To: state.ZExile, From: state.ZStack})

	// A spell copy the engine parked in exile after resolution (IsCopy,
	// live zone exile): ceased per CR 707.10h, must match nothing.
	copyObj := g.AddObject(mkCard(t, "Name:Tendrils\nManaCost:2 B B\nTypes:Sorcery\nOracle:x\n"), 0)
	copyObj.IsCopy = true
	copyObj.Zone = state.ZExile
	g.SetZone(state.ZExile, 0, append(g.Zone(state.ZExile, 0), copyObj.ID))
	g.Entered = append(g.Entered, state.ZoneEntry{Obj: copyObj.ID, To: state.ZExile, From: state.ZStack})

	// A copy of a permanent spell that resolved onto the battlefield
	// (CR 707.10g): a real permanent, must still count.
	permObj := g.AddObject(mkCard(t, "Name:Clone\nManaCost:2 U\nTypes:Creature Shapeshifter\nPT:0/0\nOracle:x\n"), 0)
	permObj.IsCopy = true
	permObj.Zone = state.ZBattlefield
	g.SetZone(state.ZBattlefield, 0, append(g.Zone(state.ZBattlefield, 0), permObj.ID))
	g.Entered = append(g.Entered, state.ZoneEntry{Obj: permObj.ID, To: state.ZBattlefield, From: state.ZStack})

	c := &Ctx{Controller: 0}
	for _, tc := range []struct {
		body string
		want int32
	}{
		// The Ennis head: 2 before the CR 707.10h gate (real card + exiled
		// copy), 1 after.
		{"Count$ThisTurnEntered_Exile_Card.!token", 1},
		{"Count$ThisTurnEntered_Exile_Card", 1},
		// The battlefield copy is real (CR 707.10g) and must count.
		{"Count$ThisTurnEntered_Battlefield_Permanent", 1},
	} {
		got, ok := EvalCountOK(h, c, tc.body)
		if !ok || got != tc.want {
			t.Errorf("%s = (%d, %v), want (%d, true)", tc.body, got, ok, tc.want)
		}
	}
}
