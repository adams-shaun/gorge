package seat

import (
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/policynet"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// A checkpoint trained under a non-v1 feature set (pn12) is scored under
// that set: the seat encodes with the scorer's Features, and a v1 scorer
// encodes exactly as before.
func TestPolicyNetEncodesUnderTheCheckpointsFeatureSet(t *testing.T) {
	v := view.View{Players: []view.PlayerView{
		{ID: 0, Hand: []view.CardView{{ID: 3, Name: "Shock", Types: "Instant", ManaCost: "R"}}, Battlefield: []view.CardView{{ID: 4, Name: "Mountain", Types: "Land"}}},
		{ID: 1, Battlefield: []view.CardView{{ID: 5, Name: "Grizzly Bears", Types: "Creature", Power: 2, Toughness: 2}}},
	}}
	d := &decision.Decision{Player: 0, Kind: decision.KPriority, Options: []decision.Option{{Index: 0, Kind: "cast", Obj: 3}, {Index: 1, Kind: "pass"}}}
	for _, fs := range []policynet.FeatureSet{policynet.FeaturesV1, policynet.FeaturesMZ} {
		m := policynet.NewModel(policynet.TableRows, 4, 4, rand.New(rand.NewPCG(1, 1)))
		m.Features = fs
		b := NewPolicyNetBot(1, policynet.NewScorer(m))
		st, opts, ok := b.encode(v, d)
		if !ok {
			t.Fatal("empty surface")
		}
		if !reflect.DeepEqual(st, policynet.EncodeStateWith(fs, v, state.PlayerID(0), nil)) {
			t.Fatalf("%s: seat state encoding differs", fs)
		}
		for i := range opts {
			if !reflect.DeepEqual(opts[i], policynet.EncodeOptionWith(fs, v, 0, d.Kind, d.Options[i], i, len(d.Options))) {
				t.Fatalf("%s: seat option %d encoding differs", fs, i)
			}
		}
		if fs == policynet.FeaturesV1 && !reflect.DeepEqual(st, policynet.EncodeState(v, 0)) {
			t.Fatal("v1 seat encoding moved")
		}
	}
}
