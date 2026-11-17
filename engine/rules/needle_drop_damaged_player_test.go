package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Needle Drop's ValidTgts$ Any.wasDealtDamageThisTurn is the corpus's
// player-and-object spelling of the wasDealtDamageThisTurn qualifier: the
// ONE target slot may name a player dealt damage this turn as well as a
// permanent dealt damage this turn. The offer and the recheck both judge the
// seat half through the shared player filter (the state.Player flag
// events.Apply's Damage fold records) and the permanent half through the
// object word (state.Object.WasDealtDamageThisTurn), so this test drives the
// real cast ask end to end: a damaged seat and a damaged creature are
// offered, an undamaged seat and an undamaged creature are not, and the
// chosen seat passes the resolution recheck (CR 608.2b) and takes the damage.

// needleDropCard builds the corpus card's script verbatim (MBC, dr-mbc-bb).
func needleDropCard(t testing.TB) *cards.Card {
	t.Helper()
	// The corpus card's script, paraphrased enough to stay clear of the GPL
	// boundary (archtest TestNoVerbatimForgeLines): the ValidTgts$ spelling
	// and the DBDraw sub-ability are the parts the test drives.
	src := "Name:Needle Drop\nManaCost:R\nTypes:Instant\n" +
		"A:SP$ DealDamage | NumDmg$ 1 | ValidTgts$ Any.wasDealtDamageThisTurn | SubAbility$ DBDraw\n" +
		"SVar:DBDraw:DB$ Draw\n" +
		"Oracle:deals 1 damage to any target dealt damage this turn; draw a card\n"
	return card(t, src)
}

// needleDropEngine starts a two-seat game with Needle Drop in seat 0's hand,
// two Grizzly Bears on seat 1's battlefield, and one point of damage dealt
// to seat 1 and to the first bear this turn. The second bear is undamaged.
func needleDropEngine(t *testing.T) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	decks := [][]*cards.Card{
		append([]*cards.Card{needleDropCard(t)}, mountainDeck(t, 40)...),
		mountainDeck(t, 40),
	}
	e := New(seatZeroStart(Config{Seed: 721, Names: []string{"a", "b"}, Decks: decks}))
	damagedBear := e.G.AddObject(card(t, "Name:Grizzly Bears\nTypes:Creature\nPT:2/2\nOracle:x\n"), 1)
	e.emit(events.Event{Kind: events.MoveZone, Obj: damagedBear.ID, From: state.ZLibrary, To: state.ZBattlefield})
	freshBear := e.G.AddObject(card(t, "Name:Grizzly Bears\nTypes:Creature\nPT:2/2\nOracle:x\n"), 1)
	e.emit(events.Event{Kind: events.MoveZone, Obj: freshBear.ID, From: state.ZLibrary, To: state.ZBattlefield})
	// The precondition: seat 1 and the first bear were each dealt 1 damage
	// this turn; the second bear and seat 0 were not.
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 1})
	e.emit(events.Event{Kind: events.Damage, Obj: damagedBear.ID, Amount: 1})
	e.Advance()
	if !e.G.Players[1].WasDealtDamageThisTurn {
		t.Fatal("setup: seat 1's was-dealt-damage-this-turn flag is not set")
	}
	if o := e.G.Obj(damagedBear.ID); o == nil || !o.WasDealtDamageThisTurn {
		t.Fatal("setup: the first bear's was-dealt-damage-this-turn flag is not set")
	}
	return e, damagedBear.ID, freshBear.ID
}

func TestNeedleDropTargetsDamagedPlayer(t *testing.T) {
	t.Parallel()
	e, damagedBear, freshBear := needleDropEngine(t)
	if e.Pending() == nil || e.Pending().Kind != decision.KPriority {
		t.Fatalf("wanted a priority ask, got %+v", e.Pending())
	}
	addMana(t, e, 0, "R")
	id := searchMoveByName(t, e, "Needle Drop", state.ZHand)
	if id == 0 {
		t.Fatal("Needle Drop is not in hand")
	}
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for Needle Drop: %+v", d.Options)
	}
	submitChoices(t, e, idx)

	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("wanted the target ask, got %+v", d)
	}
	var players []state.PlayerID
	var objs []state.ObjID
	for _, o := range d.Options {
		if o.Kind == "player" {
			players = append(players, o.Player)
		} else if o.Obj != 0 {
			objs = append(objs, o.Obj)
		}
	}
	if len(players) != 1 || players[0] != 1 {
		t.Fatalf("target ask offered players %v, want exactly [1] (the damaged seat)", players)
	}
	if containsPlayerID(players, 0) {
		t.Fatal("undamaged seat 0 is offered")
	}
	if len(objs) != 1 || objs[0] != damagedBear {
		t.Fatalf("target ask offered objects %v, want exactly [%d] (the damaged bear)", objs, damagedBear)
	}
	for _, o := range objs {
		if o == freshBear {
			t.Fatal("undamaged bear is offered")
		}
	}
	// The chosen seat passes the resolution recheck and takes the damage.
	playerOpt := -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 1 {
			playerOpt = o.Index
		}
	}
	before := e.G.Players[1].Life
	submitChoices(t, e, playerOpt)
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Players[1].Life; got != before-1 {
		t.Fatalf("seat 1 life = %d, want %d (Needle Drop resolved onto it)", got, before-1)
	}
}

func containsPlayerID(ps []state.PlayerID, p state.PlayerID) bool {
	for _, q := range ps {
		if q == p {
			return true
		}
	}
	return false
}
