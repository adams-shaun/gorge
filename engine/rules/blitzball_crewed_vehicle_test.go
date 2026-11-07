package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestBlitzballCountsCrewedLegendaryVehicle: CR 510.2, 301.7b, 608.2h. Blitzball's
// "dealt combat damage by a legendary creature" gate reads the source as it dealt
// the damage, so a crewed legendary Vehicle (an artifact creature then) counts.
const blitzballCrewerSrc = "Name:Crew Bear\nManaCost:G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

func TestBlitzballCountsCrewedLegendaryVehicle(t *testing.T) {
	blitz := setAuditRealCard(t, "Blitzball")
	vista := setAuditRealCard(t, "The Prima Vista")
	bear := card(t, blitzballCrewerSrc)
	e, _ := setAuditDeck(t, 8243, []*cards.Card{blitz, vista, bear}, nil)
	blitzID := searchMoveByName(t, e, "Blitzball", state.ZBattlefield)
	vistaID := searchMoveByName(t, e, "The Prima Vista", state.ZBattlefield)
	bearID := putCreature(t, e, 0, blitzballCrewerSrc)
	crewWith(t, e, vistaID, bearID)
	if !e.IsCreature(vistaID) {
		t.Fatal("precondition: the crewed Vehicle must be a creature")
	}
	if o := e.G.Obj(vistaID); o.Face().IsCreature() {
		t.Fatal("precondition: the printed face must not be a creature")
	}
	e.combatHitsThisTurn = append(e.combatHitsThisTurn, e.combatHit(1, vistaID, 6))
	// The Vehicle then leaves, so only the hit's snapshot still says "creature".
	e.emit(events.Event{Kind: events.MoveZone, Obj: vistaID, From: state.ZBattlefield, To: state.ZGraveyard})
	if e.G.Obj(vistaID).Zone != state.ZGraveyard {
		t.Fatal("precondition: the Vehicle must have left the battlefield")
	}
	// The window crewWith left open predates the hit (injected out of band, so
	// the priority memo cannot know it); walk the legal actions afresh.
	n := 0
	for _, o := range e.legalActionsWithWindow(0, nil) {
		if o.Obj == blitzID && (o.Kind == "activate" || o.Kind == "ability") {
			n++
		}
	}
	// The mana ability is always offered; the gated draw ability is the second.
	if n != 2 {
		t.Fatalf("Blitzball offers %d abilities, want 2 (mana + GOOOOAAAALLL!)", n)
	}
}
