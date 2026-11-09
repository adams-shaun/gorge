package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCommanderMulligansAskEverySeat: the Config gameConfig builds for
// -format commander -mulligans 1 carries the round, and genesis poses the
// keep/mulligan ask to every seat before turn 1. Answering every ask with
// option 0 (keep) has no bottoming penalty, so the only pregame decisions
// are the per-seat keep asks.
func TestCommanderMulligansAskEverySeat(t *testing.T) {
	dir := testutil.CorpusDirOrSkip(t)
	reg, err := testutil.OpenCorpusRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	decks := make([][]*cards.Card, len(commanderPair))
	cmds := make([][]int, len(commanderPair))
	for i, name := range commanderPair {
		if decks[i], err = testutil.LoadRepoDeck(reg, name); err != nil {
			t.Fatal(err)
		}
		if cmds[i] = testutil.RepoDeckFile(t, name).CommanderIndices(); len(cmds[i]) == 0 {
			t.Fatalf("deck %s: no commander indices", name)
		}
	}
	cfg := gameConfig(0, commanderPair, decks, reg, rules.FormatCommander, cmds, 1)
	if cfg.Mulligans != 1 {
		t.Fatalf("precondition: Config.Mulligans = %d, want 1", cfg.Mulligans)
	}
	e := rules.New(cfg)
	e.Advance()
	asked := make(map[state.PlayerID]bool)
	for e.G.Turn == 0 && e.Pending() != nil && !e.G.Over {
		d := e.Pending()
		if d.Kind != decision.KMulligan {
			t.Fatalf("pregame decision kind = %s, want %s", d.Kind, decision.KMulligan)
		}
		asked[d.Player] = true
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
			t.Fatalf("seat %d keep: %v", d.Player, err)
		}
	}
	for i := range commanderPair {
		if !asked[state.PlayerID(i)] {
			t.Errorf("seat %d was never asked to keep or mulligan", i)
		}
	}
}

// TestCommanderRunMulligansReachTheSeats drives the simOptions path the
// -mulligans flag feeds: a Commander run with Mulligans 1 records keep/
// mulligan decisions in its shape census, while the default (0) records none.
func TestCommanderRunMulligansReachTheSeats(t *testing.T) {
	dir := testutil.CorpusDirOrSkip(t)
	for _, tc := range []struct {
		name      string
		mulligans int
		want      int
	}{
		{name: "one mulligan", mulligans: 1, want: 2},
		{name: "default none", mulligans: 0, want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			shapePath := filepath.Join(t.TempDir(), "shape.json")
			var buf bytes.Buffer
			err := runSim(simOptions{
				Decks: strings.Join(commanderPair, ","), Seats: 2, Games: 1,
				Dir: dir, Format: "commander", Mulligans: tc.mulligans, Shape: shapePath,
			}, &buf)
			if err != nil {
				t.Fatalf("runSim: %v\n%s", err, buf.String())
			}
			raw, err := os.ReadFile(shapePath)
			if err != nil {
				t.Fatal(err)
			}
			var got int
			for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
				var row struct {
					Row     string `json:"row"`
					Options map[string]struct {
						N int `json:"n"`
					} `json:"options"`
				}
				if err := json.Unmarshal([]byte(line), &row); err != nil {
					t.Fatalf("decoding shape row: %v", err)
				}
				if row.Row == "aggregate" {
					got = row.Options["mulligan"].N
				}
			}
			if got != tc.want {
				t.Errorf("shape census recorded %d mulligan decisions, want %d\n%s", got, tc.want, buf.String())
			}
		})
	}
}
