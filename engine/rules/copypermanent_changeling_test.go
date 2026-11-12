package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 707.9a/b, 702.73a: Dominion Infiltration's tokens are copies "except
// it has changeling", so the exception is part of each token's copiable
// values and the token is every creature type; the haste it gains is not an
// exception. The spell targets only creatures an opponent controls.
func TestDominionInfiltrationTokenCopyHasChangelingAndHaste(t *testing.T) {
	t.Parallel()
	fixture := choiceCorpusCard(t, "Dominion Infiltration")
	cfg := seatZeroStart(Config{Seed: 4417, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{fixture, card(t, gateRaiderSrc)}, mountainDeck(t, 38)...),
			append([]*cards.Card{card(t, gateRaiderSrc)}, mountainDeck(t, 39)...),
		},
		Tokens: map[string]*cards.Card{},
	})
	e := New(cfg)
	e.Advance()
	di := findInZones(t, e, 0, "Dominion Infiltration")
	mine := gateMoveFromLibrary(t, e, "Raider", state.ZBattlefield)
	theirs := findInZones(t, e, 1, "Raider")
	from := state.ZLibrary
	if inZone(e, state.ZHand, 1, theirs) {
		from = state.ZHand
	}
	moveQuiet(e, theirs, from, state.ZBattlefield)
	if inZone(e, state.ZLibrary, 0, di) {
		moveQuiet(e, di, state.ZLibrary, state.ZHand)
	}
	toMain1(t, e)
	addMana(t, e, 0, "RRRR") // X = 1

	submitChoices(t, e, striveCastOption(t, e, di).Index)
	d := e.Pending()
	pick := -1
	for _, o := range d.Options {
		if o.Kind == "x" && o.Label == "X = 1" {
			pick = o.Index
		}
	}
	if pick < 0 {
		t.Fatalf("no X = 1 announce: %+v", d)
	}
	submitChoices(t, e, pick)
	d = e.Pending()
	for _, o := range d.Options {
		if o.Obj == mine {
			t.Fatalf("your own creature offered as a target: %+v", d.Options)
		}
	}
	chooseStriveTargets(t, e, theirs)
	passUntilStackEmpty(t, e, 20)

	var tok state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o.IsToken {
			tok = id
		}
	}
	if tok == 0 || e.G.Obj(tok).Face().Name != "Raider" {
		t.Fatalf("no Raider token copy on seat 0's battlefield")
	}
	if !e.HasKeyword(tok, "Changeling") || !e.Derived(tok).AllCreatureTypes {
		t.Fatalf("token copy: changeling %v, all creature types %v; want both",
			e.HasKeyword(tok, "Changeling"), e.Derived(tok).AllCreatureTypes)
	}
	if !e.HasKeyword(tok, "Haste") {
		t.Fatal("token copy lacks haste")
	}
	if e.Derived(theirs).AllCreatureTypes || e.HasKeyword(theirs, "Changeling") {
		t.Fatal("the copied creature gained changeling")
	}
	replayCheck(t, e, cfg)
}
