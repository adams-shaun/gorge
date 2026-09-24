package effects

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func TestTargetedPlayerOwn(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Boareskyr Tollkeeper")
	if !ok {
		t.Fatal("missing Boareskyr Tollkeeper")
	}
	var reveal *cards.SA
	for _, face := range card.Faces {
		if a := cards.ResolveSVar(face.SVars, "TrigReveal"); a != nil {
			reveal = a
		}
	}
	if reveal == nil || reveal.Params["RevealAllValid"] != "Creature.TargetedPlayerOwn,Land.TargetedPlayerOwn" {
		t.Fatalf("Boareskyr RevealAllValid changed: %+v", reveal)
	}
	for _, spec := range []string{"Creature.TargetedPlayerOwn", "Land.TargetedPlayerOwn", "Card.TargetedPlayerOwn"} {
		if unknown := UnknownPredicates(spec); len(unknown) != 0 {
			t.Errorf("%s unknown predicates = %v", spec, unknown)
		}
	}

	g, ids := board(t)
	target := (&Ctx{Controller: 0, Targets: []state.Target{{IsPlayer: true, Player: 1}}}).SpecContext(0)
	// Keep owner and controller different in both directions: ownership, not
	// control, is the semantic discriminator for this qualifier.
	ownedElsewhere := g.Obj(ids["myBear"]).CloneDeep()
	ownedElsewhere.Owner, ownedElsewhere.Controller = 1, 0
	controlledByTarget := g.Obj(ids["theirBig"]).CloneDeep()
	controlledByTarget.Owner, controlledByTarget.Controller = 0, 1
	if ownedElsewhere.Owner == ownedElsewhere.Controller || controlledByTarget.Owner == controlledByTarget.Controller {
		t.Fatal("precondition: candidate owner/controller values must differ")
	}
	if !MatchesObjectCtx(g, "Creature.TargetedPlayerOwn", &ownedElsewhere, target) {
		t.Error("object owned by targeted player must match despite another controller")
	}
	if MatchesObjectCtx(g, "Creature.TargetedPlayerOwn", &controlledByTarget, target) {
		t.Error("object merely controlled by targeted player must not match")
	}
	if MatchesObjectCtx(g, "Creature.TargetedPlayerOwn", &ownedElsewhere, SpecContext{You: 0}) {
		t.Error("TargetedPlayerOwn must fail closed without resolution targets")
	}
	if MatchesObjectCtx(g, "Creature.nonTargetedPlayerOwn", &ownedElsewhere, target) {
		t.Error("nonTargetedPlayerOwn must remain unknown/fail closed")
	}
	if unknown := UnknownPredicates("Creature.nonTargetedPlayerOwn"); len(unknown) != 1 || unknown[0] != "nonTargetedPlayerOwn" {
		t.Errorf("nonTargetedPlayerOwn unknown = %v", unknown)
	}

	// Exercise the actual trigger DB$ Reveal against a hand containing the
	// union's two matching types and one nonmatch, preserving hand order.
	h := newHost(t, 2)
	names := []string{"Grizzly Bears", "Plains", "Lightning Bolt"}
	var hand []state.ObjID
	for _, name := range names {
		c, found := reg.Lookup(name)
		if !found {
			t.Fatalf("missing %s", name)
		}
		o := h.g.AddObject(c, 1)
		h.g.Obj(o.ID).Zone = state.ZHand
		hand = append(hand, o.ID)
	}
	h.g.SetZone(state.ZHand, 1, hand)
	source := h.g.AddObject(card, 0)
	ctx := &Ctx{Source: source.ID, Controller: 0, Targets: []state.Target{{IsPlayer: true, Player: 1}}, TargetsOffered: true}
	SetSVars(ctx, card.Faces[0].SVars)
	sh := &suspendHost{fakeHost: *h}
	Resolve(sh, ctx, reveal)
	want := []state.ObjID{hand[0], hand[1]}
	found := false
	for _, e := range sh.log {
		if e.Kind == events.Note && len(e.IDs) > 0 {
			found = true
			if !slices.Equal(e.IDs, want) {
				t.Fatalf("revealed %v, want exactly %v", e.IDs, want)
			}
		}
	}
	if !found {
		t.Fatalf("no reveal event; log: %+v", sh.log)
	}
}
