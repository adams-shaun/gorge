package rules

// The audit half of the oracle runner's exported surface. RunOracleScenarioJSON
// (oracle_export.go) is the compliance pipeline's entry point; the functions
// here are what the scenario-corpus drivers need on top of it: the hand
// audit's own run mode, the log-only replay check every audit scenario gets,
// and a corpus-free shape check. The corpora themselves and their drivers
// live in gorge-compliance (github.com/adams-shaun/gorge-compliance); gorge
// keeps the runner and a handful of scenarios as its contract check
// (oracle_contract_test.go).

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// OracleAuditResult is one audit scenario's outcome.
type OracleAuditResult struct {
	// Fails are the runner's mismatches against the scenario's own
	// expectations (nil = pass). A leftover step answer carries
	// OracleUnconsumedMarker.
	Fails      []string
	Transcript []string
	// ReplayDiff is every difference between the finished game and the game
	// its event log alone rebuilds from genesis ("" = identical). The setup
	// and the stand-in ops are logged events, so a non-empty diff is an
	// engine replay bug or a runner write outside emit -- never a card
	// verdict. Empty when setup failed before genesis.
	ReplayDiff string
}

// OracleUnconsumedMarker tags the fail a step raises when its declared
// `answers` queue still holds entries after the step finished, so a driver
// can excuse exactly those fails through its known-unconsumed ratchet.
const OracleUnconsumedMarker = oracleUnconsumedMarker

// RunOracleAuditScenarioJSON plays one scenario object (an element of an
// oracle file's "scenarios" array; unknown fields are rejected) and replays
// its log. fixture=false is the hand Oracle-text audit's mode: setup
// placements fire their zone-change triggers as in a real game.
// fixture=true is RunOracleScenarioJSON's XMage-fixture mode (setup
// permanents are present before turn 1 with no triggers), which the
// XMage-ported scenarios use.
func RunOracleAuditScenarioJSON(reg *cards.Registry, raw []byte, fixture bool) (OracleAuditResult, error) {
	sc, err := decodeOracleScenario(raw)
	if err != nil {
		return OracleAuditResult{}, err
	}
	sc.xmageFixture = fixture
	fails, transcript, run := runOracleScenario(reg, sc)
	res := OracleAuditResult{Fails: fails, Transcript: transcript}
	if run != nil && run.e != nil {
		res.ReplayDiff = gameStateDiff(run.e.G, logReplayGame(run.cfg, run.e.L.Events))
	}
	return res, nil
}

// CheckOracleScenarioJSON reports what is wrong with one scenario object's
// shape without a corpus: the strict schema, the closed step vocabulary and
// the card/seat ref syntax. nil = well formed. Authoring policy (a scenario
// states why/cr, asserts something) is the corpus owner's to check.
func CheckOracleScenarioJSON(raw []byte) []string {
	sc, err := decodeOracleScenario(raw)
	if err != nil {
		return []string{err.Error()}
	}
	var out []string
	checkRef := func(where, ref string) {
		if ref == "" {
			return
		}
		if _, ok := parseSeatRef(ref); ok {
			return
		}
		if _, _, _, _, _, err := splitRef(ref); err != nil {
			out = append(out, fmt.Sprintf("%s: %v", where, err))
		}
	}
	exps := append([]oracleExpect(nil), sc.Expect...)
	for i, st := range sc.Steps {
		where := fmt.Sprintf("step %d", i)
		if !oracleAuditOps[st.Op] {
			out = append(out, fmt.Sprintf("%s: unknown op %q", where, st.Op))
		}
		for _, ref := range append(append([]string{st.Card, st.Defender}, st.Targets...), st.Attackers...) {
			checkRef(where, ref)
		}
		exps = append(exps, st.Expect...)
	}
	for _, x := range exps {
		checkRef("expect", x.Card)
		checkRef("expect", x.TriggerOnStack)
	}
	return out
}

// oracleAuditOps is the closed step vocabulary an authored scenario may use.
var oracleAuditOps = map[string]bool{
	"mana": true, "cast": true, "activate": true, "play": true, "resolve": true,
	"attack": true, "block": true, "pass": true, "pass_to": true, "move": true, "life": true,
}

