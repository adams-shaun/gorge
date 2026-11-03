package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// intensityEngine seats the named corpus cards in seat 0's deck (padded with
// Mountains) and advances to the first priority ask.
func intensityEngine(t *testing.T, seed uint64, names ...string) (*Engine, Config) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	var deck []*cards.Card
	for _, name := range names {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("corpus has no %q", name)
		}
		deck = append(deck, c)
	}
	cfg := seatZeroStart(Config{Seed: seed, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{append(deck, mountainDeck(t, 40-len(deck))...), mountainDeck(t, 40)}})
	e := New(cfg)
	e.Advance()
	return e, cfg
}

// ownedNamed lists seat 0's objects with the given printed name, in id order.
func ownedNamed(e *Engine, name string) []state.ObjID {
	var out []state.ObjID
	for i := range e.G.Objs {
		if o := &e.G.Objs[i]; o.Owner == 0 && o.Face() != nil && o.Face().Name == name {
			out = append(out, o.ID)
		}
	}
	return out
}

// sendTo moves id into zone to with a logged MoveZone (a no-op when it is
// already there).
func sendTo(e *Engine, id state.ObjID, to state.Zone) {
	if from := e.G.Obj(id).Zone; from != to {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: from, To: to})
	}
}

func intensityOf(e *Engine, id state.ObjID) int32 { return e.G.Obj(id).IntensityTotal() }

// TestStaticDischargeIntensifiesEveryCopyYouOwn: the spell deals damage equal
// to its intensity, then every card you own named Static Discharge intensifies
// in whatever zone it sits (AllDefined$), and the increase is the card's own,
// so the library copy deals the larger number once drawn.
func TestStaticDischargeIntensifiesEveryCopyYouOwn(t *testing.T) {
	t.Parallel()
	e, cfg := intensityEngine(t, 31, "Static Discharge", "Static Discharge")
	both := ownedNamed(e, "Static Discharge")
	first, second := both[0], both[1]
	sendTo(e, first, state.ZHand)
	sendTo(e, second, state.ZLibrary)
	if got := intensityOf(e, first); got != 3 {
		t.Fatalf("starting intensity %d, want the printed 3", got)
	}
	addMana(t, e, 0, "RR")
	life := e.G.Players[1].Life
	castTargeting(t, e, first, true, 1, 0)
	if got := e.G.Players[1].Life; got != life-3 {
		t.Fatalf("opponent life %d, want %d (3 damage)", got, life-3)
	}
	if z := e.G.Obj(first).Zone; z != state.ZGraveyard {
		t.Fatalf("cast copy is in %s, want the graveyard", z)
	}
	for name, id := range map[string]state.ObjID{"graveyard": first, "library": second} {
		if got := intensityOf(e, id); got != 4 {
			t.Fatalf("%s copy intensity %d, want 4", name, got)
		}
	}
	sendTo(e, second, state.ZHand)
	if got := intensityOf(e, second); got != 4 {
		t.Fatalf("intensity %d after changing zones, want 4: the increase is the card's, not the zone stint's", got)
	}
	addMana(t, e, 0, "RR")
	life = e.G.Players[1].Life
	castTargeting(t, e, second, true, 1, 0)
	if got := e.G.Players[1].Life; got != life-4 {
		t.Fatalf("opponent life %d, want %d (4 damage)", got, life-4)
	}
	replayCheck(t, e, cfg)
}

// TestSummitfestIntensifiesOnlyOnAFiveManaSpell: Covercast. The trigger runs
// from the hand, gates on the other spell's mana spent, and the ritual adds X
// of each colour for the intensity it has by then.
func TestSummitfestIntensifiesOnlyOnAFiveManaSpell(t *testing.T) {
	t.Parallel()
	e, cfg := intensityEngine(t, 32, "Summitfest Closing Ceremony", "Rout", "Lightning Bolt")
	fest := ownedNamed(e, "Summitfest Closing Ceremony")[0]
	rout := ownedNamed(e, "Rout")[0]
	bolt := ownedNamed(e, "Lightning Bolt")[0]
	for _, id := range []state.ObjID{fest, rout, bolt} {
		sendTo(e, id, state.ZHand)
	}
	addMana(t, e, 0, "R")
	castTargeting(t, e, bolt, true, 1, 0)
	if got := intensityOf(e, fest); got != 3 {
		t.Fatalf("intensity %d after a one-mana spell, want the printed 3", got)
	}
	addMana(t, e, 0, "WWWWW")
	submitChoices(t, e, miscCastOption(t, e, rout))
	settleStackAnswering(t, e)
	if got := intensityOf(e, fest); got != 4 {
		t.Fatalf("intensity %d after a five-mana spell, want 4", got)
	}
	handBefore := len(e.G.Zone(state.ZHand, 0))
	addMana(t, e, 0, "UUURR")
	submitChoices(t, e, miscCastOption(t, e, fest))
	settleStackAnswering(t, e)
	if u, r := e.G.Players[0].Pool[state.MU], e.G.Players[0].Pool[state.MR]; u != 4 || r != 4 {
		t.Fatalf("pool U=%d R=%d, want 4 and 4 (X is the intensity)", u, r)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore {
		t.Fatalf("hand %d, want %d (cast one, drew one)", got, handBefore)
	}
	replayCheck(t, e, cfg)
}

// TestMintharaIntensifiesItselfAndItsNamesakes: a permanent leaving the
// battlefield raises Minthara's own intensity and that of the copies in the
// library (a union of AllDefined$ alternatives), and her static reads it.
func TestMintharaIntensifiesItselfAndItsNamesakes(t *testing.T) {
	t.Parallel()
	e, cfg := intensityEngine(t, 33, "Minthara of the Absolute", "Minthara of the Absolute")
	both := ownedNamed(e, "Minthara of the Absolute")
	onBoard, inLibrary := both[0], both[1]
	sendTo(e, onBoard, state.ZBattlefield)
	sendTo(e, inLibrary, state.ZLibrary)
	land := ownedNamed(e, "Mountain")[0]
	sendTo(e, land, state.ZBattlefield)
	e.pending = nil
	e.priorityRound()
	if p := e.Power(onBoard); p != 2 {
		t.Fatalf("power %d before the trigger, want the printed 2", p)
	}
	sendTo(e, land, state.ZGraveyard)
	e.pending = nil
	e.priorityRound()
	resolveStack(t, e)
	for name, id := range map[string]state.ObjID{"battlefield": onBoard, "library": inLibrary} {
		if got := intensityOf(e, id); got != 1 {
			t.Fatalf("%s copy intensity %d, want 1", name, got)
		}
	}
	if p := e.Power(onBoard); p != 3 {
		t.Fatalf("power %d after the trigger, want 3 (creatures get +X/+0)", p)
	}
	replayCheck(t, e, cfg)
}
