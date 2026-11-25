package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Farm finding SUP-s109-supp (xe:RAND:1:9:w10 and siblings, field `stack`):
// at the end step gorge's stack was [] while XMage's was
// [ability:Laboratory Drudge@p0]. Laboratory Drudge reads "At the beginning of
// each end step, draw a card if you've cast a spell from a graveyard or
// activated an ability of a card in a graveyard this turn." The "if" follows
// the effect, so it is not an intervening-if (CR 603.4: "this rule only
// applies to an 'if' that immediately follows a trigger condition"); the
// 2020-11-10 ruling: the ability "will trigger at the beginning of each end
// step whether you've cast a spell from a graveyard ... or not". XMage
// (LaboratoryDrudge.java, a ConditionalOneShotEffect checked at resolution)
// is right; the script's CheckSVar$ NumCast on the T: line is a resolution
// check, so with no graveyard cast the ability triggers and draws nothing.
func TestFarmLaboratoryDrudge(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleScenarioJSON(reg, []byte(`{
	  "name": "farm-laboratory-drudge-trailing-if",
	  "setup": {"p0": {"battlefield": ["Laboratory Drudge"], "library_top": ["Island","Swamp","Forest"]}},
	  "steps": [
	    {"op": "pass_to", "seat": 0, "step": "end", "active": "p0"},
	    {"op": "resolve", "seat": 0}
	  ]
	}`))
	if err != nil {
		t.Fatalf("scenario JSON: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("%v", res.Fails)
	}
	if len(res.Snapshots) < 3 {
		t.Fatalf("got %d snapshots, want setup, end step and post-resolve", len(res.Snapshots))
	}
	setup, atEnd, last := res.Snapshots[0], res.Snapshots[1], res.Snapshots[len(res.Snapshots)-1]
	onBF := false
	for _, p := range setup.Permanents {
		if p.Name == "Laboratory Drudge" && p.Controller == 0 {
			onBF = true
		}
	}
	if !onBF {
		t.Fatalf("precondition: p0 does not control Laboratory Drudge at setup")
	}
	if len(atEnd.Stack) != 1 || atEnd.Stack[0].Kind != "ability" || atEnd.Stack[0].Source != "p0:Laboratory Drudge" || atEnd.Stack[0].Controller != 0 {
		t.Fatalf("end-step stack = %+v, want [ability:Laboratory Drudge@p0] (CR 603.4: a trailing if is not an intervening-if)", atEnd.Stack)
	}
	if len(last.Stack) != 0 {
		t.Fatalf("stack after resolve = %+v, want empty", last.Stack)
	}
	if got, want := len(last.Players[0].Hand), len(setup.Players[0].Hand); got != want {
		t.Fatalf("p0 hand %d after resolution, want %d: no graveyard cast this turn, so the ability draws nothing", got, want)
	}
}

// TestTrailingIfSentenceShape pins the sentence shape that moves a CheckSVar$
// clause off the fire-time gate: only an "if" after the effect does (CR 603.4).
func TestTrailingIfSentenceShape(t *testing.T) {
	t.Parallel()
	for desc, want := range map[string]bool{
		"At the beginning of each end step, draw a card if you've cast a spell from a graveyard or activated an ability of a card in a graveyard this turn.":  true,
		"Whenever CARDNAME deals combat damage to a player, draw a card if you had two or more creatures enter the battlefield under your control this turn.": true,
		"Whenever CARDNAME attacks, if you've completed a dungeon, defending player loses 1 life for each card they own in exile with a hit counter on it.":   false,
		"Raid — At the beginning of your end step, if you attacked this turn, create a 1/1 red Devil creature token.":                                         false,
		"At the beginning of your upkeep, roll two six-sided dice. If you rolled 7, sacrifice CARDNAME.":                                                      false,
		"When you cast this spell, draw four cards.": false,
	} {
		if got := cards.TrailingIfSentence(desc); got != want {
			t.Errorf("trailingIfSentence(%q) = %v, want %v", desc, got, want)
		}
	}
}
