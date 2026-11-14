package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// kickedCastOffered reports whether seat 0's pending priority offers the
// card's kicked cast (the cast option whose Mode is "kicked") and whether it
// offers the plain one.
func kickedCastOffered(t *testing.T, e *Engine, id state.ObjID) (kicked, plain bool) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("pending = %+v, want seat 0's priority", d)
	}
	for _, o := range d.Options {
		if o.Kind != "cast" || o.Obj != id {
			continue
		}
		switch o.Mode {
		case "kicked":
			kicked = true
		case "":
			plain = true
		}
	}
	return kicked, plain
}

// TestKickedUniqueChainIsJudgedAtTheOffer pins CR 601.2c for Urborg
// Repossession ("return target creature card from your graveyard to your
// hand ... if kicked, return another target permanent card from your
// graveyard"): the Condition$ Kicked, TargetUnique$ link owes a target only
// when the spell is kicked, so the plain cast needs one creature card and the
// kicked cast needs two distinct permanent cards. With a lone permanent card
// in the graveyard the kicked cast was offered and then reversed with "cast
// aborted: no legal target" (the root ask withholds the only creature, as it
// leaves the unique link nothing).
func TestKickedUniqueChainIsJudgedAtTheOffer(t *testing.T) {
	t.Parallel()
	e, cfg, mine, _ := cr601Board(t, 61100,
		map[string]state.Zone{"Urborg Repossession": state.ZHand, "Grizzly Bears": state.ZGraveyard, "Craw Wurm": state.ZLibrary},
		map[string]state.Zone{})
	spell := mine["Urborg Repossession"]
	addMana(t, e, 0, "BG11")
	edrSeatZeroPriority(t, e)
	kicked, plain := kickedCastOffered(t, e, spell)
	if !plain {
		t.Fatalf("precondition: the plain cast must be offered with a creature card in the graveyard")
	}
	if kicked {
		t.Fatalf("the kicked cast is offered with one permanent card in the graveyard: its unique link has nothing to name")
	}
	moveByName(t, e, 0, "Craw Wurm", state.ZGraveyard)
	e.pending = nil
	e.priorityRound()
	edrSeatZeroPriority(t, e)
	if kicked, _ = kickedCastOffered(t, e, spell); !kicked {
		t.Fatalf("the kicked cast is not offered with two permanent cards in the graveyard")
	}
	replayCheck(t, e, cfg)
}
