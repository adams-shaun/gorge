package searchseat

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/internal/policynet"
	"github.com/adams-shaun/gorge/view"
)

func valueModel(fs policynet.FeatureSet) *policynet.Model {
	m := policynet.NewModel(policynet.TableRows, 8, 6, rand.New(rand.NewPCG(1, 2)))
	m.InitValue(5, rand.New(rand.NewPCG(3, 4)))
	m.Features = fs
	return m
}

// omniViews are two omniscient leaf views that differ ONLY in the opponent's
// hand.
func omniViews() (view.View, view.View) {
	mk := func(opp view.CardView) view.View {
		return view.View{Turn: 5, Players: []view.PlayerView{
			{ID: 0, Life: 14, HandSize: 1, Hand: []view.CardView{{Name: "Lightning Bolt", Types: "Instant", ManaCost: "R"}}},
			{ID: 1, Life: 9, HandSize: 1, Hand: []view.CardView{opp}},
		}}
	}
	return mk(view.CardView{Name: "Counterspell", Types: "Instant", ManaCost: "U U"}),
		mk(view.CardView{Name: "Grizzly Bears", Types: "Creature", ManaCost: "1 G", Power: 2, Toughness: 2})
}

// The oracle leaf reads the opponent's hand off the omniscient view, encoded
// as the Diag the model was trained with; a redacted value model scoring the
// same two views cannot tell them apart.
func TestOracleLeafReadsTheOpponentHand(t *testing.T) {
	a, b := omniViews()
	m := valueModel(policynet.FeaturesMZOppHand)
	leaf := oracleLeaf(m)
	if leaf == nil {
		t.Fatal("oracle model gave no leaf")
	}
	own, diag := policynet.SplitOmniscientView(a, 0)
	if want := float64(m.Value(policynet.EncodeStateWith(m.Features, own, 0, diag))); leaf(a, 0) != want {
		t.Fatalf("oracle leaf %v, want the value head on the split view %v", leaf(a, 0), want)
	}
	if leaf(a, 0) == leaf(b, 0) {
		t.Fatalf("oracle leaf ignored the opponent hand: %v", leaf(a, 0))
	}
	red := valueLeaf(valueModel(policynet.FeaturesMZ))
	if red(a, 0) != red(b, 0) {
		t.Fatalf("redacted leaf read the opponent hand: %v vs %v", red(a, 0), red(b, 0))
	}
	if oracleLeaf(nil) != nil {
		t.Fatal("nil oracle model must give no leaf")
	}
}

func TestOptionsValidateTheOracleLeaf(t *testing.T) {
	oracle := valueModel(policynet.FeaturesMZOppHand)
	noValue := policynet.NewModel(policynet.TableRows, 8, 6, rand.New(rand.NewPCG(1, 2)))
	noValue.Features = policynet.FeaturesMZOppHand
	for _, tc := range []struct {
		name string
		o    Options
		want string // "" = valid
	}{
		{"zero", Options{}, ""},
		{"defaults", Defaults(), ""},
		{"redacted value", Options{Value: valueModel(policynet.FeaturesMZ)}, ""},
		{"oracle", Options{OracleValue: oracle}, ""},
		{"oracle model as the redacted leaf", Options{Value: oracle}, "oracle model"},
		{"redacted model as the omniscient leaf", Options{OracleValue: valueModel(policynet.FeaturesMZ)}, "reads no hidden information"},
		{"mz-oracle needs library order", Options{OracleValue: valueModel(policynet.FeaturesMZOracle)}, "library order"},
		{"oracle on the clairvoyant ceiling", Options{OracleValue: oracle, Clairvoyant: true}, "Clairvoyant"},
		{"both leaves", Options{OracleValue: oracle, Value: valueModel(policynet.FeaturesMZ)}, "exclusive"},
		{"oracle without a value head", Options{OracleValue: noValue}, "no value head"},
	} {
		err := tc.o.Validate()
		if (err == nil) != (tc.want == "") || (err != nil && !strings.Contains(err.Error(), tc.want)) {
			t.Errorf("%s: Validate() = %v, want %q", tc.name, err, tc.want)
		}
	}
}
