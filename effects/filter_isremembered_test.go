package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// Tests for the IsRemembered filter predicate (SpecContext.ResolutionRemembered):
// a resolution-bound predicate that matches an object in the resolving spell
// or ability's Remembered set, and fails closed outside a resolution -- never
// an always-true widening.

func TestIsRememberedPredicateIsResolutionBound(t *testing.T) {
	g, ids := board(t)
	bear := g.Obj(ids["myBear"])

	// Resolution-bound: the remembered set is consulted.
	sc := SpecContext{You: 0, Resolving: true,
		ResolutionRemembered: []state.Target{{Obj: ids["myBear"]}}}
	if !MatchesObjectCtx(g, "Card.IsRemembered", bear, sc) {
		t.Error("Card.IsRemembered must match a remembered object while resolving")
	}
	if MatchesObjectCtx(g, "Card.IsRemembered", g.Obj(ids["myFlier"]), sc) {
		t.Error("Card.IsRemembered must not match an object outside the remembered set")
	}
	// The '!' negation composes with the positive form.
	if MatchesObjectCtx(g, "Card.!IsRemembered", bear, sc) {
		t.Error("Card.!IsRemembered must not match a remembered object")
	}
	if !MatchesObjectCtx(g, "Card.!IsRemembered", g.Obj(ids["myFlier"]), sc) {
		t.Error("Card.!IsRemembered must match an object outside the remembered set")
	}
	// Player entries in the remembered set are not objects: a remembered
	// PLAYER must not make an object match.
	scPlayer := SpecContext{You: 0, Resolving: true,
		ResolutionRemembered: []state.Target{{Player: 1, IsPlayer: true}}}
	if MatchesObjectCtx(g, "Card.IsRemembered", bear, scPlayer) {
		t.Error("a remembered player entry must not satisfy Card.IsRemembered")
	}

	// Outside a resolution (a target offer, a shape-only check) the set is
	// absent: the predicate matches nothing -- recognised, never widening.
	if MatchesObjectCtx(g, "Card.IsRemembered", bear, SpecContext{You: 0}) {
		t.Error("Card.IsRemembered must fail closed when not resolving")
	}
	if !MatchesObjectCtx(g, "Card.!IsRemembered", bear, SpecContext{You: 0}) {
		t.Error("Card.!IsRemembered must fail closed (match nothing) when not resolving")
	}
	// And UnknownPredicates no longer reports it: the word is classified.
	if un := UnknownPredicates("Card.IsRemembered"); len(un) != 0 {
		t.Errorf("UnknownPredicates(Card.IsRemembered) = %v, want empty", un)
	}
}

// TestCtxSpecContextCarriesRemembered pins the Ctx side of the binding: a
// resolving context's Remembered set (built by RememberChanged$) reaches the
// filter through SpecContext, so a chained sub-ability's IsRemembered filter
// sees what the parent remembered.
func TestCtxSpecContextCarriesRemembered(t *testing.T) {
	_, ids := board(t)
	c := &Ctx{Remembered: []state.Target{{Obj: ids["myBear"]}}}
	sc := c.SpecContext(0)
	if !sc.Resolving || len(sc.ResolutionRemembered) != 1 || sc.ResolutionRemembered[0].Obj != ids["myBear"] {
		t.Fatalf("Ctx.SpecContext must carry the Remembered set, got %+v", sc)
	}
	// The copy must not alias the Ctx's slice: an effect that appends to
	// c.Remembered after building a SpecContext must not mutate the snapshot.
	c.Remembered = append(c.Remembered, state.Target{Obj: ids["myFlier"]})
	if len(sc.ResolutionRemembered) != 1 {
		t.Fatal("SpecContext.ResolutionRemembered aliases the Ctx's slice")
	}
}
