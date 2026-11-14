package pickbot_test

import (
	"math"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	pb "github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
)

// TestCommanderValueTable: with a Value hook the Rank Value term follows
// the table for nonlands (and the builder reads the same hook), and
// CommanderValue inverts the heuristic bridge.
func TestCommanderValueTable(t *testing.T) {
	reg := corpus(t)
	var names []string
	for _, n := range fdnMainSetNames(t) {
		c, ok := reg.Lookup(n)
		if ok && len(c.Faces) > 0 && !c.Faces[0].IsLand() && !deck.IsCommanderEligible(c) {
			names = append(names, n)
		}
		if len(names) == 2 {
			break
		}
	}
	if len(names) != 2 {
		t.Fatalf("found %d nonland noncommander cards, want 2", len(names))
	}
	table := map[string]float64{names[0]: 7.5, names[1]: -3}
	val := func(_ *cards.Card, ref limited.CardRef) float64 { return table[ref.Name] }
	pack := []limited.CardRef{ref("a", names[0], limited.Common), ref("b", names[1], limited.Common)}
	d := &pb.CommanderDrafter{Lookup: lookupOf(reg), Value: val}
	got, _ := d.Rank(pb.PickView{Set: "FDN", Round: 1, PickNo: 1, Pack: pack})
	for i, n := range names {
		if got[i].Value != table[n] {
			t.Errorf("%s: Value term %v, want %v", n, got[i].Value, table[n])
		}
	}
	if base, _ := (&pb.CommanderDrafter{Lookup: lookupOf(reg)}).Rank(pb.PickView{Set: "FDN", Round: 1, PickNo: 1, Pack: pack}); base[0].Value == table[names[0]] {
		t.Fatalf("precondition: the heuristic already values %s at %v", names[0], base[0].Value)
	}

	tun := pb.DefaultTuning
	ov, err := pb.ParseRatingsOverride([]byte(`{"x":{"v":0.55},"y":{"v":0.57}}`), "t.json", "v", "MBC")
	if err != nil {
		t.Fatal(err)
	}
	cv := ov.CommanderValue(tun)
	for _, n := range []string{"x", "y"} {
		want := (ov.Value(n)-tun.RatingCentre)*tun.RatingScale/tun.HeuristicScale + tun.HeuristicCentre
		if g := cv(nil, limited.CardRef{Name: n}); math.Abs(g-want) > 1e-9 {
			t.Errorf("CommanderValue(%s) = %v, want %v", n, g, want)
		}
		// the bridge round-trips: the persona's centred score is the override's
		if c := (cv(nil, limited.CardRef{Name: n}) - tun.HeuristicCentre) * tun.HeuristicScale; math.Abs(c-(ov.Value(n)-tun.RatingCentre)*tun.RatingScale) > 1e-9 {
			t.Errorf("%s: centred score %v does not round-trip", n, c)
		}
	}
}
