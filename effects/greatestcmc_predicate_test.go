package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// cmcBoard builds a two-seat battlefield with deliberately chosen mana values
// so the greatestCMC/lowerCMC comparison set is unambiguous. Inline scripts
// carry explicit costs because the predicate turns on the CMC, not on any
// card text.
func cmcBoard(t testing.TB) (*state.Game, map[string]state.ObjID) {
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
		// CMC 4 and 1 creatures, mine, plus a 2-cost artifact and a land;
		// opponents hold 5/2 creatures and a 3-cost artifact.
		"myBig":    mk(0, "Name:MyBig\nManaCost:3 G\nTypes:Creature Bear\nPT:4/4\nOracle:x\n"),
		"mySmall":  mk(0, "Name:MySmall\nManaCost:G\nTypes:Creature Bear\nPT:1/1\nOracle:x\n"),
		"myLand":   mk(0, "Name:MyLand\nTypes:Basic Land Forest\nOracle:x\n"),
		"myRock":   mk(0, "Name:MyRock\nManaCost:2\nTypes:Artifact\nOracle:x\n"),
		"theBig":   mk(1, "Name:TheBig\nManaCost:4 G\nTypes:Creature Giant\nPT:5/5\nOracle:x\n"),
		"theSmall": mk(1, "Name:TheSmall\nManaCost:1 G\nTypes:Creature Elf\nPT:2/2\nOracle:x\n"),
		"theRock":  mk(1, "Name:TheRock\nManaCost:3\nTypes:Artifact\nOracle:x\n"),
		"theBig2":  mk(1, "Name:TheBig2\nManaCost:4 R\nTypes:Creature Giant\nPT:6/6\nOracle:x\n"),
	}
	return g, ids
}

func TestGreatestCMCPredicate(t *testing.T) {
	g, id := cmcBoard(t)
	// theirBig (5) is the global creature maximum; myBig (4) is not.
	cases := []struct {
		name string
		spec string
		obj  string
		want bool
	}{
		{"global max creature matches", "Creature.greatestCMC_Creature", "theBig", true},
		{"lesser creature does not", "Creature.greatestCMC_Creature", "myBig", false},
		{"noncreature never in a Creature set", "Creature.greatestCMC_Creature", "myRock", false},
		// The suffix names the SET; a +YouCtrl predicate constrains only the
		// candidate. myRock (2) is not the global artifact maximum (theirRock
		// 3), so it does not match even though it is mine.
		{"artifact max is opponent's", "Artifact.greatestCMC_Artifact+YouCtrl", "myRock", false},
		{"opponent artifact is the max", "Artifact.greatestCMC_Artifact+YouCtrl", "theRock", false},
		// nonland permanents: mySmall (1) is the lowest nonland.
		{"lowest nonland permanent matches", "Permanent.nonLand+lowestCMC", "mySmall", true},
		{"a dearer nonland does not", "Permanent.nonLand+lowestCMC", "myBig", false},
		// a land is skipped from the comparison, and the base excludes it too.
		{"land is never a lowestCMC target", "Permanent.nonLand+lowestCMC", "myLand", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := MatchesSpec(g, c.spec, id[c.obj], 0); got != c.want {
				t.Errorf("MatchesSpec(%q, %s) = %v, want %v", c.spec, c.obj, got, c.want)
			}
		})
	}
}

// TestGreatestCMCTiesMatch pins Forge's getCardsWithHighestCMC tie semantics:
// every card at the maximum matches, so a tied pair both satisfies the
// predicate and neither is excluded by the other.
func TestGreatestCMCTiesMatch(t *testing.T) {
	g, id := cmcBoard(t)
	// theBig and theBig2 are both CMC 5, the joint creature maximum.
	for _, name := range []string{"theBig", "theBig2"} {
		if !MatchesSpec(g, "Creature.greatestCMC_Creature", id[name], 0) {
			t.Errorf("tied-for-highest %s did not match greatestCMC", name)
		}
	}
	// A third creature must not share a set with neither being the max.
	if MatchesSpec(g, "Creature.greatestCMC_Creature", id["theSmall"], 0) {
		t.Error("a lesser creature matched a tied maximum")
	}
}

// TestGreatestCMCControlledByRemembered resolves the ControlledBy suffix
// through the same remembered-player grammar greatestPower uses: the set is
// the remembered player's creatures, so their own maximum matches and no
// other player's does, even when a third player's creature is globally
// greater.
func TestGreatestCMCControlledByRemembered(t *testing.T) {
	g, id := cmcBoard(t)
	sc := SpecContext{Resolving: true, Remembered: []state.Target{{Player: 1, IsPlayer: true}}}
	// The remembered player's own maximum is theBig2 (5, tied with theBig).
	if !MatchesSpecCtx(g, "Creature.greatestCMC_CreatureControlledByRemembered", id["theBig2"], sc) {
		t.Error("the remembered player's own maximum creature did not match")
	}
	// Seat 0's creature is not in the remembered player's set.
	if MatchesSpecCtx(g, "Creature.greatestCMC_CreatureControlledByRemembered", id["myBig"], sc) {
		t.Error("a creature outside the remembered player's set matched")
	}
	// With NO remembered player the referent is unbound and the predicate
	// fails closed -- it must not degrade to the uncontrolled whole-field read.
	if MatchesSpec(g, "Creature.greatestCMC_CreatureControlledByRemembered", id["theBig2"], 0) {
		t.Error("an unbound ControlledBy referent degraded to the whole field")
	}
}

// TestCMCPredicatesAreRecognised pins the shared classifier: every corpus
// spelling of the greatestCMC/lowestCMC family is recognised by both the
// matcher and the UnknownPredicates census.
func TestCMCPredicatesAreRecognised(t *testing.T) {
	for _, spec := range []string{
		"Creature.greatestCMC_Creature",
		"Creature.greatestCMC_CreatureControlledByRemembered",
		"Permanent.greatestCMC_NonLandPermanentControlledByRemembered",
		"Artifact.greatestCMC_Artifact+YouCtrl",
		"Permanent.nonLand+lowestCMC",
	} {
		if un := UnknownPredicates(spec); len(un) != 0 {
			t.Errorf("UnknownPredicates(%q) = %v, want empty", spec, un)
		}
	}
}
