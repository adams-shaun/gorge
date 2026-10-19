package forgeheuristic

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	_ "github.com/adams-shaun/gorge/bots/bot"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/replay"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/testkit/bench"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestForgeHeuristicRegistered pins the registry entry and the factory's
// policy shape: experimental, same formats as the production bot, and a
// built seat that is the heuristic bot.
func TestForgeHeuristicRegistered(t *testing.T) {
	e, ok := bots.Lookup(Policy)
	if !ok {
		t.Fatalf("bots.Lookup(%q): not registered (known: %v)", Policy, bots.Names())
	}
	if e.Tier != bots.Experimental {
		t.Errorf("Tier = %q, want experimental", e.Tier)
	}
	if e.Name != Policy || e.Label == "" || e.Description == "" {
		t.Errorf("Info = %+v, want a name, label and description", e.Info)
	}
	if len(e.Strength) == 0 || e.Strength[0].Claim == "" || e.Strength[0].Source == "" {
		t.Errorf("Strength = %+v, want the measured claim and source", e.Strength)
	}
	botEntry, ok := bots.Lookup(bots.Default)
	if !ok {
		t.Fatal("the production bot is not linked into this test binary")
	}
	if !reflect.DeepEqual(e.Formats, botEntry.Formats) || e.MaxSeats != botEntry.MaxSeats {
		t.Errorf("Formats/MaxSeats = %v/%d, want the bot's %v/%d", e.Formats, e.MaxSeats, botEntry.Formats, botEntry.MaxSeats)
	}
	if err := bots.Validate(); err != nil {
		t.Errorf("bots.Validate: %v", err)
	}

	s, err := New(bots.Options{Seed: 1, AutoPayMana: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, ok := s.(*heuristic.Bot); !ok {
		t.Errorf("built seat is %T, want *heuristic.Bot", s)
	}
}

// TestForgeHeuristicPlaysAndReplays is the end-to-end contract: the policy
// plays a whole game against the production bot and the recorded log
// replays byte-identically.
func TestForgeHeuristicPlaysAndReplays(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 2)
	cfg := rules.Config{Seed: 11, Names: names, Decks: decks, Mulligans: 1}
	seats := func() []seat.Seat {
		s, err := New(bots.Options{Seed: 11, AutoPayMana: true})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		return []seat.Seat{s, heuristic.NewBot(11).EnableAutoPayMana()}
	}
	_, e, err := bench.PlayGame(cfg, seats(), 200, 40000, bench.Hooks{})
	if err != nil {
		t.Fatalf("PlayGame: %v", err)
	}
	if !e.G.Over {
		t.Fatalf("game did not finish: turn %d", e.G.Turn)
	}
	rep, err := replay.Replay(e.L, cfg)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if rep.L.Head() != e.L.Head() {
		t.Fatalf("replay head %s, live head %s", rep.L.Head(), e.L.Head())
	}
}
