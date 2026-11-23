package replay

import (
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestScriptedStartReplay: a game opened by Config.Start (a scripted hand,
// battlefield, graveyard, library top and life) is played three turns by the
// bot and replays from (Config, log) alone to the same chain: the setup is
// genesis configuration, so the replay rebuilds it from the Config.
func TestScriptedStartReplay(t *testing.T) {
	raider := compileCard(t, raiderSrc)
	mountain := compileCard(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n")
	deck := func(first *cards.Card) []*cards.Card {
		d := make([]*cards.Card, 40)
		for i := range d {
			d[i] = mountain
		}
		d[0] = first
		return d
	}
	life := int32(15)
	start := &rules.StartSpec{Seats: []rules.StartSeat{{
		Hand:        []int{0, 1},
		Battlefield: []int{2, 3, 4},
		Tapped:      []int{3},
		Graveyard:   []int{5},
		Library:     []int{6, 7, 8},
		Life:        &life,
	}, {Battlefield: []int{1, 2}}}}
	cfg := rules.Config{
		Seed:  rules.SeedForStarter(1, 2, 0),
		Names: []string{"scripted", "plain"},
		Decks: [][]*cards.Card{deck(raider), deck(mountain)},
		Start: start,
	}
	e := rules.New(cfg)
	if e.G.StartingPlayer != 0 {
		t.Fatalf("starting player %d, want 0", e.G.StartingPlayer)
	}
	if got := len(e.G.Zone(state.ZBattlefield, 0)); got != 3 {
		t.Fatalf("scripted battlefield has %d permanents, want 3", got)
	}
	e.Advance()
	b := heuristic.NewBot(cfg.Seed)
	n := 0
	for !e.G.Over && e.Pending() != nil && e.G.Turn < 4 && n < maxIntents {
		in, err := botAnswer(b, e, e.Pending())
		if err != nil {
			t.Fatalf("bot Decide: %v", err)
		}
		if err := e.Submit(in); err != nil {
			t.Fatalf("Submit intent %d: %v", n, err)
		}
		n++
	}
	if e.G.Turn < 3 || len(e.L.Intents) == 0 {
		t.Fatalf("game stopped at turn %d after %d intents", e.G.Turn, len(e.L.Intents))
	}
	got, err := Replay(copyLog(e.L), cfg)
	if err != nil {
		t.Fatalf("replay of the scripted game: %v", err)
	}
	if got.L.Head() != e.L.Head() || len(got.L.Events) != len(e.L.Events) {
		t.Fatalf("replay chain %s (%d events), recorded %s (%d)", got.L.Head(), len(got.L.Events), e.L.Head(), len(e.L.Events))
	}
	// Without the Start in the Config the same log must NOT replay: the
	// setup is part of the chain, not an artifact of the bot.
	plain := cfg
	plain.Start = nil
	if _, err := Replay(copyLog(e.L), plain); err == nil {
		t.Fatal("the scripted log replayed without its Start: the setup is not in the chain")
	}
}
