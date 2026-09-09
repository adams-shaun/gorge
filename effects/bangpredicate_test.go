package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestBangPredicateNegation is the effects leaf for the leading-'!' negation
// grammar. Before this change '!' was only a literal character inside the one
// hand-written "!token" map entry, so `Creature.!attacking` (the positive form
// is implemented and working) matched NOTHING -- a card whose target was "a
// nonattacking creature" could never be cast or activated.
//
// The rule: !<X> is the negation of <X> whenever <X> is a predicate this build
// recognises, by exactly the same resolution the positive path uses. When <X>
// is NOT recognised, !<X> is unknown and fails closed, the same as <X> -- the
// negation of "I do not know" is not "yes". A second '!' (!!X) is not a shape
// this grammar defines, so it too fails closed.
func TestBangPredicateNegation(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})

	// Leaf 1: !<X> negates a recognised predicate. !attacking negates the
	// "attacking" map predicate, so a creature that is not attacking matches
	// and one that is attacking does not.
	resting := g.Obj(corpusObject(t, reg, g, "Grizzly Bears").ID)
	attacking := g.Obj(corpusObject(t, reg, g, "Grizzly Bears").ID)
	attacking.IsAttacking = true
	if !MatchesObjectCtx(g, "Creature.!attacking", resting, SpecContext{You: 0}) {
		t.Errorf("Creature.!attacking must match a creature that is not attacking")
	}
	if MatchesObjectCtx(g, "Creature.!attacking", attacking, SpecContext{You: 0}) {
		t.Errorf("Creature.!attacking must not match a creature that is attacking")
	}

	// Leaf 1b: !negates a game-aware wordPredicate too. !HasCounters negates
	// the HasCounters family, and !IsCommander negates the IsCommander family.
	plain := g.Obj(corpusObject(t, reg, g, "Grizzly Bears").ID)
	counted := g.Obj(corpusObject(t, reg, g, "Grizzly Bears").ID)
	counted.AddCounter("P1P1", 1)
	if !MatchesObjectCtx(g, "Creature.!HasCounters", plain, SpecContext{You: 0}) {
		t.Errorf("Creature.!HasCounters must match a creature with no counters")
	}
	if MatchesObjectCtx(g, "Creature.!HasCounters", counted, SpecContext{You: 0}) {
		t.Errorf("Creature.!HasCounters must not match a creature with a counter")
	}
	commander := g.Obj(corpusObject(t, reg, g, "Grizzly Bears").ID)
	g.Players[0].Commanders = []state.ObjID{commander.ID}
	notCommander := g.Obj(corpusObject(t, reg, g, "Grizzly Bears").ID)
	if !MatchesObjectCtx(g, "Creature.!IsCommander", notCommander, SpecContext{You: 0}) {
		t.Errorf("Creature.!IsCommander must match a non-commander")
	}
	if MatchesObjectCtx(g, "Creature.!IsCommander", commander, SpecContext{You: 0}) {
		t.Errorf("Creature.!IsCommander must not match a commander")
	}

	// Leaf 2: !<X> on an UNRECOGNISED <X> matches nothing and is still
	// reported by UnknownPredicates. IsRemembered is a family pc2 deliberately
	// left unknown; its negation must fail closed too -- "not known" is not
	// "yes".
	if MatchesObjectCtx(g, "Creature.!IsRemembered", plain, SpecContext{You: 0}) {
		t.Errorf("Creature.!IsRemembered must match nothing (IsRemembered is unrecognised)")
	}
	if un := UnknownPredicates("Creature.!IsRemembered"); len(un) != 1 || un[0] != "!IsRemembered" {
		t.Errorf("UnknownPredicates(Creature.!IsRemembered) = %v, want [!IsRemembered]", un)
	}
	// The bare unknown form is still reported exactly the same way.
	if un := UnknownPredicates("Creature.IsRemembered"); len(un) != 1 || un[0] != "IsRemembered" {
		t.Errorf("UnknownPredicates(Creature.IsRemembered) = %v, want [IsRemembered]", un)
	}

	// Leaf 4: ! and non<X> compose without one path shadowing the other.
	// !nonLand negates the generic non<X> negation, so it is "is a land".
	mountain := g.Obj(corpusObject(t, reg, g, "Mountain").ID)
	if !MatchesObjectCtx(g, "Permanent.!nonLand", mountain, SpecContext{You: 0}) {
		t.Errorf("Permanent.!nonLand must match a land (the double negation of nonLand)")
	}
	if MatchesObjectCtx(g, "Permanent.!nonLand", plain, SpecContext{You: 0}) {
		t.Errorf("Permanent.!nonLand must not match a non-land (the double negation of nonLand)")
	}
	// sanctity of the original non<X>: still a real negation on its own.
	if MatchesObjectCtx(g, "Permanent.nonLand", mountain, SpecContext{You: 0}) {
		t.Errorf("Permanent.nonLand must not match a land")
	}
	if !MatchesObjectCtx(g, "Permanent.nonLand", plain, SpecContext{You: 0}) {
		t.Errorf("Permanent.nonLand must match a non-land")
	}

	// Leaf 4b: the matcher and UnknownPredicates agree that !nonLand and
	// !attacking are recognised (so the census does not report them).
	for _, spec := range []string{"Creature.!attacking", "Permanent.!nonLand", "Creature.!HasCounters"} {
		if un := UnknownPredicates(spec); len(un) != 0 {
			t.Errorf("UnknownPredicates(%q) = %v, want empty (recognised negation)", spec, un)
		}
	}

	// !!X is not a thing: it fails closed like any unknown and is reported.
	if MatchesObjectCtx(g, "Creature.!!attacking", plain, SpecContext{You: 0}) {
		t.Errorf("Creature.!!attacking must match nothing (a double bang is not a shape)")
	}
	if un := UnknownPredicates("Creature.!!attacking"); len(un) != 1 || un[0] != "!!attacking" {
		t.Errorf("UnknownPredicates(Creature.!!attacking) = %v, want [!!attacking]", un)
	}

	// !token: previously the ONE hand-written map entry. With the map entry
	// removed, the generic '!' path must reproduce it: a token (Card == nil)
	// does not match !token, a real card does.
	token := g.Obj(corpusObject(t, reg, g, "Grizzly Bears").ID)
	token.Card = nil // a token: Card is nil
	if MatchesObjectCtx(g, "Creature.!token", token, SpecContext{You: 0}) {
		t.Errorf("Creature.!token must not match a token")
	}
	if !MatchesObjectCtx(g, "Creature.!token", plain, SpecContext{You: 0}) {
		t.Errorf("Creature.!token must match a non-token (a real card)")
	}
	if un := UnknownPredicates("Creature.!token"); len(un) != 0 {
		t.Errorf("UnknownPredicates(Creature.!token) = %v, want empty", un)
	}
}
