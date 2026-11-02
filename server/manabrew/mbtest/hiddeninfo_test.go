package mbtest

import (
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	manabrew "github.com/adams-shaun/gorge/server/manabrew"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/bench"
	"github.com/adams-shaun/gorge/testkit/hiddeninfo"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestManaBrewPayloadsNameNoHiddenCard checks what the wire sends each seat at
// every decision, in games with face-down cards and a monarch: the state
// message, the prompt and the synthetic reveal prompts.
func TestManaBrewPayloadsNameNoHiddenCard(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	pairs := [][2]string{{"deadly-disguise", "death-n-taxes"}, {"deadly-disguise", "cavalry-charge"}, {"deadly-disguise", "uw-control"}, {"ur-delver", "uw-control"}, {"uw-control", "uw-tempo"}}
	var decisions, faceDown int
	for i, p := range pairs {
		cfg := rules.Config{Seed: uint64(i + 1), Names: []string{p[0], p[1]},
			Decks:  [][]*cards.Card{testutil.RepoDeck(t, reg, p[0]), testutil.RepoDeck(t, reg, p[1])},
			Tokens: reg.Tokens, NameUniverse: reg.Universe(), Mulligans: 1}
		tr := manabrew.New("leak", int64(cfg.Seed), nil)
		var e *rules.Engine
		var o *hiddeninfo.Oracle
		hooks := bench.Hooks{
			Setup: func(en *rules.Engine) { e, o = en, hiddeninfo.New(en) },
			Decision: func(_ int, d *decision.Decision, _ decision.Intent, _ *botpolicy.Board) error {
				decisions++
				evs := o.Observe(d)
				for s := range e.G.Players {
					sp := state.PlayerID(s)
					var asked *decision.Decision
					if d.Player == sp {
						asked = d
					}
					v := view.ProjectFor(e.G, e, sp, view.Seat, asked)
					// A seat's own deck list is its own to read.
					v.OwnDeck = nil
					leaks := o.CheckJSON(sp, "state", tr.State(v))
					if asked != nil && !hiddeninfo.PublicChoice(asked) {
						msg, err := tr.Prompt(asked, &v)
						if err != nil {
							t.Fatal(err)
						}
						leaks = append(leaks, o.CheckJSON(sp, "prompt", msg)...)
					}
					bodies := make([]protocol.EventBody, 0, len(evs))
					for _, ev := range evs {
						r := view.RedactEventFor(e.G, ev, sp, view.Seat)
						bodies = append(bodies, protocol.EventBody{Event: protocol.EventFrom(r), Line: view.Describe(e.G, r)})
					}
					leaks = append(leaks, o.CheckJSON(sp, "synthetic", tr.Synthetic(bodies, &v))...)
					for _, l := range leaks {
						t.Errorf("%v, seed %d: %s", p, cfg.Seed, l)
					}
				}
				if hiddeninfo.AnyFaceDown(e.G) {
					faceDown++
				}
				return nil
			},
		}
		seats := []seat.Seat{heuristic.NewBot(cfg.Seed), heuristic.NewBot(cfg.Seed + 1)}
		if _, _, err := bench.PlayGame(cfg, seats, 60, 8000, hooks); err != nil {
			t.Fatalf("%v: %v", p, err)
		}
	}
	t.Logf("%d games, %d decisions, %d with a face-down card", len(pairs), decisions, faceDown)
	if faceDown == 0 {
		t.Error("the games stopped reaching a face-down card")
	}
}
