package conformance

import (
	"cmp"
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/adapter"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/manabrew/mbtest"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const benchDecks = "../../dirac/testdata/bench/fdn"

// diracEngine plays the fdn tables on surface, skipping where they are not built.
func diracEngine(t *testing.T, reg *cards.Registry, surface dirac.Surface) *adapter.Engine {
	t.Helper()
	e, err := adapter.New([]string{"fdn"}, reg, surface)
	if err != nil {
		t.Skip(err)
	}
	return e
}

// benchConfig is game g of the bench decks, each against the next.
func benchConfig(t *testing.T, reg *cards.Registry, g int) game.Config {
	t.Helper()
	names := testutil.DeckDirNames(t, benchDecks)
	c := game.Config{Seed: uint64(1000 + g), Tokens: reg.Tokens, NameUniverse: reg.Universe()}
	for i := range 2 {
		name := names[(g+i)%len(names)]
		_, d, err := deck.Load(reg, filepath.Join(benchDecks, name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		c.Names = append(c.Names, name)
		c.Decks = append(c.Decks, d)
	}
	return c
}

// TestManaBrewServesADiracGame plays whole games over the ManaBrew wire on dirac
// tables, every seat a seeded client.
func TestManaBrewServesADiracGame(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := diracEngine(t, reg, dirac.SurfaceFull)
	for g := range 4 {
		t.Run(fmt.Sprintf("game%d", g), func(t *testing.T) {
			c := benchConfig(t, reg, g)
			s := ManaBrew{}.Open(t, e, c).(*ManaBrewSession)
			pick := Random(c.Seed)
			frames := 0
			for {
				f, more := s.Next()
				if !more {
					break
				}
				frames++
				for offered := f; ; {
					if len(offered.Answers) == 0 {
						t.Fatalf("seed %d, frame %d (%s): every answer on offer was refused", c.Seed, frames, f.Key)
					}
					key := pick(offered)
					if err := s.Answer(key); err == nil {
						break
					}
					offered.Answers = slices.DeleteFunc(slices.Clone(offered.Answers), func(a string) bool { return a == key })
				}
			}
			if frames < minFrames {
				t.Fatalf("the game finished after %d frames, want at least %d", frames, minFrames)
			}
			t.Logf("%s vs %s: %d frames", c.Names[0], c.Names[1], frames)
		})
	}
}

// acrossEngines is a wire whose frames compare across two engines. What it takes off
// differs by engine for a named reason:
//
//   - A table's CR 103.1 ask: gorge's host poses it and dirac has the toss winner go
//     first (Caps.StartingAsk), so the winner answering with its own seat is the same
//     game.
//   - Each player's available, a census no client reads that the kernel does not keep,
//     and potential_actions, which gorge also fills for a seat without priority.
//   - A card's keywords off the battlefield, on the stack included: gorge's derived
//     keywords list a battlefield-only static's grant on them (CR 109.2).
//   - A card's keywords order, and blocked_by as anything but a count: gorge zeroes a
//     blocker that has left.
//   - An option's player when it names no player: gorge leaves it zero in most asks,
//     and nothing reads it there.
//   - Facts the adapter does not read yet, which change no answer: a target effect's
//     damage amount unless it is literal, and its statics; an unless-pay option's
//     wording; a hand card's effective_mana_cost.
//   - The last frame but for its result: a player who lost leaves the game with
//     their objects (CR 800.4a) on gorge, and dirac stops at the result.
type acrossEngines struct{ Native }

func (w acrossEngines) Open(t testing.TB, e game.Engine, c game.Config) Session {
	return &acrossSession{Session: w.Native.Open(t, e, c), t: t, asks: e.Caps().StartingAsk}
}

type acrossSession struct {
	Session
	t    testing.TB
	asks bool
}

// resultKeys and playerResultKeys are what the last frame compares.
var (
	resultKeys       = []string{"over", "winner", "draw", "turn", "step", "players"}
	playerResultKeys = []string{"life", "lost"}
)

func keepOnly(m map[string]any, keys []string) {
	for k := range m {
		if !slices.Contains(keys, k) {
			delete(m, k)
		}
	}
}

func (s *acrossSession) Next() (Frame, bool) {
	if s.asks {
		s.asks = false
		f, _ := s.Session.Next()
		seat, _, _ := strings.Cut(f.Key, "/")
		if err := s.Session.Answer(strings.TrimPrefix(seat, "seat")); err != nil {
			s.t.Fatalf("the toss winner taking the first turn: %v", err)
		}
	}
	f, more := s.Session.Next()
	for _, viewer := range f.State {
		v, _ := viewer.(map[string]any)
		card := func(zone string, c map[string]any) {
			if zone != "battlefield" {
				delete(c, "keywords")
			}
			delete(c, "effective_mana_cost")
			if b, ok := c["blocked_by"].([]any); ok {
				c["blocked_by"] = len(b)
			}
			if k, ok := c["keywords"].([]any); ok {
				slices.SortFunc(k, func(a, b any) int { return cmp.Compare(a.(string), b.(string)) })
			}
		}
		eachZoneCard(v, card)
		stack, _ := v["stack"].([]any)
		for _, e := range stack {
			if c, ok := e.(map[string]any)["card"].(map[string]any); ok {
				card("stack", c)
			}
		}
		if !more {
			keepOnly(v, resultKeys)
		}
		if d, ok := v["decision"].(map[string]any); ok {
			if te, ok := d["target_effect"].(map[string]any); ok {
				delete(te, "damage")
				delete(te, "statics")
			}
			options, _ := d["options"].([]any)
			for _, o := range options {
				m := o.(map[string]any)
				if m["kind"] != "player" {
					delete(m, "player")
				}
				if m["mode"] == decision.ModeUnlessPay {
					delete(m, "label")
				}
			}
		}
		players, _ := v["players"].([]any)
		for _, p := range players {
			m, _ := p.(map[string]any)
			delete(m, "available")
			delete(m, "potential_actions")
			if !more {
				keepOnly(m, playerResultKeys)
			}
		}
	}
	return f, more
}

// TestNativeWireMatchesGorge plays games on gorge and on dirac through the native wire:
// same seed, same answers, every frame compared.
func TestNativeWireMatchesGorge(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := diracEngine(t, reg, dirac.SurfaceFull)
	for g := range 16 {
		t.Run(fmt.Sprintf("game%d", g), func(t *testing.T) {
			t.Parallel()
			c := benchConfig(t, reg, g)
			if d := Run(t, acrossEngines{Native{OfferedOnly: true}}, gorgegame.Engine{}, e, c, Random(c.Seed)); d != nil {
				t.Fatal(d)
			}
		})
	}
}

// playSeats plays c on e with one seat per player and returns the game's log head.
func playSeats(t *testing.T, e game.Engine, c game.Config, seats []seat.Seat) string {
	t.Helper()
	g, err := e.New(c)
	if err != nil {
		t.Fatal(err)
	}
	for d := g.Pending(); d != nil; d = g.Pending() {
		v := view.Project(g.Board(), g.Chars(), d.Player, d)
		v.Round = view.RoundOf(g.Board(), g.Log().Events)
		in, err := seats[d.Player].Decide(context.Background(), v, *d)
		if err != nil {
			t.Fatal(err)
		}
		if err := g.Submit(in); err != nil {
			t.Fatalf("seq %d: %v", d.Seq, err)
		}
	}
	if !g.Board().Over {
		t.Fatal("the game stopped before its end")
	}
	return g.Log().Head()
}

// TestBotThroughManaBrewOnDirac has the production bot answer through the ManaBrew
// translator on dirac tables: it plays the game it plays with no wire in between.
func TestBotThroughManaBrewOnDirac(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := diracEngine(t, reg, dirac.SurfaceFull)
	for g := range 8 {
		t.Run(fmt.Sprintf("game%d", g), func(t *testing.T) {
			c := benchConfig(t, reg, g)
			native := playSeats(t, e, c, []seat.Seat{heuristic.NewBot(c.Seed), heuristic.NewBot(c.Seed)})
			census := mbtest.NewCensus()
			wire := func() seat.Seat { return mbtest.NewBotWireSeat("dirac", int64(g), c.Seed, census) }
			if head := playSeats(t, e, c, []seat.Seat{wire(), wire()}); head != native {
				t.Fatalf("seed %d: through the wire %s, with no wire %s", c.Seed, head, native)
			}
			if census.TotalUnmapped()+census.TotalRejected()+census.TotalFallback() != 0 {
				t.Fatal(census.Summary())
			}
		})
	}
}
