package botpolicy

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/decision"
)

// The legacy bench seat is also an unattended driver. It must not take an
// OptionalDecider$ election on the named player's behalf even when its RNG
// would previously have landed on "yes".
func TestLegacyTriggerOptionalDeclinesLikeProduction(t *testing.T) {
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
			legacy := LegacyDecide(Board{}, &d, rng(seed))
			if err := d.Validate(legacy); err != nil {
				t.Fatalf("seed %d: legacy answer %v invalid: %v", seed, legacy.Choices, err)
			}
			production := Decide(Board{}, &d, rng(seed))
			if !reflect.DeepEqual(legacy.Choices, want) || !reflect.DeepEqual(legacy.Choices, production.Choices) {
				t.Fatalf("seed %d: legacy %v, production %v; want decline %v", seed, legacy.Choices, production.Choices, want)
			}
		}
	}
}
