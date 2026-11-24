package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Farm finding SUP `p0.life` (xe-gorge-dig-remember-rest-arm, farm row
// 2026-11-24T04:49Z): gorge gained 3 life off Chrome Courier's Dig where Forge
// stayed at 20. Chrome Courier's trigger says "Put one of them into your hand
// and the other into your graveyard. If you put an artifact card into your
// hand this way, you gain 3 life." The script encodes it as
//
//	DB$ Dig | ... | DestinationZone2$ Graveyard | RememberChanged$ True | SubAbility$ DBGainLife
//	DB$ GainLife | LifeAmount$ 3 | ConditionDefined$ Remembered | ConditionPresent$ Artifact
//
// CR 608.2c says the controller follows the instructions in the order written
// and reads the whole text (the rules of English): the condition is on the
// artifact card put into the HAND, not on any card the Dig moved. Here the top
// two are Wall of Mourning (nonartifact) and Ethersworn Canonist (artifact);
// the recorded choice puts Wall of Mourning into the hand and the artifact into
// the graveyard, so the condition is false and p0's life stays 20. Forge's
// DigEffect remembers the rest-arm cards only under RememberMovedToZone$
// containing 2; RememberChanged$ alone remembers the primary (hand) arm.
func TestFarmChromeCourier(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	scenario := `{
	  "name": "farm-chrome-courier-remember-rest-arm",
	  "setup": {"p0": {"battlefield": ["Island","Plains"], "hand": ["Chrome Courier"],
	    "library_top": ["Wall of Mourning","Ethersworn Canonist"]}},
	  "steps": [
	    {"op": "cast", "seat": 0, "card": "p0:Chrome Courier", "mana": "CWU"},
	    {"op": "resolve", "seat": 0, "answers": [{"kind": "choose", "pick": ["Wall of Mourning"]}]}
	  ]
	}`
	res, err := RunOracleScenarioJSON(reg, []byte(scenario))
	if err != nil {
		t.Fatalf("scenario JSON: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("%v", res.Fails)
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	onBF := false
	for _, p := range last.Permanents {
		if p.Name == "Chrome Courier" {
			onBF = true
		}
	}
	if !onBF {
		t.Fatalf("precondition: Chrome Courier did not resolve")
	}
	p0 := last.Players[0]
	if !slices.Contains(p0.Hand, "Wall of Mourning") || !slices.Contains(p0.Graveyard, "Ethersworn Canonist") {
		t.Fatalf("precondition: the Dig did not put Wall of Mourning in hand and Ethersworn Canonist in the graveyard: hand %v graveyard %v",
			p0.Hand, p0.Graveyard)
	}
	if p0.Life != 20 {
		t.Fatalf("p0 life = %d, want 20: the artifact card went to the graveyard, so \"if you put an artifact card into your hand this way\" is false (CR 608.2c)",
			p0.Life)
	}
}
