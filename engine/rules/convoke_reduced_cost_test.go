package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestConvokeAnnouncementPricesReducedCost replays the hunt seed 777
// situation (mtgsim search leg, 4 seats, the az-redeal seat casting): City
// on Fire (Convoke, 5 R R R) announced under two {1} cost reductions --
// Herald of the Pantheon's enchantment spells and Goblin Anarchomancer's
// red-or-green spells -- with the pool floated and no untapped mana source
// left. The bot's announcement must be priced against the REDUCED
// outstanding cost (the CR 601.2f composition the absorb gate walks), not
// the printed one: printed pricing taps five generic creatures against a
// three-generic outstanding, so validateCastContributions rejects the
// intent ("announcement reduces nothing"), and for a bot seat a rejected
// intent is a game failure (mtgsim search: "intent 1771 rejected").
func TestConvokeAnnouncementPricesReducedCost(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, n := range []string{"City on Fire", "Goblin Anarchomancer", "Herald of the Pantheon", "Bear Cub", "Goblin Piker"} {
		if _, ok := reg.Lookup(n); !ok {
			t.Fatalf("corpus fixture %q is missing", n)
		}
	}
	city := corpusCard(t, "City on Fire")
	e := handEngine(t, city)
	// The board, in battlefield order: the two reduction statics first, then
	// five green creatures (generic-only convoke candidates), then one red
	// pip candidate. No lands anywhere: the pool is the only mana, so the
	// policy's helper selection is what pays the spell.
	var ids []state.ObjID
	board := []string{"Goblin Anarchomancer", "Herald of the Pantheon", "Bear Cub", "Bear Cub", "Bear Cub", "Bear Cub", "Bear Cub", "Goblin Piker"}
	for _, name := range board {
		o := e.G.AddObject(corpusCard(t, name), 0)
		o.Zone = state.ZBattlefield
		ids = append(ids, o.ID)
	}
	e.G.SetZone(state.ZBattlefield, 0, ids)
	if n := len(e.G.Zone(state.ZBattlefield, 0)); n != len(board) {
		t.Fatalf("precondition: %d permanents on the battlefield, want %d", n, len(board))
	}
	e.G.Players[0].Pool[state.MR] = 2
	e.priorityRound()

	// Announce the cast at priority.
	d := e.Pending()
	cityID := e.G.Zone(state.ZHand, 0)[0]
	castIdx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == cityID {
			castIdx = o.Index
		}
	}
	if castIdx < 0 {
		t.Fatalf("precondition: no cast option for City on Fire: %+v", d.Options)
	}
	d = e.Pending()
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == cityID {
			castIdx = o.Index
		}
	}
	if castIdx < 0 {
		t.Fatalf("precondition: no cast option for the hand's City on Fire (%d): %+v", cityID, d.Options)
	}
	in := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{castIdx}}
	if err := e.Submit(in); err != nil {
		t.Fatalf("precondition: the cast announce was rejected: %v", err)
	}

	// The Convoke announcement ask.
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Prompt != "Choose creatures to help pay for City on Fire" {
		t.Fatalf("precondition: pending is %+v, want the City on Fire convoke ask", d)
	}
	// The ask's own bound is the reduced cost's slot count: 3 coloured pips
	// + 3 generic (5 R R R less Herald's {1} and Anarchomancer's {1}), so a
	// printed-cost answer clamps to 6 picks and over-selects the generic.
	if d.Max != 6 {
		t.Fatalf("precondition: convoke ask Max = %d, want the reduced cost's 6 slots", d.Max)
	}

	// The bot answers; the engine's absorb gate must accept it, and the
	// payment must carry the spell to the battlefield.
	bot := newTestBot(7)
	in = bot.answer(e, d)
	if err := e.Submit(in); err != nil {
		t.Fatalf("the bot's convoke announcement was rejected: %v", err)
	}
	for i := 0; i < 20; i++ {
		d = e.Pending()
		if d == nil {
			break
		}
		in = bot.answer(e, d)
		if err := e.Submit(in); err != nil {
			t.Fatalf("decision %s rejected after the announcement: %v", d.Kind, err)
		}
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.SpellCastReversed {
			t.Fatalf("the cast reversed: %+v", ev)
		}
	}
	cityObj := e.G.Obj(cityID)
	if cityObj == nil || cityObj.Zone != state.ZBattlefield {
		t.Fatalf("City on Fire did not resolve to the battlefield: %+v", city)
	}
}
