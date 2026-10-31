package forgeheuristic

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	_ "github.com/adams-shaun/gorge/bots/bot"
	"github.com/adams-shaun/gorge/bots/botpolicy"
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

	s, err := bots.New(Policy, bots.Options{Seed: 1, AutoPayMana: true})
	if err != nil {
		t.Fatalf("bots.New(%q): %v", Policy, err)
	}
	hb, ok := s.(*heuristic.Bot)
	if !ok {
		t.Fatalf("built seat is %T, want *heuristic.Bot", s)
	}
	want := botpolicy.TargetRules{LandPriority: true, ProminentColour: true, FilterFatalCopies: true}
	if got := hb.TargetRules(); got != want {
		t.Errorf("armed TargetRules = %+v, want %+v", got, want)
	}
	if got := hb.RemovalRules(); got != (botpolicy.RemovalRules{HoldForBigger: true}) {
		t.Errorf("armed RemovalRules = %+v, want HoldForBigger", got)
	}
	if got := hb.TimingRules(); got != (botpolicy.TimingRules{CounterTop: true, DrawTokenLate: true, PumpInCombat: true}) {
		t.Errorf("armed TimingRules = %+v, want CounterTop, DrawTokenLate and PumpInCombat", got)
	}
	if got := hb.AttachRules(); got != (botpolicy.AttachRules{AuraPolarity: true, Carrier: true, HoldOwnAura: true}) {
		t.Errorf("armed AttachRules = %+v, want AuraPolarity, Carrier and HoldOwnAura", got)
	}
	if got := hb.PostureRules(); got != (botpolicy.PostureRules{Attack: true, Reserve: true}) {
		t.Errorf("armed PostureRules = %+v, want Attack and Reserve", got)
	}
	if got := hb.MulliganRule(); got != botpolicy.MulliganForge {
		t.Errorf("armed MulliganRule = %s, want forge", got)
	}

	def, err := bots.New(bots.Default, bots.Options{Seed: 1})
	if err != nil {
		t.Fatalf("bots.New(%q): %v", bots.Default, err)
	}
	db, ok := def.(*heuristic.Bot)
	if !ok {
		t.Fatalf("default bot is %T, want *heuristic.Bot", def)
	}
	if got := db.MulliganRule(); got != botpolicy.MulliganForge {
		t.Errorf("default bot MulliganRule = %s, want forge", got)
	}
}

// TestForgeHeuristicPlaysAndReplays is the end-to-end contract: the policy
// plays a whole game against the production bot and the recorded log
// replays byte-identically.
func TestForgeHeuristicPlaysAndReplays(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 2)
	cfg := rules.Config{Seed: 11, Names: names, Decks: decks, Mulligans: 1}
	seats := func() []seat.Seat {
		forge, err := bots.New(Policy, bots.Options{Seed: 11, AutoPayMana: true})
		if err != nil {
			t.Fatalf("bots.New(%q): %v", Policy, err)
		}
		base, err := bots.New(bots.Default, bots.Options{Seed: 11, AutoPayMana: true})
		if err != nil {
			t.Fatalf("bots.New(%q): %v", bots.Default, err)
		}
		return []seat.Seat{forge, base}
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
