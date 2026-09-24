package botpolicy

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/decision"
)

// The legacy bench seat declines only marked Effect elections; ordinary
// optional triggers retain the historical RNG path.
func TestLegacyEffectOptionalDeclineOnly(t *testing.T) {
	for _, options := range [][]decision.Option{
		{{Index: 0, Kind: "yes"}, {Index: 1, Kind: "no"}},
		{{Index: 0, Kind: "no"}, {Index: 1, Kind: "yes"}},
	} {
		d := decision.Decision{Seq: 4, Player: 1, Kind: decision.KTriggerOptional,
			Min: 1, Max: 1, Options: options}
		if len(d.Options) != 2 || d.Options[0].Kind == d.Options[1].Kind {
			t.Fatal("test must offer distinct accept and decline branches")
		}
		want := []int{0}
		if d.Options[0].Kind == "yes" {
			want = []int{1}
		}
		for seed := uint64(0); seed < 25; seed++ {
			d.EffectOptional = true
			legacy := LegacyDecide(Board{}, &d, rng(seed))
			if err := d.Validate(legacy); err != nil {
				t.Fatalf("seed %d: legacy answer %v invalid: %v", seed, legacy.Choices, err)
			}
			production := Decide(Board{}, &d, rng(seed))
			if !reflect.DeepEqual(legacy.Choices, want) || !reflect.DeepEqual(legacy.Choices, production.Choices) {
				t.Fatalf("seed %d: legacy %v, production %v; want decline %v", seed, legacy.Choices, production.Choices, want)
			}
			d.EffectOptional = false
			// With the marker absent, legacy must still exercise its coin.
			got := LegacyDecide(Board{}, &d, rng(seed))
			if err := d.Validate(got); err != nil {
				t.Fatalf("seed %d: unmarked legacy answer invalid: %v", seed, err)
			}
			if len(got.Choices) != 1 || got.Choices[0] != options[rng(seed).IntN(2)].Index {
				t.Fatalf("seed %d: legacy coin answer %v changed", seed, got.Choices)
			}
		}
	}
}
