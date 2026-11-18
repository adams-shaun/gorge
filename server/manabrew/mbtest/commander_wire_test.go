//go:build manabrew

package mbtest

// The 4-seat Commander cell of the ManaBrew wire matrix. Constructed games
// cross the wire in the census and parity tests; these two add the one format
// whose extra state (the command zone, commander damage, the commander tax)
// has its own prompt (KCommanderZone) and its own wire fields
// (PlayerDto.CommanderCasts / CommanderDamage, a "command" ZoneDto).
//
//   - TestParityCommanderFourSeat: the production bot plays the Foundations
//     precon pod natively and again through the wire, at several seeds; the
//     chain heads, outcomes and intent streams must match.
//   - TestCensusCommanderFourSeat: the same pod over the wire must leave no
//     unmapped prompt and no refusal, and the census plus the wire state must
//     show a KCommanderZone ask, a commander in the "command" zone, a
//     non-zero commander tax and non-zero commander damage.

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// commanderSeeds are the pod seeds: rules.CommanderAcceptanceSeed first (the
// game TestHeads/cmd4 pins), then neighbours so one stalled seed cannot be
// the only evidence. Each plays under the census watchdogs.
var commanderSeeds = []uint64{rules.CommanderAcceptanceSeed, 43, 44}

// commanderWireObs tallies what the ManaBrew state message showed the seats.
type commanderWireObs struct {
	states        int
	commandZone   int // states with a card in a "command" ZoneDto
	taxCasts      int // states with a PlayerDto.CommanderCasts entry > 0
	damage        int // states with a PlayerDto.CommanderDamage entry > 0
	maxCasts      int
	maxCmdDamage  int
	zoneAsks      int
	commanderSeen map[string]bool
}

// observe folds one wire state into the tallies.
func (o *commanderWireObs) observe(m mb.EngineMessage) error {
	st, ok := m.Value.(mb.StateUpdate)
	if !ok {
		return fmt.Errorf("State returned %T, want mb.StateUpdate", m.Value)
	}
	o.states++
	for _, z := range st.GameView.Zones {
		if z.Zone == mb.ZoneCommand && len(z.Cards) > 0 {
			o.commandZone++
			break
		}
	}
	taxed, hurt := false, false
	for _, p := range st.GameView.Players {
		for id, n := range p.CommanderCasts {
			o.commanderSeen[id] = true
			if n > 0 {
				taxed = true
				o.maxCasts = max(o.maxCasts, n)
			}
		}
		for _, n := range p.CommanderDamage {
			if n > 0 {
				hurt = true
				o.maxCmdDamage = max(o.maxCmdDamage, n)
			}
		}
	}
	if taxed {
		o.taxCasts++
	}
	if hurt {
		o.damage++
	}
	return nil
}

// observedWireSeat is a BotWireSeat that also reads the wire state for every
// decision it answers, so commander fields are checked as a client sees them.
type observedWireSeat struct {
	*BotWireSeat
	obs *commanderWireObs
	err *error
}

func (s *observedWireSeat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	if d.Kind == decision.KCommanderZone {
		s.obs.zoneAsks++
	}
	if err := s.obs.observe(s.Translator.State(v)); err != nil && *s.err == nil {
		*s.err = err
	}
	return s.BotWireSeat.Decide(ctx, v, d)
}

// commanderPodConfig is the 4-seat Foundations Commander pod at the given seed.
func commanderPodConfig(t *testing.T, seed uint64) rules.Config {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	names, decks, commanders, err := testutil.CommanderAcceptanceDecks(reg)
	if err != nil {
		t.Fatalf("commander pod: %v", err)
	}
	cfg := rules.CommanderAcceptanceConfig(reg, names, decks, commanders)
	cfg.Seed = seed
	if cfg.Format != rules.FormatCommander || len(cfg.Names) != 4 {
		t.Fatalf("commander pod: format %v, %d seats; want a 4-seat Commander game", cfg.Format, len(cfg.Names))
	}
	return cfg
}

