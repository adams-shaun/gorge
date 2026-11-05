package host

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/explain"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// kindsGame plays one game for a human seat driven by the heuristic bot, asks
// the paired bot about every decision, and hands each decision and its advice to
// see. It stops after maxAsks decisions.
func kindsGame(t *testing.T, humanDeck, oppDeck string, seed uint64, maxAsks int, see func(*decision.Decision, protocol.Advice)) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	own, opp := testutil.RepoDeck(t, reg, humanDeck), testutil.RepoDeck(t, reg, oppDeck)
	o := testOptions(t)
	o.LoadDeck = func(name string) (Deck, error) {
		if name == "human" {
			return Deck{Name: "own", Cards: own}, nil
		}
		return Deck{Name: "opposing", Cards: opp}, nil
	}
	o.PairedBot = pairedTestEntry(t)
	o.Seats = func([]string, uint64) []seat.Seat { return []seat.Seat{passSeat{}, heuristic.NewBot(seed + 1)} }
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	c := pairedTable([]int{0})
	c.Seed, c.Mulligans = seed, 7
	if err := r.AddTable(c); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	bot := heuristic.NewBot(seed + 2)
	var after uint64
	for i := 0; i < maxAsks; i++ {
		d := censusParked(r, after)
		if d == nil {
			return
		}
		a, err := r.Advice(context.Background(), "t1", 1, 0, d.Seq)
		if err != nil {
			t.Fatalf("seq %d (%s): %v", d.Seq, d.Kind, err)
		}
		see(d, a)
		_, m, _ := r.lookup("t1", 1)
		m.mu.RLock()
		brd := botpolicy.BoardFromGameInto(m.e.G, m.e, 0, new(botpolicy.Board)).Detached()
		m.mu.RUnlock()
		in, err := bot.DecideBoard(context.Background(), brd, *d)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.SubmitIntent("t1", 1, 0, in); err != nil {
			if err := r.SubmitIntent("t1", 1, 0, minimalAnswer(d)); err != nil {
				t.Fatal(err)
			}
		}
		after = d.Seq
	}
}

// Every kind of decision a human meets gets an answer: the search's when it can
// rank the options, otherwise the bot's rules with the reason search did not
// weigh it. None of them is left as "none", a rules answer names the options
// it picks and carries no search, and the keep-or-mulligan ask is searched.
func TestPairedBotAnswersEveryKindAHumanMeets(t *testing.T) {
	if testing.Short() {
		t.Skip("plays whole games")
	}
	seen := map[decision.Kind]int{}
	rules := map[decision.Kind]int{}
	for i, pair := range [][2]string{{"uw-control", "ur-delver"}, {"death-n-taxes", "dimir-tempo"}} {
		kindsGame(t, pair[0], pair[1], uint64(20261103+i), 350, func(d *decision.Decision, a protocol.Advice) {
			seen[d.Kind]++
			if a.State == protocol.AdviceNone {
				t.Errorf("%s decision %d (%q): none (%s)", d.Kind, d.Seq, d.Prompt, a.Reason)
			}
			if a.State == protocol.AdviceRefused {
				t.Errorf("%s decision %d (%q): refused (%s)", d.Kind, d.Seq, d.Prompt, a.Reason)
			}
			if a.State != protocol.AdviceReady {
				return
			}
			var n explain.Note
			if err := json.Unmarshal(a.Explain, &n); err != nil {
				t.Fatal(err)
			}
			if d.Kind == decision.KMulligan && len(d.Options) == 2 && d.Options[1].Kind == "mulligan" && (n.Search == nil || n.Search.Sims == 0) {
				t.Errorf("mulligan decision %d (%q): not searched (%s)", d.Seq, d.Prompt, a.Reason)
			}
			if n.Search == nil {
				rules[d.Kind]++
				if a.Reason == "" {
					// A forced answer carries no reason; everything else says why.
					if len(d.Options) > 1 && d.Kind != decision.KPriority {
						t.Errorf("%s decision %d: a rules answer with no reason", d.Kind, d.Seq)
					}
				}
				if len(n.Bot) == 0 && d.Min > 0 {
					t.Errorf("%s decision %d: a rules answer picks nothing", d.Kind, d.Seq)
				}
				for _, c := range n.Bot {
					found := false
					for _, o := range d.Options {
						found = found || o.Index == c
					}
					if !found {
						t.Errorf("%s decision %d: picks option %d, which is not offered", d.Kind, d.Seq, c)
					}
				}
			}
		})
	}
	for _, k := range []decision.Kind{decision.KMulligan, decision.KModes, decision.KTriggerOptional, decision.KChoose, decision.KTarget, decision.KPriority} {
		if seen[k] == 0 {
			t.Errorf("fixture: no %s decision was asked", k)
		}
	}
	t.Logf("asked %v; answered by rules %v", seen, rules)
}

// Advice reads only what the seat holds: two games that differ in the
// opponent's list, and so its hidden cards, give the same bytes before the
// opponent shows a card -- for the searched London mulligan, whose worlds deal
// the opponent's cards from the belief, and for a rules answer, the choice of
// who plays first.
func TestPairedBotRulesAdviceIgnoresHiddenOpponentCards(t *testing.T) {
	first := func(opp string) (mull, toss []byte) {
		// The human wins the toss at this seed and chooses who plays first.
		kindsGame(t, "uw-control", opp, 20261104, 3, func(d *decision.Decision, a protocol.Advice) {
			b, _ := json.Marshal(a)
			switch d.Kind {
			case decision.KMulligan:
				mull = append(mull, b...)
			case decision.KStartingPlayer:
				toss = append(toss, b...)
			}
		})
		if len(mull) == 0 || len(toss) == 0 {
			t.Fatalf("fixture: asked %d bytes of mulligan and %d of who plays first, want both", len(mull), len(toss))
		}
		return mull, toss
	}
	am, at := first("mono-red-goblins")
	bm, bt := first("mono-blue-tempo")
	if !bytes.Equal(am, bm) {
		t.Fatalf("the mulligan advice differs with the opponent's list:\n%s\n%s", am, bm)
	}
	if !strings.Contains(string(am), `"search":`) {
		t.Fatalf("the mulligan advice carries no search: %s", am)
	}
	if !bytes.Equal(at, bt) {
		t.Fatalf("the advice on who plays first differs with the opponent's list:\n%s\n%s", at, bt)
	}
	if !strings.Contains(string(at), "rules would do") {
		t.Fatalf("the advice on who plays first does not say it is the bot's rules: %s", at)
	}
}

// Words for each kind search cannot rank: every one says what the bot's rules
// do, and none quotes a report name.
func TestRulesReasonNamesEachKind(t *testing.T) {
	for _, k := range decision.Kinds {
		d := &decision.Decision{Kind: k}
		w := rulesReason(d)
		if !strings.HasSuffix(w, "This is what the bot's rules would do.") || strings.Contains(w, "_") {
			t.Errorf("%s: %q", k, w)
		}
	}
}
