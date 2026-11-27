package policynet

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// TestSplitOmniscientRecordCarriesLibraryOrder: the record's library order
// reaches the Diag top first (own and opponents apart), and depth 0 is
// SplitOmniscientView unchanged.
func TestSplitOmniscientRecordCarriesLibraryOrder(t *testing.T) {
	lib := func(names ...string) []view.CardView {
		out := make([]view.CardView, len(names))
		for i, n := range names {
			out[i] = view.CardView{Name: n}
		}
		return out
	}
	rec := view.OmniscientRecord{
		View: view.View{Players: []view.PlayerView{
			{ID: 0, Hand: lib("H0")}, {ID: 1, Hand: lib("H1")}, {ID: 2, Hand: lib("H2")},
		}},
		Libraries: [][]view.CardView{lib("a", "b", "c"), lib("x", "y"), lib("p", "q", "r")},
	}
	const seat = state.PlayerID(1)
	v, diag := SplitOmniscientRecord(rec, seat, 2)
	if want := []string{"x", "y"}; !reflect.DeepEqual(diag.OwnLibraryTop, want) {
		t.Fatalf("own tops = %v, want %v", diag.OwnLibraryTop, want)
	}
	if want := []string{"a", "b", "p", "q"}; !reflect.DeepEqual(diag.OppLibraryTop, want) {
		t.Fatalf("opp tops = %v, want %v", diag.OppLibraryTop, want)
	}
	if len(diag.OppHand) != 2 || v.Players[0].Hand != nil || v.Players[1].Hand == nil {
		t.Fatalf("hands not split as SplitOmniscientView does: %+v", diag.OppHand)
	}
	v0, d0 := SplitOmniscientRecord(rec, seat, 0)
	wv, wd := SplitOmniscientView(rec.View, seat)
	if !reflect.DeepEqual(v0, wv) || !reflect.DeepEqual(d0, wd) {
		t.Fatal("depth 0 differs from SplitOmniscientView")
	}
}
