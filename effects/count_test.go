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
