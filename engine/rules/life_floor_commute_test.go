package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
)

// CR 616.1: two life floors (Worship, Elderscale Wurm) compete for one damage event, but
// the tighter cap wins in either order and a met cap leaves the other inapplicable, so
// the affected player has nothing to choose and gorge poses no order ask (the kernel
// never asks either: tail-55-c000 pair 2, two Worships).
func TestLifeFloorsCommuteWithoutOrderChoice(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		cards []string
		life  int32
		want  int32
	}{
		{"two Worships", []string{"Worship", "Worship", "Grizzly Bears"}, 5, 1},
		{"Worship beside Elderscale Wurm", []string{"Worship", "Elderscale Wurm"}, 10, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := layerEngine(t)
			for _, n := range tc.cards {
				onBoardCard(t, e, 0, corpusCard(t, n))
			}
			e.G.Players[0].Life = tc.life
			e.emit(events.Event{Kind: events.Damage, Player: 0, Amount: 12})
			if d := e.Pending(); d != nil && d.Kind == decision.KReplacement {
				t.Fatalf("commuting life floors posed an order choice: %+v", d)
			}
			if got := e.G.Players[0].Life; got != tc.want {
				t.Fatalf("life after 12 damage = %d, want %d", got, tc.want)
			}
		})
	}
}
