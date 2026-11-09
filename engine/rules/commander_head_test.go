package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// commanderHeadPath is TestHeads/cmd4's pinned chain head. It is not a
// <seats>.txt file because the same seat count already has a constructed
// head (4.txt); the cmd4 name keeps the two games distinct while the file
// still lives beside the others.
var commanderHeadPath = filepath.Join(headsDir, "cmd4.txt")

// pinnedHeadAt reads an arbitrary pinned-head file and trims its newline.
func pinnedHeadAt(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cmd4: reading the pinned head: %v (re-pin with %s)", err, rePinHint)
	}
	return strings.TrimSpace(string(raw))
}

// playCommanderAcceptance plays the 4-seat Commander acceptance game to
// completion: testutil.CommanderAcceptanceDecks's seating, the same
// rules.CommanderAcceptanceConfig TestHeads/cmd4 pins, and one testBot
// (botpolicy.GameBot) answering every seat through PlayAcceptance.
func playCommanderAcceptance(t *testing.T, reg *cards.Registry, step func(e *Engine, n int)) (*Engine, Config, int) {
	t.Helper()
	names, decks, commanders, err := testutil.CommanderAcceptanceDecks(reg)
	if err != nil {
		t.Fatalf("cmd4: %v", err)
	}
	cfg := CommanderAcceptanceConfig(reg, names, decks, commanders)
	e, n, err := PlayAcceptance(cfg, acceptanceTestBot, step)
	if err != nil {
		t.Fatalf("4-seat Commander: %v", err)
	}
	return e, cfg, n
}

// commanderAcceptanceHead is the finished game's chain head, replayed from
// its own recorded (Config, Log) so the pinned game is also checked
// deterministic, the same way playAcceptance checks the seat-count heads.
func commanderAcceptanceHead(t *testing.T, reg *cards.Registry) string {
	t.Helper()
	e, cfg, _ := playCommanderAcceptance(t, reg, nil)
	re, err := replayFor(cfg, e.L)
	if err != nil {
		t.Fatalf("cmd4: replay: %v", err)
	}
	if re.L.Head() != e.L.Head() {
		t.Fatalf("cmd4: chain %s, replay %s", e.L.Head(), re.L.Head())
	}
	return e.L.Head()
}

// TestCommanderHeadGameEnds pins the shape of the game behind TestHeads/cmd4:
// a 4-seat Commander game at the format-default 40 life, every seat's
// commander in the command zone at the start, and a winner at the end -- not
// a stall, which is why its seed was chosen by measurement.
func TestCommanderHeadGameEnds(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("long")
	}
	reg := testutil.CorpusRegistry(t)
	checkedStart := false
	e, _, n := playCommanderAcceptance(t, reg, func(e *Engine, n int) {
		if checkedStart {
			return
		}
		checkedStart = true
		if got := len(e.G.Players); got != 4 {
			t.Fatalf("cmd4: %d seats, want 4", got)
		}
		for p := range e.G.Players {
			pl := &e.G.Players[p]
			if pl.Life != 40 {
				t.Errorf("cmd4: seat %d starts at %d life, want the FormatCommander default 40", p, pl.Life)
			}
			if len(pl.Commanders) == 0 {
				t.Fatalf("cmd4: seat %d names no commander", p)
			}
			for _, id := range pl.Commanders {
				o := e.G.Obj(id)
				if o == nil {
					t.Fatalf("cmd4: seat %d's commander object %d is missing", p, id)
				}
				if o.Zone != state.ZCommand {
					t.Errorf("cmd4: seat %d's commander %s starts in zone %v, want the command zone", p, o.Face().Name, o.Zone)
				}
			}
		}
	})
	if !e.G.Over {
		t.Fatalf("cmd4: game not over after %d intents (turn %d)", n, e.G.Turn)
	}
	if e.G.Draw {
		t.Fatalf("cmd4: game ended in a draw, not a win, after %d intents (turn %d)", n, e.G.Turn)
	}
	if int(e.G.Winner) >= len(e.G.Players) {
		t.Fatalf("cmd4: winner %d is not a seat", e.G.Winner)
	}
	t.Logf("cmd4: %d intents, %d events, %d turns, winner=seat %d, chain=%s",
		n, len(e.L.Events), e.G.Turn, e.G.Winner, e.L.Head())
}
