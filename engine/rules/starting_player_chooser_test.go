package rules

// A Config.StartChooser names the seat posed CR 103.1's starting-player
// choice instead of the toss winner (MTR 2.2: the loser of the previous game
// of a match chooses), and TossWinner exports the toss itself as a pure
// function. A Config that does not set the field must be byte-identical to
// today: no golden, head or replay change (TestHeads, untouched, is that
// proof).

import (
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// chooserConfig is tossedTwoSeat with the configured chooser c.
func chooserConfig(t *testing.T, seed uint64, c int) Config {
	t.Helper()
	cfg := tossedTwoSeat(t, seed, 0)
	cfg.StartChooser = &c
	return cfg
}

// chooserNote returns the game's chooser genesis Note (the Note whose text
// ends " chooses who takes the first turn"), failing when there is none.
func chooserNote(t *testing.T, e *Engine) events.Event {
	t.Helper()
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.HasSuffix(ev.Text, " chooses who takes the first turn") {
			return ev
		}
	}
	t.Fatal("no chooser genesis Note in the log")
	return events.Event{}
}

// TestTossWinnerMatchesGenesis pins TossWinner to the engine: for a sweep of
// seeds at two seat counts, the exported pure function equals the seat the
// genesis toss Note names (Note.Player) and the seat AskStartingPlayer poses
// the choice to (the decision's Player) on a game with no StartChooser.
func TestTossWinnerMatchesGenesis(t *testing.T) {
	t.Parallel()
	sweeps := []struct{ seats, seeds int }{{2, 200}, {4, 50}}
	for _, sw := range sweeps {
		for seed := 1; seed <= sw.seeds; seed++ {
			want := TossWinner(uint64(seed), sw.seats)
			if want < 0 || want >= sw.seats {
				t.Fatalf("TossWinner(%d, %d) = %d, outside the seats", seed, sw.seats, want)
			}
			names := make([]string, sw.seats)
			decks := make([][]*cards.Card, sw.seats)
			for i := range names {
				names[i] = string(rune('a' + i))
				decks[i] = mountainDeck(t, 40)
			}
			cfg := Config{Seed: uint64(seed), Names: names, Decks: decks}
			e := NewStartingPlayerChoice(cfg)
			notes := tossNotes(e)
			if len(notes) != 1 {
				t.Fatalf("seed %d seats %d: %d toss Notes, want 1", seed, sw.seats, len(notes))
			}
			if got := int(notes[0].Player); got != want {
				t.Fatalf("seed %d seats %d: genesis toss seat %d, TossWinner %d", seed, sw.seats, got, want)
			}
			d := e.AskStartingPlayer()
			if d == nil {
				t.Fatalf("seed %d seats %d: no starting-player ask", seed, sw.seats)
			}
			if got := int(d.Player); got != want {
				t.Fatalf("seed %d seats %d: ask posed to seat %d, TossWinner %d", seed, sw.seats, got, want)
			}
			if d.Prompt != "You won the toss. Choose who takes the first turn." {
				t.Fatalf("seed %d seats %d: unconfigured prompt %q", seed, sw.seats, d.Prompt)
			}
		}
	}
}

// TestConfiguredChooserIsAsked: with StartChooser set, the choice is posed to
// the CONFIGURED seat (both constructors fold it as genesis's resolved toss
// so replay can rebuild a log that never recorded the ask), the prompt is the
// honest configured one, the genesis Note reads "<name> chooses who takes the
// first turn", and naming the other seat still starts that seat.
func TestConfiguredChooserIsAsked(t *testing.T) {
	t.Parallel()
	for _, c := range []int{0, 1} {
		for seed := 1; seed <= 50; seed++ {
			cfg := chooserConfig(t, uint64(seed), c)
			e := NewStartingPlayerChoice(cfg)
			note := chooserNote(t, e)
			if got := int(note.Player); got != c {
				t.Fatalf("seed %d chooser %d: Note names seat %d", seed, c, got)
			}
			if other := tossNotes(e); len(other) != 0 {
				t.Fatalf("seed %d chooser %d: %d toss Notes alongside the chooser Note", seed, c, len(other))
			}
			d := e.AskStartingPlayer()
			if d == nil {
				t.Fatalf("seed %d chooser %d: no starting-player ask", seed, c)
			}
			if d.Player != state.PlayerID(c) {
				t.Fatalf("seed %d chooser %d: ask posed to seat %d", seed, c, d.Player)
			}
			if d.Prompt != "Choose who takes the first turn." {
				t.Fatalf("seed %d chooser %d: prompt %q, want the configured one", seed, c, d.Prompt)
			}
			// Precondition: the other seat is offered, so the answer is a real
			// alternative to the default.
			other := state.PlayerID(1 - c)
			opt := optionForSeat(t, d, other)
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: state.PlayerID(c),
				Choices: []int{opt.Index}}); err != nil {
				t.Fatalf("seed %d chooser %d: submit: %v", seed, c, err)
			}
			e.Advance()
			if e.G.StartingPlayer != other || e.G.Active != other {
				t.Fatalf("seed %d chooser %d: starting %d active %d, want the chosen seat %d",
					seed, c, e.G.StartingPlayer, e.G.Active, other)
			}
		}
	}
}