// TestParityCommanderFourSeat is the Commander row of the bot-parity matrix:
// native heuristic bot vs the same bot through the ManaBrew wire, same chain
// head, outcome and intent stream.
func TestParityCommanderFourSeat(t *testing.T) {
	for _, seed := range commanderSeeds {
		t.Run(fmt.Sprintf("seed%d", seed), func(t *testing.T) {
			cfg := commanderPodConfig(t, seed)
			oN, eN, rN := playBotParityGame(t, cfg, func(int) seat.Seat { return heuristic.NewBot(seed) })

			census := NewCensus()
			oW, eW, rW := playBotParityGame(t, cfg, func(int) seat.Seat { return NewBotWireSeat("parity-cmd", int64(seed), seed, census) })
			if n := census.TotalUnmapped(); n > 0 {
				t.Fatalf("%d unmapped prompt(s): %v", n, census.Unmapped)
			}
			if n := census.TotalRejected(); n > 0 {
				t.Fatalf("the bot's answers were refused %d time(s): %v", n, census.Rejected)
			}
			if len(rN) == 0 || len(rW) == 0 {
				t.Fatalf("recorded %d native / %d wire intents; a game with no answers proves nothing", len(rN), len(rW))
			}
			if hN, hW := eN.L.Head(), eW.L.Head(); hN == "" || hN != hW {
				t.Fatalf("chain heads differ: native %q, wire %q\n%s", hN, hW, firstDivergence(cfg, intents(rN), intents(rW)))
			}
			if oN.Draw != oW.Draw || oN.WinnerSeat != oW.WinnerSeat {
				t.Fatalf("outcomes differ: native draw=%v winner=%d, wire draw=%v winner=%d", oN.Draw, oN.WinnerSeat, oW.Draw, oW.WinnerSeat)
			}
			if len(rN) != len(rW) {
				t.Fatalf("intent streams differ in length: native %d, wire %d", len(rN), len(rW))
			}
			t.Logf("seed %d: %d intents, head %s, stall %q", seed, len(rN), eN.L.Head(), oN.StallOn)
		})
	}
}

// TestCensusCommanderFourSeat plays the Commander pod over the wire and
// asserts the census and the wire state carry the Commander surface.
func TestCensusCommanderFourSeat(t *testing.T) {
	census := NewCensus()
	obs := &commanderWireObs{commanderSeen: map[string]bool{}}
	var obsErr error
	for _, seed := range commanderSeeds {
		cfg := commanderPodConfig(t, seed)
		playBotParityGame(t, cfg, func(int) seat.Seat {
			return &observedWireSeat{BotWireSeat: NewBotWireSeat("census-cmd", int64(seed), seed, census), obs: obs, err: &obsErr}
		})
		census.AddGame()
	}
	if obsErr != nil {
		t.Fatal(obsErr)
	}
	var enum []string
	for k, n := range census.Enumerated {
		enum = append(enum, fmt.Sprintf("%s=%d", k, n))
	}
	sort.Strings(enum)
	t.Logf("census: %s", census.Summary())
	t.Logf("enumerated: %v", enum)
	t.Logf("wire: %d states, command zone in %d, tax in %d (max %d casts), commander damage in %d (max %d), %d commander-zone asks, %d commanders",
		obs.states, obs.commandZone, obs.taxCasts, obs.maxCasts, obs.damage, obs.maxCmdDamage, obs.zoneAsks, len(obs.commanderSeen))

	if n := census.TotalUnmapped(); n > 0 {
		t.Fatalf("%d unmapped prompt(s): %v", n, census.Unmapped)
	}
	if n := census.TotalRejected(); n > 0 {
		t.Fatalf("%d rejected answer(s): %v", n, census.Rejected)
	}
	if got := len(obs.commanderSeen); got != 4 {
		t.Errorf("wire CommanderCasts named %d commanders, want one per seat (4)", got)
	}
	if obs.commandZone == 0 {
		t.Error("no wire state carried a card in the command zone")
	}
	if obs.zoneAsks == 0 || census.Posed["chooseFromSelection"]+census.Posed["confirm"] == 0 {
		t.Errorf("no KCommanderZone ask reached the wire (asks %d)", obs.zoneAsks)
	}
	if obs.taxCasts == 0 {
		t.Error("no wire state showed a commander cast (the tax) -- commanderCasts stayed 0")
	}
	if obs.damage == 0 {
		t.Error("no wire state showed commander damage")
	}
}
