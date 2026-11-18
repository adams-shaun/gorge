package decision

import (
	"encoding/json"
	"testing"
)

// TestContributionCostRoundTripsWireShape pins the published outstanding
// requirement's wire contract: it serialises only when present (the omitempty
// keeps every pre-existing decision's bytes), decodes into the PaymentCost the
// engine published, and a round trip preserves the shape a rules-ignorant
// client prices its cast-payment announcement against.
func TestContributionCostRoundTripsWireShape(t *testing.T) {
	t.Parallel()
	bare, err := json.Marshal(Decision{Seq: 1, Player: 2, Kind: KChoose})
	if err != nil {
		t.Fatal(err)
	}
	var back Decision
	if err := json.Unmarshal(bare, &back); err != nil {
		t.Fatal(err)
	}
	if back.ContributionCost != nil {
		t.Fatalf("a decision without a published cost serialises one: %s", bare)
	}
	if want := `{"seq":1,"player":2,"kind":"choose","prompt":"","min":0,"max":0,"options":null}`; string(bare) != want {
		t.Fatalf("a decision without a published cost changed its bytes: %s, want %s", bare, want)
	}

	d := Decision{Seq: 2, Player: 0, Kind: KChoose,
		ContributionCost: &PaymentCost{Generic: 3, Mana: ManaAmount{0, 0, 0, 3}}}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.ContributionCost == nil || back.ContributionCost.Generic != 3 || back.ContributionCost.Mana[3] != 3 {
		t.Fatalf("round trip lost the published cost: %s -> %+v", raw, back.ContributionCost)
	}
}

// TestContributionCostCloneIsOwned pins CloneInto's copy: the clone's
// published cost is its own allocation (the projecting caller refills one
// Decision per call), and mutating the source afterwards cannot reach it.
func TestContributionCostCloneIsOwned(t *testing.T) {
	t.Parallel()
	d := Decision{ContributionCost: &PaymentCost{Generic: 1, Mana: ManaAmount{2}}}
	var c Decision
	c.ContributionCost = &PaymentCost{Generic: 9}
	d.CloneInto(&c)
	if c.ContributionCost == nil || c.ContributionCost == d.ContributionCost || c.ContributionCost.Generic != 1 {
		t.Fatalf("CloneInto did not write its own PaymentCost: %+v", c.ContributionCost)
	}
	d.ContributionCost.Generic = 7
	if c.ContributionCost.Generic != 1 {
		t.Fatalf("the clone shares the source's published cost: %+v", c.ContributionCost)
	}
}
