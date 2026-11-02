package hiddeninfo

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
	"github.com/adams-shaun/gorge/testkit/bench"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestNoSeatReadsAHiddenName plays every repo deck, and the deck full of
// face-down cards against the decks that make a monarch, and at every decision
// checks all a seat and a spectator read: the projection, the decision's text
// and the new log.
func TestNoSeatReadsAHiddenName(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	names := testutil.RepoDeckNames()
	var pairs [][2]string
	for i := range names {
		pairs = append(pairs, [2]string{names[i], names[(i+1)%len(names)]})
	}
	for _, other := range []string{"death-n-taxes", "cavalry-charge", "mono-red-goblins", "uw-control", "dimir-tempo"} {
		pairs = append(pairs, [2]string{"deadly-disguise", other})
	}
	var decisions, faceDown, monarch, looks int
	var leaks []Leak
	for i, p := range pairs {
		cfg := rules.Config{Seed: uint64(i + 1), Names: []string{p[0], p[1]},
			Decks:  [][]*cards.Card{testutil.RepoDeck(t, reg, p[0]), testutil.RepoDeck(t, reg, p[1])},
			Tokens: reg.Tokens, NameUniverse: reg.Universe(), Mulligans: 1}
		var e *rules.Engine
		var o *Oracle
		hooks := bench.Hooks{
			Setup: func(en *rules.Engine) { e, o = en, New(en) },
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
					leaks = append(leaks, o.CheckView(sp, &v)...)
					leaks = append(leaks, o.CheckEvents(sp, view.Seat, evs)...)
				}
				v := view.ProjectFor(e.G, e, view.NoSeat, view.Public, nil)
				leaks = append(leaks, o.CheckView(view.NoSeat, &v)...)
				leaks = append(leaks, o.CheckEvents(view.NoSeat, view.Public, evs)...)
				if AnyFaceDown(e.G) {
					faceDown++
				}
				if d.ResumeKind == "look_ack" {
					looks++
				}
				for _, ev := range evs {
					if ev.Counter == "__monarch_draw" {
						monarch++
					}
				}
				return nil
			},
		}
		seats := []seat.Seat{heuristic.NewBot(cfg.Seed), heuristic.NewBot(cfg.Seed + 1)}
		if _, _, err := bench.PlayGame(cfg, seats, 60, 8000, hooks); err != nil {
			t.Fatalf("%v: %v", p, err)
		}
		for j, l := range leaks {
			if j == 20 {
				t.Errorf("%v, seed %d: %d more", p, cfg.Seed, len(leaks)-j)
				break
			}
			t.Errorf("%v, seed %d: %s", p, cfg.Seed, l)
		}
		leaks = leaks[:0]
	}
	t.Logf("%d games, %d decisions: %d with a face-down card, %d monarch draws, %d looks", len(pairs), decisions, faceDown, monarch, looks)
	if faceDown == 0 || monarch == 0 || looks == 0 {
		t.Errorf("the games stopped reaching what this test is for: %d with a face-down card, %d monarch draws, %d looks", faceDown, monarch, looks)
	}
}
