package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 122.6a: the counters an object enters the battlefield with are put on it
// by the player the effect names, else by the object's controller. An effect
// one player controls that returns another player's creature card "under its
// owner's control with a +1/+1 counter on it" (Dr. Leonard "Bones" McCoy's
// delayed return) does not name a player, so the owner, who controls it as it
// enters, puts the counter. Dr. Joseph M'Benga's "if you would put one or more
// counters on a creature you control" on the owner's side therefore doubles
// it, and Vorinclex's "if an opponent would put" half does not apply.

// ownerReturnSource is an authored source on its controller's battlefield whose
// ability brings a creature card from any graveyard back under its owner's
// control with one +1/+1 counter.
func ownerReturnSource(t *testing.T) *cards.Card {
	return card(t, "Name:Graveside Lantern\nTypes:Artifact\n"+
		"A:AB$ ChangeZone | Cost$ T | Origin$ Graveyard | Destination$ Battlefield | TgtZone$ Graveyard | "+
		"ValidTgts$ Creature | WithCountersType$ P1P1 | SpellDescription$ Bring back target creature card from a graveyard under its owner's control with a +1/+1 counter.\n"+
		"Oracle:x\n")
}

// entryPutterBoard puts replName on seat 0's battlefield, the authored return
// source on seat 1's, and a 2/2 creature card in seat 0's graveyard, and
// activates the source from seat 1 targeting the card. It returns the
// engine, cfg and the card's id.
func entryPutterBoard(t *testing.T, seed uint64, replName string) (*Engine, Config, state.ObjID) {
	t.Helper()
	repl := corpusAlternativeCard(t, replName)
	src := ownerReturnSource(t)
	target := card(t, "Name:Fallen Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e, cfg := tokenReplGameSeats(t, seed, []*cards.Card{repl, target}, []*cards.Card{src})
	moveSeededCard(t, e, 0, repl, state.ZBattlefield)
	srcID := moveSeededCard(t, e, 1, src, state.ZBattlefield)
	targetID := moveSeededCard(t, e, 0, target, state.ZGraveyard)
	activateCounterSourceAs(t, e, 1, srcID, targetID)
	return e, cfg, targetID
}

func entrantCounters(t *testing.T, e *Engine, card state.ObjID) int32 {
	t.Helper()
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o.Card != nil && o.Card.Faces[0].Name == "Fallen Bear" {
			return o.Counter("P1P1")
		}
	}
	t.Fatalf("the returned card %d is not on seat 0's battlefield", card)
	return 0
}

func TestEntryCountersArePutByTheEnteringObjectsController(t *testing.T) {
	t.Parallel()
	e, cfg, bear := entryPutterBoard(t, 223, "Dr. Joseph M'Benga")
	if got := entrantCounters(t, e, bear); got != 2 {
		t.Fatalf("M'Benga's owner gets the returned creature with %d counters, want 2: the owner puts the entry counter (CR 122.6a), so \"if you would put\" doubles it", got)
	}
	replayCheck(t, e, cfg)

	e2, cfg2, bear2 := entryPutterBoard(t, 227, "Vorinclex, Monstrous Raider")
	if got := entrantCounters(t, e2, bear2); got != 2 {
		t.Fatalf("Vorinclex's controller gets the returned creature with %d counters, want 2: its own placement doubles, and the opponent's halving does not apply (CR 122.6a)", got)
	}
	replayCheck(t, e2, cfg2)
}
