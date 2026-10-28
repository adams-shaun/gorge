package rules

// cr-g293: Blade of the Swarm's mode 2 is `ValidTgts$ Card.withWarp` over
// Origin$ Exile ("target exiled card with warp"). The with<Keyword> filter
// vocabulary did not register Warp, so keywordPredicateFor("withWarp") failed
// closed and the mode was withheld as having no legal target. This drives the
// real corpus card's ETB Charm against a real exiled warp card and a real
// exiled non-warp card.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestBladeOfTheSwarmTargetsExiledWarpCard(t *testing.T) {
	t.Parallel()
	reg := freshCorpusRegistry(t,
		"b/blade_of_the_swarm.txt", "k/knight_luminary.txt",
		"g/grizzly_bears.txt", "f/forest.txt")
	e, cfg := etbreplEngine(t, reg, "Blade of the Swarm", "Knight Luminary", "Grizzly Bears")
	warp := searchMoveByName(t, e, "Knight Luminary", state.ZExile)
	plain := searchMoveByName(t, e, "Grizzly Bears", state.ZExile)
	blade := searchMoveByName(t, e, "Blade of the Swarm", state.ZHand)

	// Preconditions: both candidate targets are exiled, the card the filter
	// looks for really prints Warp, and the comparison card really does not.
	for _, tc := range []struct {
		name    string
		id      state.ObjID
		wantWar bool
	}{{"Knight Luminary", warp, true}, {"Grizzly Bears", plain, false}} {
		o := e.G.Obj(tc.id)
		if o == nil || o.Zone != state.ZExile || o.Face() == nil {
			t.Fatalf("precondition: %s = %+v, want an exiled card", tc.name, o)
		}
		if got := o.Face().HasKeyword("Warp"); got != tc.wantWar {
			t.Fatalf("precondition: %s printed Warp = %t, want %t", tc.name, got, tc.wantWar)
		}
	}

	addMana(t, e, 0, "BBBB")
	submitChoices(t, e, castOptionFor(t, e, blade).Index)

	d := passUntilNonPriority(t, e, 30)
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("pending = %+v, want Blade of the Swarm's Charm mode ask", d)
	}
	mode := -1
	for _, o := range d.Options {
		if strings.Contains(o.Label, "exiled card with warp") {
			mode = o.Index
		}
	}
	if mode < 0 {
		t.Fatalf("exiled-card-with-warp mode not offered: %+v", d.Options)
	}
	submitChoices(t, e, mode)

	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending = %+v, want the chosen mode's target ask", d)
	}
	warpOpt, plainOffered := -1, false
	for _, o := range d.Options {
		if o.Obj == warp {
			warpOpt = o.Index
		}
		if o.Obj == plain {
			plainOffered = true
		}
	}
	if warpOpt < 0 {
		t.Fatalf("mode 2 did not offer the exiled warp card: %+v", d.Options)
	}
	if plainOffered {
		t.Fatalf("mode 2 offered an exiled card without warp: %+v", d.Options)
	}
	submitChoices(t, e, warpOpt)
	passUntilStackEmpty(t, e, 20)

	if o := e.G.Obj(warp); o == nil || o.Zone != state.ZLibrary {
		t.Fatalf("Knight Luminary zone = %+v, want library after mode 2 resolved", o)
	}
	replayCheck(t, e, cfg)
}