// logReplayGame reconstructs a Game from cfg's decks (replicating New's own
// unlogged genesis AddObject calls, in the same per-deck order) plus every
// event in log, applied through events.Apply directly -- no Engine involved,
// exactly what a replay from the log alone does. Tokens, starting life,
// commander identity/cast counters and the commander damage-vector size are
// genesis bookkeeping rather than logged state, so they are rebuilt from
// Config and the event stream as New initializes them.
func logReplayGame(cfg Config, log []events.Event) *state.Game {
	g := state.NewGameLife(cfg.Names, startingLife(cfg))
	g.Tokens = cfg.Tokens
	cmdIDs := make([][]state.ObjID, len(cfg.Names))
	totalCmd := 0
	for i := range cfg.Names {
		if i >= len(cfg.Decks) {
			continue
		}
		deck := cfg.Decks[i]
		p := state.PlayerID(i)
		ids := make([]state.ObjID, 0, len(deck))
		for _, c := range deck {
			ids = append(ids, g.AddObject(c, p).ID)
		}
		for _, idx := range cfg.legalCommandersFor(i, len(deck), deck) {
			cmdIDs[i] = append(cmdIDs[i], ids[idx])
		}
		totalCmd += len(cmdIDs[i])
		g.SetZone(state.ZLibrary, p, ids)
		if i < len(cfg.Sideboards) {
			sb := make([]state.ObjID, 0, len(cfg.Sideboards[i]))
			for _, c := range cfg.Sideboards[i] {
				o := g.AddObject(c, p)
				o.Zone = state.ZSideboard
				sb = append(sb, o.ID)
			}
			g.SetZone(state.ZSideboard, p, sb)
		}
		if i < len(cfg.PlanarDecks) {
			for _, c := range cfg.PlanarDecks[i] {
				g.AddObject(c, p)
			}
		}
	}
	// events.Apply's CmdDamage case resolves a dense index from Commanders
	// and folds into CmdDamage, so both must exist before applying the log.
	for p := range g.Players {
		if len(cmdIDs[p]) > 0 {
			g.Players[p].Commanders = append([]state.ObjID(nil), cmdIDs[p]...)
		}
		if totalCmd > 0 {
			g.Players[p].CmdDamage = make([]int32, totalCmd)
		}
	}
	for _, ev := range log {
		events.Apply(g, ev)
	}
	for p := range g.Players {
		if len(cmdIDs[p]) > 0 {
			g.Players[p].CmdCasts = make([]int32, len(cmdIDs[p]))
			// The same genesis seed New applies before the opening deal: a
			// scenario's restated cast history (CR 903.8) is not in the log,
			// so the replay reads it from Config like the commander list.
			if p < len(cfg.CommanderCasts) {
				for k := range g.Players[p].CmdCasts {
					if k < len(cfg.CommanderCasts[p]) {
						g.Players[p].CmdCasts[k] += cfg.CommanderCasts[p][k]
					}
				}
			}
			for _, ev := range log {
				if ev.Kind != events.PutOnStack || ev.Player != state.PlayerID(p) || ev.From != state.ZCommand {
					continue
				}
				for k, id := range cmdIDs[p] {
					if id == ev.Obj {
						g.Players[p].CmdCasts[k]++
						break
					}
				}
			}
		}
	}
	return g
}

// gameStateDiff reports every structural difference between two games, or ""
// if they match: Players, Objs, Stack, each zone, and the turn-structure
// scalars (Winner, Draw and NextID included).
func gameStateDiff(a, b *state.Game) string {
	var diffs []string
	// Players compare by value with nil and empty slices alike (the
	// formatted text): a spent RestrictedMana batch list is an empty slice
	// after events.Apply and nil after a Clone, and the resolution kernel's
	// restore is a clone.
	if ap, bp := fmt.Sprintf("%+v", a.Players), fmt.Sprintf("%+v", b.Players); ap != bp {
		diffs = append(diffs, fmt.Sprintf("players: %s vs %s", ap, bp))
	}
	n := len(a.Objs)
	if len(b.Objs) > n {
		n = len(b.Objs)
	}
	for i := 0; i < n; i++ {
		var ao, bo any
		if i < len(a.Objs) {
			ao = a.Objs[i]
		}
		if i < len(b.Objs) {
			bo = b.Objs[i]
		}
		if !reflect.DeepEqual(ao, bo) {
			diffs = append(diffs, fmt.Sprintf("obj[%d]: %+v vs %+v", i+1, ao, bo))
		}
	}
	// Stack and per-zone ID lists use slices.Equal, not reflect.DeepEqual.
	// events.remove (engine/events/apply.go) always allocates via make, even for a
	// zero-length result, so a zone that just lost its last object holds a
	// non-nil empty slice; state.Game.Clone's append-based copy normalises
	// that same empty zone to nil. slices.Equal compares length and elements
	// only, so it correctly treats nil and empty as the same zone; DeepEqual
	// does not.
	if !slices.Equal(a.Stack, b.Stack) {
		diffs = append(diffs, fmt.Sprintf("stack: %v vs %v", a.Stack, b.Stack))
	}
	for p := state.PlayerID(0); int(p) < len(a.Players); p++ {
		for _, z := range []state.Zone{state.ZLibrary, state.ZHand, state.ZBattlefield, state.ZGraveyard, state.ZExile} {
			az, bz := a.Zone(z, p), b.Zone(z, p)
			if !slices.Equal(az, bz) {
				diffs = append(diffs, fmt.Sprintf("p%d:%s: %v vs %v", p, z, az, bz))
			}
		}
	}
	// Winner, Draw and NextID are compared
	// too. They were missing, so "a whole-game diff of empty" claimed more
	// than this helper delivered -- most sharply for Draw, which GameOver
	// added precisely because Winner's zero value is a real seat, so a game
	// that ended in a draw compared equal to one seat 0 won. Replay
	// inherits this harness; a comparison that cannot
	// see who won is a bad foundation to hand it.
	if a.Turn != b.Turn || a.Active != b.Active || a.Clock != b.Clock ||
		a.Priority != b.Priority || a.Passes != b.Passes || a.Step != b.Step ||
		a.Over != b.Over || a.Winner != b.Winner || a.Draw != b.Draw || a.NextID != b.NextID {
		diffs = append(diffs, fmt.Sprintf(
			"scalars: turn=%d/%d active=%d/%d clock=%d/%d priority=%d/%d passes=%d/%d step=%s/%s over=%v/%v winner=%d/%d draw=%v/%v nextid=%d/%d",
			a.Turn, b.Turn, a.Active, b.Active, a.Clock, b.Clock,
			a.Priority, b.Priority, a.Passes, b.Passes, a.Step, b.Step, a.Over, b.Over,
			a.Winner, b.Winner, a.Draw, b.Draw, a.NextID, b.NextID))
	}
	return strings.Join(diffs, "\n")
}
