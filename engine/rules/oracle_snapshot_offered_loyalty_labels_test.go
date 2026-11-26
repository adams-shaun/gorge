package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// cometLoyaltyLabelsScenario is xproof find 11260248's Comet, Stellar Pup +
// Way of the Deathbringer setup, stripped to the runner's schema (the find's
// `compare: [offered]` scenario, whose first divergence is the setup
// snapshot's offered rows). Way of the Deathbringer's AddAbility$ static
// grants every planeswalker you control a [-2] loyalty ability; Comet's own
// [0] ability is the die roll whose rules text carries six branches.
const cometLoyaltyLabelsScenario = `{"name":"comet-offered-loyalty-labels","cr":["606"],"why":"the setup offered rows name a planeswalker's loyalty abilities the way the card's rules text does","setup":{"p0":{"battlefield":["Way of the Deathbringer","Comet, Stellar Pup"],"library_top":["Mountain","Mountain","Mountain","Swamp","Swamp","Swamp","Swamp","Plains","Plains","Mountain","Swamp","Plains","Swamp","Mountain","Swamp","Plains","Swamp","Plains","Mountain","Mountain","Mountain","Plains","Mountain","Mountain","Swamp","Swamp","Swamp","Plains","Swamp","Plains","Plains","Plains","Plains","Plains","Mountain","Swamp","Plains","Mountain"],"life":20},"p1":{"library_top":["Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains","Plains"],"life":20}},"steps":[{"op":"activate","seat":0,"card":"p0:Comet, Stellar Pup","ability_index":0,"ability":"Comet, Stellar Pup: You may sacrifice a creature. If you do, create a 4/4 green Beast creature token with trample."},{"op":"pass","seat":0},{"op":"pass","seat":1},{"op":"pass","seat":0,"answers":[{"kind":"modes","pick":["Don't pay"]}]},{"op":"pass","seat":1},{"op":"pass","seat":0},{"op":"pass","seat":1},{"op":"pass","seat":0},{"op":"pass","seat":1},{"op":"pass","seat":0},{"op":"pass","seat":1},{"op":"pass","seat":0},{"op":"pass","seat":1},{"op":"pass","seat":0},{"op":"pass","seat":1}]}`

// TestOracleSnapshotOfferedLoyaltyLabelsCarryTheCostAndWholeText pins the
// setup snapshot's loyalty-activation labels (the xproof `compare:
// [offered]` surface) to the card's own rules-text spelling: the loyalty
// cost is named in front of the body ("0:", "-2:"), and a multi-branch
// ability -- Comet's [0] die roll, six CR 709 branches -- is labelled with
// its WHOLE rules text, not the one branch a run's own random roll would
// render, so a follower's label can ever match. Only the snapshot's rows
// spell this way; the decision options the bots and the scenario steps
// read keep the bare description.
func TestOracleSnapshotOfferedLoyaltyLabelsCarryTheCostAndWholeText(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleScenarioJSON(reg, []byte(cometLoyaltyLabelsScenario))
	if err != nil {
		t.Fatal(err)
	}
	// The find's steps replay cleanly: the labels the snapshot spells are
	// presentation-only, so the scenario's own activate step -- whose
	// `ability` text matches the decision options' label, unchanged here --
	// still selects the same ability.
	if len(res.Fails) != 0 {
		t.Fatalf("the scenario's steps must still replay: %v", res.Fails)
	}
	if len(res.Snapshots) == 0 || res.Snapshots[0].Checkpoint != "setup" {
		t.Fatalf("precondition: snapshots = %+v", res.Snapshots)
	}
	var zero, minus2 string
	for _, o := range res.Snapshots[0].Offered {
		if o.Source != "p0:Comet, Stellar Pup" || o.Kind != "activate" {
			continue
		}
		switch {
		case strings.Contains(o.Label, "Roll a six-sided die"):
			zero = o.Label
		case strings.Contains(o.Label, "You may sacrifice a creature"):
			minus2 = o.Label
		}
	}
	if zero == "" || minus2 == "" {
		t.Fatalf("precondition: the setup offered set must list both of Comet's loyalty abilities, zero=%q minus2=%q", zero, minus2)
	}
	if !strings.HasPrefix(zero, "Comet, Stellar Pup: 0: ") {
		t.Fatalf("the [0] ability's offered label must carry the loyalty cost: %q", zero)
	}
	// The whole rules text, all six branches: the label a follower spells
	// from the card's Oracle text can only match this, never a branch render.
	for _, branch := range []string{"1 or 2", "3 ", "4 or 5", "6 "} {
		if !strings.Contains(zero, branch) {
			t.Fatalf("the [0] ability's offered label must carry the ability's whole rules text, not one roll branch: %q", zero)
		}
	}
	if !strings.HasPrefix(minus2, "Comet, Stellar Pup: -2: ") {
		t.Fatalf("the [-2] ability's offered label must carry the loyalty cost: %q", minus2)
	}
	if !strings.Contains(minus2, "You may sacrifice a creature. If you do, create a 4/4 green Beast creature token with trample.") {
		t.Fatalf("the [-2] ability's offered label must carry the granted ability's own text: %q", minus2)
	}
}