// TestConfiguredChooserKeepsTheDeal: the toss is still drawn, so the same
// seed deals identical opening hands with and without a StartChooser.
func TestConfiguredChooserKeepsTheDeal(t *testing.T) {
	t.Parallel()
	for seed := 1; seed <= 25; seed++ {
		plain := tossedTwoSeat(t, uint64(seed), 0)
		chosen := chooserConfig(t, uint64(seed), 1)
		p := NewStartingPlayerChoice(plain)
		c := NewStartingPlayerChoice(chosen)
		for seat := 0; seat < 2; seat++ {
			want := p.G.Zone(state.ZHand, state.PlayerID(seat))
			got := c.G.Zone(state.ZHand, state.PlayerID(seat))
			if len(want) != 7 || len(got) != 7 {
				t.Fatalf("seed %d seat %d: %d opening cards with a chooser, %d without, want 7",
					seed, seat, len(got), len(want))
			}
			for i := range want {
				if want[i] != got[i] {
					t.Fatalf("seed %d seat %d card %d: object %d with a chooser, %d without",
						seed, seat, i, got[i], want[i])
				}
			}
		}
	}
}

// TestConfiguredChooserReplays: a configured game whose choice was really
// answered replays to the same chain head and rng draw count; replayed
// without the same StartChooser the log cannot be rebuilt (the field is
// load-bearing).
func TestConfiguredChooserReplays(t *testing.T) {
	t.Parallel()
	seed := uint64(1)
	cfg := chooserConfig(t, seed, 0)
	e := NewStartingPlayerChoice(cfg)
	d := e.AskStartingPlayer()
	if d == nil {
		t.Fatal("precondition: no starting-player ask on the configured game")
	}
	bot := newTestBot(seed)
	if err := e.Submit(bot.answer(e, d)); err != nil {
		t.Fatalf("the bot's own answer to the configured ask was rejected: %v", err)
	}
	e.Advance()
	passAll(t, e, 300)
	re, err := replayFor(cfg, e.L)
	if err != nil {
		t.Fatalf("replay the configured game: %v", err)
	}
	if re.L.Head() != e.L.Head() {
		t.Fatalf("replay chain %s, live %s", re.L.Head(), e.L.Head())
	}
	if re.RNGDraws() != e.RNGDraws() {
		t.Fatalf("replayed rng draws %d, live %d", re.RNGDraws(), e.RNGDraws())
	}
	if len(e.L.Intents) == 0 {
		t.Fatal("precondition: the played game recorded no intents to replay")
	}
	// Without the same StartChooser the rebuild diverges: the genesis Note
	// names a different seat and the ask goes to the toss winner, so the
	// recorded intents no longer fit.
	cfgNil := cfg
	cfgNil.StartChooser = nil
	reNil, err := replayFor(cfgNil, e.L)
	if err == nil && reNil.L.Head() == e.L.Head() {
		t.Fatalf("nil-chooser replay rebuilt head %s byte-identically; StartChooser is load-bearing", reNil.L.Head())
	}
}

// TestOutOfRangeChooserIsIgnored: a StartChooser outside the seats stands
// down -- the toss stands -- byte-identically to a nil one, in both
// constructors.
func TestOutOfRangeChooserIsIgnored(t *testing.T) {
	t.Parallel()
	out := 5
	for _, ctor := range []struct {
		name string
		new  func(Config) *Engine
	}{
		{"New", New},
		{"NewStartingPlayerChoice", NewStartingPlayerChoice},
	} {
		plain := tossedTwoSeat(t, 1, 0)
		chosen := tossedTwoSeat(t, 1, 0)
		chosen.StartChooser = &out
		eNil, eOut := ctor.new(plain), ctor.new(chosen)
		if eNil.L.Head() != eOut.L.Head() {
			t.Fatalf("%s: out-of-range chooser head %s, nil %s", ctor.name, eOut.L.Head(), eNil.L.Head())
		}
		if !reflect.DeepEqual(eNil.L.Events, eOut.L.Events) {
			t.Fatalf("%s: out-of-range chooser changed the event stream", ctor.name)
		}
		if !eNil.G.Over && startingPlayerAsked(eOut.L) != startingPlayerAsked(eNil.L) {
			t.Fatalf("%s: out-of-range chooser changed whether the ask is recorded", ctor.name)
		}
	}
}
