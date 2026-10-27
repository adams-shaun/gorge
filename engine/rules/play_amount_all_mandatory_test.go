package rules

// cr-g246: a "DB$ Play | Amount$ All" WITHOUT "Optional$" is "cast the
// copies if able" -- every castable candidate must be cast, the player only
// orders them. Only with "Optional$ True" is the ask a declinable "choose any
// number" (Min 0).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// playAskDecisions returns the run's answered Play asks (effPlay's KModes
// "play" decision), in the order the scenario answered them.
func playAskDecisions(run *oracleRun) []OracleDecision {
	var out []OracleDecision
	for _, od := range run.decisions {
		if od.GorgeKind == "modes" && od.Resume == "play" {
			out = append(out, od)
		}
	}
	return out
}

// TestPlayAmountAllWithoutOptionalCastsEveryCopy pins Spelltwine's "cast the
// copies if able": its body is DB$ Play | Defined$ Remembered | Amount$ All |
// CopyCard$ True | WithoutManaCost$ True with no Optional$, so the play ask
// is Min == Max == 2 over the two exiled cards and the answer only orders
// them. Both copies are cast (each aimed at p1: Bolt 3 + Shock 2), so p1 is
// at 15, while Spelltwine and the two originals stay in exile.
func TestPlayAmountAllWithoutOptionalCastsEveryCopy(t *testing.T) {
	t.Parallel()
	run := runInlineOracle(t, `{
	  "name": "spelltwine-casts-every-copy",
	  "setup": {"p0": {"hand": ["Spelltwine"], "graveyard": ["Lightning Bolt"]},
	            "p1": {"graveyard": ["Shock"]}},
	  "steps": [
	    {"op": "cast", "seat": 0, "card": "p0:Spelltwine", "mana": "CCCCCU",
	     "targets": ["p0:Lightning Bolt", "p1:Shock"]},
	    {"op": "resolve", "answers": [{"kind": "modes", "pick": ["Play Lightning Bolt", "Play Shock"]},
	                                  {"kind": "target", "pick": ["p1"]},
	                                  {"kind": "target", "pick": ["p1"]}]}
	  ],
	  "expect": [{"life": {"p1": 15}},
	             {"card": "p0:Spelltwine", "zone": "exile"},
	             {"card": "p0:Lightning Bolt", "zone": "exile"},
	             {"card": "p1:Shock", "zone": "exile"}]
	}`)
	bolt, shock := run.refs["p0:Lightning Bolt"], run.refs["p1:Shock"]
	spelltwine := run.refs["p0:Spelltwine"]
	if bolt == 0 || shock == 0 || spelltwine == 0 {
		t.Fatalf("setup precondition: refs spelltwine=%d bolt=%d shock=%d, want all bound", spelltwine, bolt, shock)
	}
	asks := playAskDecisions(run)
	if len(asks) != 1 {
		t.Fatalf("answered Play asks = %+v, want exactly the one Spelltwine cast", asks)
	}
	if ask := asks[0]; ask.Min != 2 || ask.Max != 2 || ask.Options != 2 {
		t.Fatalf("Spelltwine Play ask = Min %d Max %d over %d options, want Min 2 Max 2 over 2 (cast both if able)",
			ask.Min, ask.Max, ask.Options)
	}
	// Both copies were minted and cast, in the answer's order: Bolt then
	// Shock (the answer order is the cast order).
	var copied []state.ObjID
	for _, ev := range run.e.L.Events {
		if ev.Kind == events.StackCopy && (ev.Obj == bolt || ev.Obj == shock) {
			copied = append(copied, ev.Obj)
		}
	}
	if len(copied) != 2 || copied[0] != bolt || copied[1] != shock {
		t.Fatalf("copied-for-play cards = %v, want Bolt %d then Shock %d", copied, bolt, shock)
	}
	replayCheck(t, run.e, run.cfg)
}

// TestPlayAmountAllOptionalStillDeclinable pins the other side of cr-g246:
// an Optional$ True Amount$ All carrier (Epic Experiment's DB$ Play) stays
// declinable -- its Play ask is Min 0, Max 2 over the two exiled spells.
func TestPlayAmountAllOptionalStillDeclinable(t *testing.T) {
	t.Parallel()
	run := runInlineOracle(t, `{
	  "name": "epic-experiment-optional-play-stays-declinable",
	  "setup": {"p0": {"hand": ["Epic Experiment"], "library_top": ["Lightning Bolt", "Shock", "Forest"]}},
	  "steps": [
	    {"op": "cast", "seat": 0, "card": "p0:Epic Experiment", "mana": "CCCUR",
	     "answers": [{"kind": "choose", "pick": ["X = 3"]}]},
	    {"op": "resolve", "answers": [{"kind": "modes", "pick": ["Play Lightning Bolt", "Play Shock"]},
	                                  {"kind": "target", "pick": ["p1"]},
	                                  {"kind": "target", "pick": ["p1"]}]}
	  ],
	  "expect": [{"life": {"p1": 15}}]
	}`)
	asks := playAskDecisions(run)
	if len(asks) != 1 {
		t.Fatalf("answered Play asks = %+v, want exactly the one Epic Experiment cast", asks)
	}
	if ask := asks[0]; ask.Min != 0 || ask.Max != 2 || ask.Options != 2 {
		t.Fatalf("Epic Experiment Play ask = Min %d Max %d over %d options, want Min 0 Max 2 over 2 (Optional$ stays declinable)",
			ask.Min, ask.Max, ask.Options)
	}
	replayCheck(t, run.e, run.cfg)
}
