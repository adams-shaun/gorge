package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestConvokeAnnouncementNamesTheHelpersItNeeds: {X}{1}{R}{W} at X = 0, with the
// mana floating and the creatures on the board, needs exactly the named helpers.
func TestConvokeAnnouncementNamesTheHelpersItNeeds(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		pool     string
		creature []string
		want     map[string]string // creature -> the option Kind it should pay with
	}{
		{"the pool pays it all", "RWG", []string{"Grizzly Bears"}, nil},
		{"one short, generic", "RW", []string{"Grizzly Bears"}, map[string]string{"Grizzly Bears": "convoke_generic"}},
		{
			"a pip the pool lacks, not the creature that could pay generic",
			"WG", []string{"Grizzly Bears", "Goblin Piker"}, map[string]string{"Goblin Piker": "convoke_R"},
		},
		{
			"no mana at all", "", []string{"Grizzly Bears", "Goblin Piker", "Savannah Lions"},
			map[string]string{"Grizzly Bears": "convoke_generic", "Goblin Piker": "convoke_R", "Savannah Lions": "convoke_W"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			board := map[string]state.Zone{"Wildfire Awakener": state.ZHand}
			for _, n := range tc.creature {
				board[n] = state.ZBattlefield
			}
			e, _, mine, _ := cr601Board(t, 60103, board, map[string]state.Zone{"Mountain": state.ZBattlefield})
			if tc.pool != "" {
				addMana(t, e, 0, tc.pool)
			}
			edrSeatZeroPriority(t, e)
			cr601Cast(t, e, mine["Wildfire Awakener"], "")
			d := e.Pending()
			if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 {
				t.Fatalf("pending = %+v, want the helper announcement", d)
			}
			got := map[string]string{}
			for _, i := range d.PaymentHelpers {
				o := d.Options[i]
				got[e.G.Obj(o.Obj).Face().Name] = o.Kind
			}
			if len(got) != len(tc.want) {
				t.Fatalf("helpers = %v, want %v", got, tc.want)
			}
			for name, kind := range tc.want {
				if got[name] != kind {
					t.Fatalf("helpers = %v, want %v", got, tc.want)
				}
			}
		})
	}
}
