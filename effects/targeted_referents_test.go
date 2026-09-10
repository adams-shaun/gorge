package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestTargetedReferentsAreResolutionOnly is the card-driven leaf for Forge's
// self-referential Targeted* grammar. The actual selected targets live on the
// resolving stack object; Ctx.SpecContext is the only path that exposes them
// to a filter. An offer-time SpecContext therefore has no binding and fails
// closed, including through !.
func TestTargetedReferentsAreResolutionOnly(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	chandra, ok := reg.Lookup("Chandra Nalaar")
	if !ok {
		t.Fatal("missing Chandra Nalaar")
	}
	emrakul, ok := reg.Lookup("Emrakul, the World Anew")
	if !ok {
		t.Fatal("missing Emrakul, the World Anew")
	}
	var chandraSA, emrakulSA *cards.SA
	for _, f := range chandra.Faces {
		chandraSA = cards.ResolveSVar(f.SVars, "DmgAll")
	}
	for _, f := range emrakul.Faces {
		emrakulSA = cards.ResolveSVar(f.SVars, "TrigGainControl")
	}
	if chandraSA == nil || chandraSA.Params["ValidCards"] != "Creature.ControlledBy TargetedOrController" {
		t.Fatalf("Chandra corpus SA changed: %+v", chandraSA)
	}
	if emrakulSA == nil || emrakulSA.Params["AllValid"] != "Creature.TargetedPlayerCtrl" {
		t.Fatalf("Emrakul corpus SA changed: %+v", emrakulSA)
	}

	g, ids := board(t)
	// A target player controls their own creature. Chandra's TargetedOrController
	// accepts it, and Emrakul's TargetedPlayerCtrl is the one-token equivalent.
	playerTarget := (&Ctx{Controller: 0,
		Targets: []state.Target{{IsPlayer: true, Player: 1}}}).SpecContext(0)
	for _, spec := range []string{
		chandraSA.Params["ValidCards"], emrakulSA.Params["AllValid"],
		"Creature.OwnedBy TargetedPlayer", "Creature.OwnedBy ThisTargetedPlayer",
	} {
		if !MatchesSpecCtx(g, spec, ids["theirBig"], playerTarget) {
			t.Errorf("%s must match the targeted player's creature", spec)
		}
		if MatchesSpecCtx(g, spec, ids["myBear"], playerTarget) {
			t.Errorf("%s must not match another player's creature", spec)
		}
		if unknown := UnknownPredicates(spec); len(unknown) != 0 {
			t.Errorf("recognised %s reported unknown %v", spec, unknown)
		}
	}

	// TargetedController names only an object target's controller; the Or form
	// accepts either an object target's controller or a direct player target.
	objectTarget := (&Ctx{Controller: 0,
		Targets: []state.Target{{Obj: ids["theirBig"]}}}).SpecContext(0)
	for _, spec := range []string{
		"Creature.ControlledBy TargetedController",
		"Creature.ControlledBy TargetedOrController",
	} {
		if !MatchesSpecCtx(g, spec, ids["theirBig"], objectTarget) {
			t.Errorf("%s must match the object target's controller", spec)
		}
		if MatchesSpecCtx(g, spec, ids["myBear"], objectTarget) {
			t.Errorf("%s must not match another player's creature", spec)
		}
	}
	// The TargetedController referent itself resolves to the target's
	// controller, even under OwnedBy. Keep target owner and controller distinct
	// so this cannot accidentally become "owned by the target".
	g.Obj(ids["theirBig"]).Owner = 0
	ownedByController := g.Obj(ids["theirBig"]).CloneDeep()
	ownedByController.Owner = 1
	if !MatchesObjectCtx(g, "Creature.OwnedBy TargetedController", &ownedByController, objectTarget) {
		t.Error("OwnedBy TargetedController must use the target's controller, not its owner")
	}

	// ! composes with the recognised Targeted* grammar. It must not turn an
	// absent resolution binding into a match, but with a binding it is the
	// ordinary complement.
	if !MatchesSpecCtx(g, "Creature.!ControlledBy TargetedPlayer", ids["myBear"], playerTarget) {
		t.Error("!ControlledBy TargetedPlayer must match another player's creature")
	}
	if MatchesSpecCtx(g, "Creature.!ControlledBy TargetedPlayer", ids["theirBig"], playerTarget) {
		t.Error("!ControlledBy TargetedPlayer must reject the targeted player's creature")
	}
	for _, spec := range []string{
		"Creature.ControlledBy TargetedPlayer",
		"Creature.!ControlledBy TargetedPlayer",
		"Creature.TargetedPlayerCtrl",
	} {
		if MatchesSpecCtx(g, spec, ids["theirBig"], SpecContext{You: 0}) {
			t.Errorf("offer-time %s must fail closed without resolution targets", spec)
		}
	}

	// non<X> does not define a Targeted* negation. Keeping it unknown (rather
	// than treating an absent target binding as false and negating to true) is
	// the required fail-closed boundary for this one-token shape.
	if MatchesSpecCtx(g, "Creature.nonTargetedPlayerCtrl", ids["theirBig"], playerTarget) {
		t.Error("nonTargetedPlayerCtrl must fail closed; non<X> does not support this grammar")
	}
	if unknown := UnknownPredicates("Creature.nonTargetedPlayerCtrl"); len(unknown) != 1 || unknown[0] != "nonTargetedPlayerCtrl" {
		t.Errorf("nonTargetedPlayerCtrl unknown = %v, want itself", unknown)
	}
}
