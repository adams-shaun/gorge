package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Both printed replacements compete for the same instruction: the affected
// player, not source scan order, chooses whether the +1 applies before draw.
func TestScryReplacementOrderChoiceAndResume(t *testing.T) {
	for _, tc := range []struct {
		name  string
		first string
		draw  int
	}{
		{"kenessos_first", "Kenessos, Priest of Thassa", 4},
		{"eligeth_first", "Eligeth, Crossroads Augur", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, _, id := scryFixture(t, 9123)
			k := onBoardCard(t, e, 0, corpusCard(t, "Kenessos, Priest of Thassa"))
			el := onBoardCard(t, e, 0, corpusCard(t, "Eligeth, Crossroads Augur"))
			for _, obj := range []state.ObjID{k, el} {
				o := e.G.Obj(obj)
				if o == nil || o.Zone != state.ZBattlefield || len(o.Face().Repls) == 0 || o.Face().Repls[0].Event != "Scry" {
					t.Fatalf("precondition: replacement source %d not on battlefield with a Scry replacement: %+v", obj, o)
				}
			}
			if n := len(e.G.Zone(state.ZLibrary, 0)); n < 5 {
				t.Fatalf("precondition: library %d cards, want 5", n)
			}
			beforeHand, beforeLib := len(e.G.Zone(state.ZHand, 0)), len(e.G.Zone(state.ZLibrary, 0))
			d := castFixture(t, e, id, -1)
			if d == nil || d.Kind != decision.KReplacement || d.Player != 0 || len(d.Options) != 2 || e.G.Obj(id).Zone != state.ZStack {
				t.Fatalf("precondition: expected seat 0's suspended replacement choice over two active sources, got %v", d)
			}
			idx := -1
			for _, opt := range d.Options {
				if opt.Kind == "replacement" && ((tc.first == "Kenessos, Priest of Thassa" && opt.Obj == k) || (tc.first == "Eligeth, Crossroads Augur" && opt.Obj == el)) {
					idx = opt.Index
				}
			}
			if idx < 0 {
				t.Fatalf("precondition: first replacement %q not offered: %+v", tc.first, d.Options)
			}
			submitChoices(t, e, idx)
			if got := e.Pending(); got != nil && (got.Kind == decision.KArrange || got.Kind == decision.KReplacement) {
				t.Fatalf("replaced scry must finish without arranging or re-asking: %+v", got)
			}
			passUntilStackEmpty(t, e, 20)
			if got := len(e.G.Zone(state.ZHand, 0)) - beforeHand; got != tc.draw-1 {
				t.Fatalf("net hand delta = %d, want %d (cast one, draw %d)", got, tc.draw-1, tc.draw)
			}
			if got := beforeLib - len(e.G.Zone(state.ZLibrary, 0)); got != tc.draw {
				t.Fatalf("library delta = %d, want %d draws", got, tc.draw)
			}
			if marks := scryMarkers(e); len(marks) != 0 {
				t.Fatalf("replaced scry left Scry records: %+v", marks)
			}
			for _, ev := range e.L.Events {
				if ev.Kind == events.Note && ev.Text == "unimplemented Scry replacement" {
					t.Fatalf("unimplemented replacement: %+v", ev)
				}
			}
		})
	}
}
