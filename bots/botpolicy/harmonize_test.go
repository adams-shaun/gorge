package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// TestBotHarmonizeTapsHighestPower pins the bot's harmonize arm: CR 702.180a/b
// lets the caster tap one creature and choose which, so the bot takes the
// largest offered power rather than the first offer.
func TestBotHarmonizeTapsHighestPower(t *testing.T) {
	d := &decision.Decision{Seq: 1, Player: 0, Kind: decision.KChoose, Min: 0, Max: 1,
		Options: []decision.Option{
			{Index: 0, Kind: "harmonize", Group: "payment:harmonize", Amount: 2},
			{Index: 1, Kind: "harmonize", Group: "payment:harmonize", Amount: 5},
		}}
	// Precondition: option 0 is the weaker one (the old first-offer fallback
	// would take it) and the maximum offered power is 5.
	if d.Options[0].Amount >= d.Options[1].Amount || d.Options[1].Amount != 5 {
		t.Fatalf("test setup: offered powers %d and %d do not rank option 1 highest",
			d.Options[0].Amount, d.Options[1].Amount)
	}
	in := Decide(Board{}, d, rng(1))
	if len(in.Choices) != 1 || in.Choices[0] != 1 {
		t.Fatalf("harmonize bot choices = %v, want [1] (the 5-power option)", in.Choices)
	}
	if err := d.Validate(in); err != nil {
		t.Fatalf("harmonize bot answer %+v rejected by Decision.Validate: %v", in, err)
	}
}
