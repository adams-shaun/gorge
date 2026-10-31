package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestUntapOrderAnswerGivesActivePlayerUpkeepPriority: answering the CR 616.1
// order ask over one Untap finishes the untap step and enters upkeep, where the
// active player receives priority (CR 117.3a). Two Sleep Magic on one creature
// pose the ask to the creature's controller on their own untap step.
func TestUntapOrderAnswerGivesActivePlayerUpkeepPriority(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc, err := decodeOracleScenario([]byte(`{"name":"sleep-magic-x2-upkeep-priority","cr":["117.3a"],"why":"priority after the order ask",
"setup":{"p0":{"hand":["Sleep Magic","Sleep Magic"],"mana":"UU"},"p1":{"battlefield":["Hill Giant"]}},
"steps":[{"op":"cast","seat":0,"card":"p0:Sleep Magic","mana":"U","targets":["p1:Hill Giant"]},{"op":"resolve"},{"op":"resolve"},
{"op":"cast","seat":0,"card":"p0:Sleep Magic#1","mana":"U","targets":["p1:Hill Giant"]},{"op":"resolve"},{"op":"resolve"},
{"op":"pass_to","step":"upkeep","active":"p1","answers":[{"kind":"any","pick":["Sleep Magic"]}]}],
"expect":[{"card":"p1:Hill Giant","tapped":true}]}`))
	if err != nil {
		t.Fatal(err)
	}
	fails, transcript, run := runOracleScenario(reg, sc)
	if len(fails) != 0 {
		t.Fatalf("fails: %v\n%s", fails, strings.Join(transcript, "\n"))
	}
	if run.e.G.Step != state.StepUpkeep || run.e.G.Active != 1 {
		t.Fatalf("step %v active %d, want p1's upkeep", run.e.G.Step, run.e.G.Active)
	}
	if got := run.e.G.Priority; got != 1 {
		t.Fatalf("priority with p%d, want the active player p1 (CR 117.3a)", got)
	}
	if d := run.e.Pending(); d == nil || d.Player != 1 {
		t.Fatalf("pending %+v, want p1's priority decision", d)
	}
}
