package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDiesReturnPutsCounterOnReturnedCard: a dies trigger that returns its
// own card to the battlefield and then puts a counter on Defined$
// TriggeredCard (Bogardan Phoenix's "return it to the battlefield under your
// control and put a death counter on it") puts the counter on the returned
// permanent. The resolution itself moved the card, so the referent is the
// object it put onto the battlefield, not a departed source (CR 400.7;
// XMage's BogardanPhoenixEffect enters it with the counter). The second
// death then exiles it.
func TestDiesReturnPutsCounterOnReturnedCard(t *testing.T) {
	t.Parallel()
	phoenix := "Name:Phoenix Fixture\nManaCost:2 R R R\nTypes:Creature Phoenix\nPT:3/3\n" +
		"T:Mode$ ChangesZone | ValidCard$ Card.Self+counters_EQ0_DEATH | Origin$ Battlefield | Destination$ Graveyard | Execute$ Back | TriggerDescription$ x\n" +
		"T:Mode$ ChangesZone | ValidCard$ Card.Self+counters_GE1_DEATH | Origin$ Battlefield | Destination$ Graveyard | Execute$ Gone | Secondary$ True | TriggerDescription$ x\n" +
		"SVar:Gone:DB$ ChangeZone | Origin$ Graveyard | Destination$ Exile | Defined$ TriggeredNewCardLKICopy\n" +
		"SVar:Back:DB$ ChangeZone | Origin$ Graveyard | Destination$ Battlefield | Defined$ TriggeredNewCardLKICopy | SubAbility$ Mark\n" +
		"SVar:Mark:DB$ PutCounter | CounterType$ DEATH | CounterNum$ 1 | Defined$ TriggeredCard\n" +
		"Oracle:x\n"
	e, cfg, p := newFixtureDeck(t, 83, phoenix)
	e.emit(events.Event{Kind: events.MoveZone, Obj: p, From: state.ZHand, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.Damage, Obj: p, Amount: 3})
	e.checkStateBased()
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)
	if o := e.G.Obj(p); o.Zone != state.ZBattlefield || o.Counter("DEATH") != 1 {
		t.Fatalf("after first death: %s, death counters %d (want battlefield, 1)", o.Zone, o.Counter("DEATH"))
	}
	e.emit(events.Event{Kind: events.Damage, Obj: p, Amount: 3})
	e.checkStateBased()
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)
	if z := e.G.Obj(p).Zone; z != state.ZExile {
		t.Fatalf("after second death: %s (want exile)", z)
	}
	replayCheck(t, e, cfg)
}
