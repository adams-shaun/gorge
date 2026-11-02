package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Zurgo, Thunder's Decree: "During your end step, Warrior tokens you control
// have 'This token can't be sacrificed.'" The grant is an AddStaticAbility$
// CantSacrifice (CR 701.21a: a permanent that can't be sacrificed isn't), so
// mobilize's own "sacrifice them at the beginning of the next end step" (CR
// 702.181a) leaves the Warrior tokens on the battlefield. The grant is gated
// on the end step (Phases$ End of Turn): in the second main phase the same
// tokens can be sacrificed.
const zurgoEndStepScenario = `{
  "name": "zurgo-end-step",
  "setup": {"p0": {"battlefield": ["Zurgo, Thunder's Decree"]}, "p1": {"battlefield": ["Grizzly Bears"]}},
  "steps": [
    {"op": "attack", "seat": 0, "attackers": ["p0:Zurgo, Thunder's Decree"], "defender": "p1"},
    {"op": "pass_to", "seat": 0, "step": "main2"},
    {"op": "pass_to", "seat": 0, "step": "end"},
    {"op": "resolve", "seat": 0}
  ]
}`

func warriorTokens(e *Engine) []state.ObjID {
	var out []state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.IsToken && strings.Contains(o.Face().Name, "Warrior") {
			out = append(out, id)
		}
	}
	return out
}

func TestGrantedCantSacrificeZurgoEndStep(t *testing.T) {
	sc, err := decodeOracleScenario([]byte(zurgoEndStepScenario))
	if err != nil {
		t.Fatal(err)
	}
	sc.xmageFixture = true
	sc.Steps = sc.Steps[:2]
	_, _, run := runOracleScenario(testutil.CorpusRegistry(t), sc)
	toks := warriorTokens(run.e)
	if len(toks) != 2 {
		t.Fatalf("precondition: %d Warrior tokens after mobilize, want 2", len(toks))
	}
	if run.e.SacrificeBlocked(toks[0], false) {
		t.Fatalf("main2: Warrior token can't be sacrificed outside the end step")
	}

	sc, _ = decodeOracleScenario([]byte(zurgoEndStepScenario))
	sc.xmageFixture = true
	fails, transcript, run := runOracleScenario(testutil.CorpusRegistry(t), sc)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(transcript, "\n"))
	}
	if run.e.G.Step != state.StepEnd {
		t.Fatalf("stopped at %v, want the end step", run.e.G.Step)
	}
	if got := len(warriorTokens(run.e)); got != 2 {
		t.Fatalf("end step: %d Warrior tokens survived mobilize's sacrifice, want 2", got)
	}
}
