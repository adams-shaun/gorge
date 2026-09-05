package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

func board(t *testing.T) (*state.Game, map[string]state.ObjID) {
	t.Helper()
	g := state.NewGame([]string{"you", "them"})
	mk := func(owner state.PlayerID, src string) state.ObjID {
		c, d := cards.ParseBytes("t.txt", []byte(src))
		if len(d) != 0 {
			t.Fatalf("diags: %v", d)
		}
		c.Link()
		for _, f := range c.Faces {
			f.ApplyIntrinsics()
		}
		o := g.AddObject(c, owner)
		o.Zone = state.ZBattlefield
		g.SetZone(state.ZBattlefield, owner, append(g.Zone(state.ZBattlefield, owner), o.ID))
		return o.ID
	}
	ids := map[string]state.ObjID{
		"myBear":   mk(0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"),
		"myFlier":  mk(0, "Name:Flier\nManaCost:1 U\nTypes:Creature Bird\nPT:1/1\nK:Flying\nOracle:x\n"),
		"myLand":   mk(0, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n"),
		"theirBig": mk(1, "Name:Giant\nManaCost:4 R\nTypes:Creature Giant\nPT:5/5\nOracle:x\n"),
	}
	return g, ids
}

func TestBaseTypeMatching(t *testing.T) {
	g, id := board(t)
	cases := []struct {
		spec string
		obj  string
		want bool
	}{
		{"Creature", "myBear", true},
		{"Creature", "myLand", false},
		{"Land", "myLand", true},
		{"Permanent", "myLand", true},
		{"Card", "myBear", true},
		{"Any", "myBear", true},
		{"Artifact", "myBear", false},
		{"nonCreature", "myLand", true},
		{"nonCreature", "myBear", false},
		{"nonLand", "myBear", true},
	}
	for _, c := range cases {
		if got := MatchesSpec(g, c.spec, id[c.obj], 0); got != c.want {
			t.Errorf("MatchesSpec(%q, %s) = %v, want %v", c.spec, c.obj, got, c.want)
		}
	}
}

func TestControlAndOwnershipPredicates(t *testing.T) {
	g, id := board(t)
	for _, c := range []struct {
		spec string
		obj  string
		you  state.PlayerID
		want bool
	}{
		{"Creature.YouCtrl", "myBear", 0, true},
		{"Creature.YouCtrl", "theirBig", 0, false},
		{"Creature.OppCtrl", "theirBig", 0, true},
		{"Creature.OppCtrl", "myBear", 0, false},
		{"Creature.YouOwn", "myBear", 0, true},
		{"Creature.YouDontCtrl", "theirBig", 0, true},
	} {
		if got := MatchesSpec(g, c.spec, id[c.obj], c.you); got != c.want {
			t.Errorf("MatchesSpec(%q, %s, you=%d) = %v, want %v", c.spec, c.obj, c.you, got, c.want)
		}
	}
}

func TestConjunctionAndAlternation(t *testing.T) {
	g, id := board(t)
	// AND: both predicates must hold.
	if !MatchesSpec(g, "Creature.YouCtrl+withFlying", id["myFlier"], 0) {
		t.Error("flier should match Creature.YouCtrl+withFlying")
	}
	if MatchesSpec(g, "Creature.YouCtrl+withFlying", id["myBear"], 0) {
		t.Error("bear should not match +withFlying")
	}
	// OR: any alternative matching is enough.
	if !MatchesSpec(g, "Land,Creature.YouCtrl", id["myLand"], 0) {
		t.Error("land should match the first alternative")
	}
	if !MatchesSpec(g, "Land,Creature.YouCtrl", id["myBear"], 0) {
		t.Error("bear should match the second alternative")
	}
	if MatchesSpec(g, "Land,Artifact", id["myBear"], 0) {
		t.Error("bear matches neither alternative")
	}
}

func TestStatePredicates(t *testing.T) {
	g, id := board(t)
	g.Obj(id["myBear"]).Tapped = true
	g.Obj(id["theirBig"]).IsAttacking = true
	if !MatchesSpec(g, "Creature.tapped", id["myBear"], 0) {
		t.Error("tapped predicate failed")
	}
	if MatchesSpec(g, "Creature.untapped", id["myBear"], 0) {
		t.Error("untapped must be the negation of tapped")
	}
	if !MatchesSpec(g, "Creature.attacking", id["theirBig"], 0) {
		t.Error("attacking predicate failed")
	}
	if !MatchesSpec(g, "Creature.powerLE2", id["myBear"], 0) {
		t.Error("powerLE2 should match a 2/2")
	}
	if MatchesSpec(g, "Creature.powerLE2", id["theirBig"], 0) {
		t.Error("powerLE2 should not match a 5/5")
	}
	if !MatchesSpec(g, "Creature.cmcLE3", id["myBear"], 0) {
		t.Error("cmcLE3 should match a two-drop")
	}
}

func TestSelfAndOtherArePerspectiveDependent(t *testing.T) {
	g, id := board(t)
	// Self and Other are evaluated against the effect's source, which the
	// caller passes as the "you" object via SpecCtx.
	if !MatchesSpecFrom(g, "Card.Self", id["myBear"], 0, id["myBear"]) {
		t.Error("Self should match the source object")
	}
	if MatchesSpecFrom(g, "Card.Self", id["myFlier"], 0, id["myBear"]) {
		t.Error("Self must not match a different object")
	}
	if !MatchesSpecFrom(g, "Creature.Other", id["myFlier"], 0, id["myBear"]) {
		t.Error("Other should match any object but the source")
	}
}

func TestUnknownPredicatesAreReportedNotSilentlyTrue(t *testing.T) {
	g, id := board(t)
	got := UnknownPredicates("Creature.YouCtrl+someMechanicWeDoNotModel")
	if len(got) != 1 || got[0] != "someMechanicWeDoNotModel" {
		t.Fatalf("UnknownPredicates = %v", got)
	}
	// An unknown predicate must not match: a filter that silently widens is
	// how a rules engine quietly does the wrong thing.
	if MatchesSpec(g, "Creature.someMechanicWeDoNotModel", id["myBear"], 0) {
		t.Fatal("unknown predicate matched")
	}
	if len(UnknownPredicates("Creature.YouCtrl+withFlying")) != 0 {
		t.Fatal("known predicates reported as unknown")
	}
}

func TestPlayerSpecs(t *testing.T) {
	g, _ := board(t)
	if !MatchesPlayerSpec(g, "Player", 1, 0) {
		t.Error("Player should match anyone")
	}
	if !MatchesPlayerSpec(g, "Opponent", 1, 0) {
		t.Error("Opponent should match the other seat")
	}
	if MatchesPlayerSpec(g, "Opponent", 0, 0) {
		t.Error("Opponent must not match yourself")
	}
	if !MatchesPlayerSpec(g, "You", 0, 0) {
		t.Error("You should match yourself")
	}
}

// twoSeatGame builds a fresh 2-seat game holding one object parsed from src,
// owned by seat 0, for tests that need a plain board plus an unrelated
// "source" object of their own (SpecContext.Source).
func twoSeatGame(t *testing.T, src string) (*state.Game, state.ObjID) {
	t.Helper()
	g := state.NewGame([]string{"you", "them"})
	return g, g.AddObject(mkCard(t, src), 0).ID
}

func TestSpecContextResolvesNumericRHSAndChoices(t *testing.T) {
	g, id := twoSeatGame(t, "Name:Bolt\nManaCost:R\nTypes:Instant\nOracle:x\n") // cmc 1; helper in this file or write it
	src := g.AddObject(g.Obj(id).Card, 0)
	src.ChosenName, src.ChosenType, src.ChosenNumber = "Bolt", "Goblin", 1
	sc := SpecContext{You: 0, Source: src.ID, Resolve: func(name string) (int32, bool) {
		switch name {
		case "Y":
			return 1, true
		case "Chosen":
			return src.ChosenNumber, true
		}
		return 0, false
	}}
	for spec, want := range map[string]bool{
		"Card.cmcEQY":        true,
		"Card.cmcEQChosen":   true,
		"Card.cmcGTY":        false,
		"Card.cmcEQZ":        false, // unresolvable: never matches
		"Card.NamedCard":     true,
		"Card.ChosenType":    false, // Bolt is not a Goblin
		"Card.kicked":        false,
		"Card.StrictlyOther": true,
	} {
		if got := MatchesSpecCtx(g, spec, id, sc); got != want {
			t.Errorf("%s: %v, want %v", spec, got, want)
		}
	}
	g.Obj(id).CastFlags = state.FlagKicked | state.FlagSurged
	if !MatchesSpecCtx(g, "Card.kicked+surged", id, sc) {
		t.Error("kicked+surged")
	}
	if MatchesSpecFrom(g, "Card.cmcEQY", id, 0, src.ID) {
		t.Error("MatchesSpecFrom has no resolver and must not match an SVar-shaped RHS")
	}
	// A copy off the stack matches nothing.
	g.Obj(id).IsCopy = true
	g.Obj(id).Zone = state.ZExile
	if MatchesSpecCtx(g, "Card", id, sc) {
		t.Error("an exiled copy matched")
	}
}

func TestPermanentOnlyMatchesBattlefield(t *testing.T) {
	g, id := board(t)
	// Object is initially on the battlefield; should match Permanent.
	if !MatchesSpec(g, "Permanent", id["myBear"], 0) {
		t.Error("card on battlefield should match Permanent")
	}
	if !MatchesSpec(g, "Card", id["myBear"], 0) {
		t.Error("card on battlefield should match Card")
	}

	// Move the object to hand (ZHand = 1 as a typical zone ID).
	g.Obj(id["myBear"]).Zone = state.ZHand
	if MatchesSpec(g, "Permanent", id["myBear"], 0) {
		t.Error("card in hand must not match Permanent")
	}
	if !MatchesSpec(g, "Card", id["myBear"], 0) {
		t.Error("card in hand should still match Card")
	}

	// Also verify graveyard.
	g.Obj(id["myBear"]).Zone = state.ZGraveyard
	if MatchesSpec(g, "Permanent", id["myBear"], 0) {
		t.Error("card in graveyard must not match Permanent")
	}
	if !MatchesSpec(g, "Card", id["myBear"], 0) {
		t.Error("card in graveyard should still match Card")
	}
}
