package effects

import (
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
