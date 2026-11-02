package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestUntapOrderAnswerGivesActivePlayerUpkeepPriority: answering the CR 616.1
// order ask over one Untap finishes the untap step and enters upkeep, where the
// active player receives priority (CR 117.3a). Two Freyalise's Winds pose the
// ask to the creature's controller on their own untap step.
func TestUntapOrderAnswerGivesActivePlayerUpkeepPriority(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc, err := decodeOracleScenario([]byte(`{"name":"freyalises-winds-x2-upkeep-priority","cr":["117.3a","616.1"],"why":"priority after the order ask",
"setup":{"p0":{"battlefield":["Freyalise's Winds","Freyalise's Winds"],"hand":["Sleep Magic"],"mana":"U"},"p1":{"battlefield":["Hill Giant"],"counters":{"Hill Giant":{"WIND":1}},"tapped":["Hill Giant"]}},
"steps":[{"op":"pass_to","step":"upkeep","active":"p1","answers":[{"kind":"any","pick":["Freyalise's Winds"]}]}],
"expect":[{"card":"p1:Hill Giant","tapped":true,"counters":{}}]}`))
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

// TestTwoDoesntUntapEffectsAskNothing: a "doesn't untap" effect is a restriction,
// not a replacement, so two on one creature pose no CR 616.1 order ask.
func TestTwoDoesntUntapEffectsAskNothing(t *testing.T) {
	t.Parallel()
	playRuleScenario(t, `{"name":"sleep-magic-x2-no-ask","cr":["502.3","616.1"],"why":"no order ask","setup":{"p0":{"hand":["Sleep Magic","Sleep Magic"],"mana":"UU"},"p1":{"battlefield":["Hill Giant"]}},"steps":[{"op":"cast","seat":0,"card":"p0:Sleep Magic","mana":"U","targets":["p1:Hill Giant"]},{"op":"resolve"},{"op":"resolve"},{"op":"cast","seat":0,"card":"p0:Sleep Magic#1","mana":"U","targets":["p1:Hill Giant"]},{"op":"resolve"},{"op":"resolve"},{"op":"pass_to","step":"upkeep","active":"p1"}],"expect":[{"card":"p1:Hill Giant","tapped":true}]}`)
}

// TestStunCounterStaysOnADoesntUntapPermanent: CR 122.1d's replacement applies
// only when the permanent "would become untapped"; Sleep Magic means it would
// not, so the untap step leaves the stun counter in place.
func TestStunCounterStaysOnADoesntUntapPermanent(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleAuditScenarioJSON(reg, []byte(`{"name":"stun-with-doesnt-untap","cr":["122.1d"],"why":"the stun counter is not spent",
"setup":{"p0":{"hand":["Sleep Magic"],"mana":"U"},"p1":{"battlefield":["Hill Giant"],"counters":{"Hill Giant":{"STUN":1}}}},
"steps":[{"op":"cast","seat":0,"card":"p0:Sleep Magic","mana":"U","targets":["p1:Hill Giant"]},{"op":"resolve"},{"op":"resolve"},
{"op":"pass_to","step":"upkeep","active":"p1"}],
"expect":[{"card":"p1:Hill Giant","tapped":true,"counters":{"STUN":1}}]}`), true)
	if err != nil || len(res.Fails) != 0 {
		t.Fatalf("err=%v fails=%v", err, res.Fails)
	}
}

// An untap-all sweep proposes the same untap TryUntap does: a stun counter comes
// off instead, on a noncreature as on a creature, and a permanent without one
// untaps. Forge and XMage both play Unstoppable Plan this way.
func TestUntapAllRemovesAStunCounterInstead(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleAuditScenarioJSON(reg, []byte(`{"name":"stun-untap-all","cr":["122.1d"],"why":"an untap sweep meets a stun counter",
"setup":{"p0":{"battlefield":["Unstoppable Plan","Hill Giant","Grizzly Bears","Chromatic Star"],"tapped_after_untap":["Hill Giant","Grizzly Bears","Chromatic Star"],"counters":{"Hill Giant":{"STUN":2},"Chromatic Star":{"STUN":1}}}},
"steps":[{"op":"pass_to","step":"end"},{"op":"resolve"}],
"expect":[{"card":"p0:Hill Giant","tapped":true,"counters":{"STUN":1}},{"card":"p0:Chromatic Star","tapped":true},{"card":"p0:Grizzly Bears","tapped":false}]}`), true)
	if err != nil || len(res.Fails) != 0 {
		t.Fatalf("err=%v fails=%v", err, res.Fails)
	}
}
