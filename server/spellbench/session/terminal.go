package session

import (
	"errors"
	"fmt"
	"os"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/gamecfg"
	"github.com/adams-shaun/gorge/server/spellbench/identity"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

var causes = []struct {
	err  error
	name string
}{
	{mapping.ErrUnmapped, "unmapped_decision"}, {mapping.ErrDeadEnd, "dead_end"},
	{mapping.ErrCandidateLimit, "candidate_limit"}, {mapping.ErrDuplicate, "duplicate_candidates"},
	{mapping.ErrUnresolvableSource, "unresolvable_source"}, {gamecfg.ErrUnplannedRandomness, "unplanned_randomness"},
	{identity.ErrIDCollision, "id_collision"}, {identity.ErrShadowDiverged, "identity_shadow_diverged"},
	{errFollowup, "followup_mismatch"},
}

func cause(err error, fallback string) string {
	fmt.Fprintf(os.Stderr, "gorge adapter: %v\n", err) // detail stays on stderr, never in reason
	for _, c := range causes {
		if errors.Is(err, c.err) {
			return c.name
		}
	}
	return fallback
}

func (s *Game) terminal(outcome, class, reason string, winner *string) {
	s.term = &protocol.TerminalResponse{ResponseType: "terminal", Protocol: protocol.Name, GameID: s.ID,
		Outcome: outcome, Classification: class, Winner: winner, Reason: reason,
		StepCount: s.step, DecisionCount: s.decisions, Provenance: s.cfg.Provenance}
	if s.cfg.AutoPay {
		s.term.XGorge = &protocol.TerminalDiagnostics{AutoPayFallbacks: s.autoPayFallbacks}
	}
	s.tx, s.resp = nil, nil
}

func (s *Game) halt(c string) { s.terminal("halted", "halted", "engine_contract_failure:"+c, nil) }

func (s *Game) truncate(cap string) { s.terminal("truncated", "truncated", cap, nil) }

func (s *Game) natural() {
	g := s.g.R.Board()
	if g.Draw {
		s.terminal("draw", "natural", "draw", nil)
		return
	}
	w := observe.Seat(g.Winner)
	s.terminal(w+"_win", "natural", lossReason(s.g.R.Log().Events, 1-g.Winner), &w)
}

func lossReason(evs []events.Event, loser state.PlayerID) string {
	seat := observe.Seat(loser)
	for i := len(evs) - 1; i >= 0; i-- {
		if ev := evs[i]; ev.Kind == events.PlayerLost && ev.Player == loser {
			switch ev.Text {
			case "life total is 0 or less":
				return seat + "_life_zero"
			case "drew from an empty library":
				return seat + "_decked"
			case "ten or more poison counters":
				return seat + "_poison"
			}
			break
		}
	}
	return seat + "_lost"
}
