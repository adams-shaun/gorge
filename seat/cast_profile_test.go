package seat

// The L2 cast-profile whole-game pins. Two properties, over full acceptance
// games (not synthetic decisions):
//
//  1. cast-profile on the EMBEDDED default profile is intent-identical to
//     bot, on BOTH adapters (the view-shaped Decide a real client drives and
//     the game-shaped DecideBoard the host drives) — the equality the bench's
//     `-a cast-profile -b bot == -a bot -b bot` baseline needs;
//  2. the wiring is NOT vacuous: a tuned profile parsed out of JSON reaches
//     the scorer through seat.Bot.decide's Board.Cast set and changes play
//     (the bot's whole-game intent stream is not reproduced), so if the
//     equality in (1) ever degenerated into both sides ignoring the profile,
//     this second leg fails first.

import (
	"context"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/view"
)

// holdEverything is a tuned profile that holds every cast: the only nonzero
// weights are NonCreatureCMC (so the profile is not the all-zero shape the
// Board zero-value contract folds back into DefaultCastWeights) and a
// CastThreshold far above any achievable score, so chooseCast returns -1 on
// every priority decision. A whole game played under it casts NOTHING, which
// is why its intent stream provably differs from the production bot's.
const holdEverything = `{"version":1,"cast":{"NonCreatureCMC":1,"CastThreshold":10000}}`

// playWholeGame drives one full game with per-seat bots built by newBot
// (seeded seed^(k+1), the host/bench derivation) through the named adapter
// half, returning every submitted intent in order.
func playWholeGame(t *testing.T, seed uint64, names []string, decks [][]*cards.Card, newBot func(uint64) *Bot, viewShaped bool) []decision.Intent {
	t.Helper()
	e := rules.New(rules.Config{Seed: seed, Names: names, Decks: decks})
	e.Advance()
	bots := make([]*Bot, len(decks))
	for k := range bots {
		bots[k] = newBot(seed ^ uint64(k+1))
	}
	var intents []decision.Intent
	n := 0
	for !e.G.Over && e.Pending() != nil && n < 20000 {
		d := e.Pending()
		var in decision.Intent
		var err error
		if viewShaped {
			in, err = bots[d.Player].Decide(context.Background(), view.Project(e.G, e, d.Player, d), *d)
		} else {
			in, err = bots[d.Player].DecideBoard(context.Background(), botpolicy.BoardFromGame(e.G, e, d.Player), *d)
		}
		if err != nil {
			t.Fatalf("intent %d: %v", n, err)
		}
		if err := e.Submit(in); err != nil {
			t.Fatalf("intent %d: submit: %v", n, err)
		}
		intents = append(intents, in)
		n++
	}
	if !e.G.Over {
		t.Fatalf("game did not terminate after %d intents (turn %d)", n, e.G.Turn)
	}
	return intents
}

func newBotFromProfile(t *testing.T, doc string) func(uint64) *Bot {
	t.Helper()
	w, err := botpolicy.ParseCastProfile([]byte(doc))
	if err != nil {
		t.Fatalf("ParseCastProfile: %v", err)
	}
	return func(seed uint64) *Bot { return NewCastProfileBotWithWeights(seed, w) }
}

func newDefaultProfileBot(t *testing.T) func(uint64) *Bot {
	t.Helper()
	return func(seed uint64) *Bot {
		b, err := NewCastProfileBot(seed)
		if err != nil {
			t.Fatalf("NewCastProfileBot: %v", err)
		}
		return b
	}
}

// TestCastProfileDefaultMatchesBotOverWholeGame is the brief's whole-game
// identity pin: on each adapter half, cast-profile playing the embedded
// default profile submits exactly the intents bot does over a full game, so
// the two chains reach the same head and the bench baseline
// `-a cast-profile -b bot` measures the same games as `-a bot -b bot`.
func TestCastProfileDefaultMatchesBotOverWholeGame(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 4)
	const seed = 9
	profile := newDefaultProfileBot(t)
	for _, adapter := range []struct {
		name string
		view bool
	}{
		{name: "board", view: false},
		{name: "view", view: true},
	} {
		botRun := playWholeGame(t, seed, names, decks, NewBot, adapter.view)
		profileRun := playWholeGame(t, seed, names, decks, profile, adapter.view)
		if len(botRun) == 0 {
			t.Fatalf("%s: bot run produced no intents", adapter.name)
		}
		if !slices.EqualFunc(botRun, profileRun, func(a, b decision.Intent) bool {
			return a.Seq == b.Seq && a.Player == b.Player && slices.Equal(a.Choices, b.Choices)
		}) {
			for i := range botRun {
				if i >= len(profileRun) || botRun[i].Seq != profileRun[i].Seq || botRun[i].Player != profileRun[i].Player || !slices.Equal(botRun[i].Choices, profileRun[i].Choices) {
					t.Fatalf("%s: intent %d diverged: bot %+v vs cast-profile %+v", adapter.name, i, botRun[i], profileRun[i])
				}
			}
			t.Fatalf("%s: cast-profile produced %d intents, bot %d", adapter.name, len(profileRun), len(botRun))
		}
	}
}

// TestCastProfileTunedProfileChangesPlay is the non-vacuity leg: the same
// adapter and seeds, but a tuned profile that holds every cast (threshold
// 10000) submits a DIFFERENT intent stream from the production bot's — the
// proof that seat.Bot.decide actually routes the parsed profile into the
// cast scorer rather than silently dropping it (in which case both sides
// would play the default arithmetic and the equality pin above would hold
// for the wrong reason).
func TestCastProfileTunedProfileChangesPlay(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 4)
	const seed = 9
	botRun := playWholeGame(t, seed, names, decks, NewBot, true)
	holdRun := playWholeGame(t, seed, names, decks, newBotFromProfile(t, holdEverything), true)
	if slices.EqualFunc(botRun, holdRun, func(a, b decision.Intent) bool {
		return a.Seq == b.Seq && a.Player == b.Player && slices.Equal(a.Choices, b.Choices)
	}) {
		t.Fatalf("hold-everything profile reproduced the production bot's %d intents — the profile never reached the scorer", len(botRun))
	}
}
