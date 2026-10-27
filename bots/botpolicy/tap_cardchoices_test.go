package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// TestBotTapPickAnswersTheMinimumAndValidates: a Tap CardChoices$ ask is
// answered with the R-9 stand-in (none for an AnyNumber$ Min 0 pick, the first
// n for a fixed count), and the answer passes Decision.Validate.
func TestBotTapPickAnswersTheMinimumAndValidates(t *testing.T) {
	opts := []decision.Option{
		{Index: 0, Kind: "tap", Label: "A", Obj: 11},
		{Index: 1, Kind: "tap", Label: "B", Obj: 12},
		{Index: 2, Kind: "tap", Label: "C", Obj: 13},
	}
	for _, tc := range []struct {
		name     string
		min, max int
		want     int
	}{{"any number", 0, 3, 0}, {"fixed two", 2, 2, 2}} {
		d := &decision.Decision{Seq: 1, Player: 0, Kind: decision.KChoose, Min: tc.min, Max: tc.max, Options: opts}
		in := Decide(Board{}, d, rng(1))
		if len(in.Choices) != tc.want {
			t.Fatalf("%s: choices = %v, want %d of them", tc.name, in.Choices, tc.want)
		}
		if err := d.Validate(in); err != nil {
			t.Fatalf("%s: bot answer %v rejected: %v", tc.name, in.Choices, err)
		}
	}
}
