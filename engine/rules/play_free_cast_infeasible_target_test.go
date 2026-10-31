package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// TestFreeCastWithAnUnsatisfiableTargetIsNotOffered pins CR 601.2c: a spell
// that needs a target no object or player can supply cannot be cast, free or
// not. Felling Blow ("target creature you control ... target creature an
// opponent controls") with no creature on the opponent's side cannot be cast
// by Epic Experiment's Play any more than from hand; the Play must not begin
// it, ask its first target and drop the second (dirac K1m pair 23256,
// Etali, Primal Conqueror).
func TestFreeCastWithAnUnsatisfiableTargetIsNotOffered(t *testing.T) {
	t.Parallel()
	run := runInlineOracle(t, `{
	  "name": "epic-experiment-felling-blow-no-opposing-creature",
	  "setup": {"p0": {"hand": ["Epic Experiment"], "battlefield": ["Grizzly Bears"], "library_top": ["Felling Blow", "Forest"]}},
	  "steps": [
	    {"op": "cast", "seat": 0, "card": "p0:Epic Experiment", "mana": "CCCUR",
	     "answers": [{"kind": "choose", "pick": ["X = 3"]}]},
	    {"op": "resolve"}
	  ],
	  "expect": [{"card": "p0:Felling Blow", "zone": "graveyard"}]
	}`)
	blow := run.refs["p0:Felling Blow"]
	for _, ev := range run.e.L.Events {
		if ev.Kind == events.PutOnStack && ev.Obj == blow {
			t.Fatalf("Felling Blow was cast with no legal second target (event %d)", ev.Seq)
		}
	}
}

// TestFreeCastWithASatisfiableTargetStillOffered is the control: the same
// Felling Blow with a creature on the opponent's side is offered and cast.
func TestFreeCastWithASatisfiableTargetStillOffered(t *testing.T) {
	t.Parallel()
	run := runInlineOracle(t, `{
	  "name": "epic-experiment-felling-blow-with-opposing-creature",
	  "setup": {"p0": {"hand": ["Epic Experiment"], "battlefield": ["Grizzly Bears"], "library_top": ["Felling Blow", "Forest"]},
	            "p1": {"battlefield": ["Hill Giant"]}},
	  "steps": [
	    {"op": "cast", "seat": 0, "card": "p0:Epic Experiment", "mana": "CCCUR",
	     "answers": [{"kind": "choose", "pick": ["X = 3"]}]},
	    {"op": "resolve", "answers": [{"kind": "modes", "pick": ["Play Felling Blow"]},
	                                  {"kind": "target", "pick": ["p0:Grizzly Bears"]},
	                                  {"kind": "target", "pick": ["p1:Hill Giant"]}]}
	  ],
	  "expect": [{"card": "p1:Hill Giant", "zone": "graveyard"}]
	}`)
	blow := run.refs["p0:Felling Blow"]
	for _, ev := range run.e.L.Events {
		if ev.Kind == events.PutOnStack && ev.Obj == blow {
			return
		}
	}
	t.Fatal("Felling Blow was never cast although both targets were available")
}
