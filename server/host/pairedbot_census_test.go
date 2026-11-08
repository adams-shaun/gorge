package host

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azredeal"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestPairedBotCensus plays whole games with a heuristic driver on the human
// seat that asks the paired bot about every decision it meets, and prints one
// tab-separated row per request. It is a measurement, not a check: it runs only
// when GORGE_ADVICE_CENSUS names an output file.
//
//	GORGE_ADVICE_CENSUS=out.tsv GORGE_ADVICE_DECKS=a,b GORGE_ADVICE_GAMES=4 \
//	  GORGE_ADVICE_SIMS=100 GORGE_ADVICE_MANUAL=1 go test -run TestPairedBotCensus ./server/host
func TestPairedBotCensus(t *testing.T) {
	out := os.Getenv("GORGE_ADVICE_CENSUS")
	if out == "" {
		t.Skip("GORGE_ADVICE_CENSUS is not set")
	}
	decks := strings.Split(os.Getenv("GORGE_ADVICE_DECKS"), ",")
	if len(decks) != 2 {
		t.Fatal("GORGE_ADVICE_DECKS wants the human's deck and the opponent's, comma-separated")
	}
	games, _ := strconv.Atoi(os.Getenv("GORGE_ADVICE_GAMES"))
	sims, _ := strconv.Atoi(os.Getenv("GORGE_ADVICE_SIMS"))
	if sims == 0 {
		sims = 100
	}
	manual := os.Getenv("GORGE_ADVICE_MANUAL") == "1"
	maxAsks := 400
	f, err := os.OpenFile(out, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	reg := testutil.CorpusRegistry(t)
	own := testutil.RepoDeck(t, reg, decks[0])
	opp := testutil.RepoDeck(t, reg, decks[1])
	commander := os.Getenv("GORGE_ADVICE_FORMAT") == "commander"
	deckOf := func(name string, cards []*cards.Card) Deck {
		d := Deck{Name: name, Cards: cards}
		if commander {
			d.Commanders = testutil.RepoDeckFile(t, name).CommanderIndices()
		}
		return d
	}
	formats := []string{"constructed"}
	if commander {
		formats = []string{"commander"}
	}
	var belief [][]*cards.Card
	for _, n := range testutil.RepoDeckNames() {
		if commander {
			if len(testutil.RepoDeckFile(t, n).CommanderIndices()) > 0 {
				belief = append(belief, testutil.RepoDeck(t, reg, n))
			}
			continue
		}
		if strings.HasPrefix(n, "mono-") || slices.Contains([]string{"ur-delver", "uw-control", "dimir-tempo", "death-n-taxes", "boros-moxite-burn", "uw-tempo", "eldrazi-stompy"}, n) {
			belief = append(belief, testutil.RepoDeck(t, reg, n))
		}
	}
	b, err := searchprobe.NewBelief(belief)
	if err != nil {
		t.Fatal(err)
	}
	entry := &bots.Entry{
		Info: bots.Info{Name: "census-paired", Tier: bots.Experimental, Env: true, Search: true, ClosedLists: true,
			Formats: formats, MaxSeats: 2},
		New: azredeal.ClosedFactory(b, sims),
	}
	for g := 0; g < games; g++ {
		o := Options{LoadDeck: func(name string) (Deck, error) {
			if name == "human" {
				return deckOf(decks[0], own), nil
			}
			return deckOf(decks[1], opp), nil
		}, Sleep: func(time.Duration, <-chan struct{}) {}}
		o.PairedBot = entry
		o.Seats = func([]string, uint64) []seat.Seat { return []seat.Seat{passSeat{}, heuristic.NewBot(uint64(g) + 77)} }
		r, err := New(o)
		if err != nil {
			t.Fatal(err)
		}
		cfg := pairedTable(nil)
		cfg.Humans = []int{0}
		if commander {
			cfg.Format = FormatCommander
		}
		cfg.Seed = uint64(20261100 + g)
		cfg.AutoMana, cfg.BotAutoPayMana, cfg.Mulligans = !manual, true, 7
		if err := r.AddTable(cfg); err != nil {
			t.Fatal(err)
		}
		if err := r.Start("t1"); err != nil {
			t.Fatal(err)
		}
		bot := heuristic.NewBot(uint64(g) + 5)
		var after uint64
		for asks := 0; asks < maxAsks; asks++ {
			d := censusParked(r, after)
			if d == nil {
				break
			}

			start := time.Now()
			a, err := r.Advice(context.Background(), "t1", 1, 0, d.Seq)
			ms := float64(time.Since(start).Microseconds()) / 1000
			state, reason := string(a.State), a.Reason
			if errors.Is(err, ErrAdviceStale) {
				state = "advice_stale"
			} else if err != nil {
				state, reason = "error", err.Error()
			}
			fmt.Fprintf(f, "%d\t%s\t%s\t%s\t%.1f\t%s\n", g, d.Kind, censusShape(d), state, ms, strings.ReplaceAll(reason, "\t", " "))
			_, m, _ := r.lookup("t1", 1)
			m.mu.RLock()
			brd := botpolicy.BoardFromGameInto(m.e.G, m.e, 0, new(botpolicy.Board)).Detached()
			m.mu.RUnlock()
			in, err := bot.DecideBoard(context.Background(), brd, *d)
			if err != nil {
				t.Fatal(err)
			}
			if err := r.SubmitIntent("t1", 1, 0, in); err != nil {
				// fall back to the minimal legal answer
				if err2 := r.SubmitIntent("t1", 1, 0, minimalAnswer(d)); err2 != nil {
					fmt.Fprintf(f, "%d\t%s\t%s\tdriver_stuck\t0\t%v\n", g, d.Kind, censusShape(d), err2)
					break
				}
			}
			after = d.Seq
		}
		r.Close()
	}
}

// censusParked is the human seat's next decision after seq, nil when the match is over.
func censusParked(r *Registry, after uint64) *decision.Decision {
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if d, err := r.Pending("t1", 1, 0); err == nil && (d.Seq > after || after == 0) {
			return d
		}
		if _, m, err := r.lookup("t1", 1); err == nil {
			m.mu.RLock()
			st := m.state
			m.mu.RUnlock()
			if st != protocol.MatchLive {
				return nil
			}
		}
		time.Sleep(time.Millisecond)
	}
	return nil
}

// censusShape subdivides a kind into the sub-asks a player meets.
func censusShape(d *decision.Decision) string {
	var tags []string
	if d.ManaPayment != nil {
		tags = append(tags, "mana_payment")
	}
	if len(d.PaymentActions) > 0 {
		tags = append(tags, "payment_actions")
	}
	kinds := map[string]bool{}
	for _, o := range d.Options {
		kinds[o.Kind] = true
	}
	var ks []string
	for k := range kinds {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	if len(ks) > 3 {
		ks = ks[:3]
	}
	if d.Kind != decision.KPriority {
		tags = append(tags, "opts="+strings.Join(ks, "+"))
	}
	if d.Kind == decision.KTarget && d.Max > 1 {
		tags = append(tags, "multi")
	}
	if len(tags) == 0 {
		return "-"
	}
	return strings.Join(tags, ",")
}
