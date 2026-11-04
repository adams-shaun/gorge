package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestTriggeredPayNotOfferedFromRestrictedFloatingMana (dirac std-fra pair
// 1395): Chandra, Chill of Compliance's +1 floated {U} that may be spent
// only to cast a noncreature spell, then a Swamp cast Lich's Relic. Its
// enter trigger's "you may pay {2}" had one untapped Swamp and that {U} to
// draw on, and the {U} cannot pay an ability's cost (CR 106.6), so the
// election offers decline only. resPayNeedsMana priced the cost against the
// raw floating pool, restricted batch included, and offered a pay that could
// only fail. The control arm floats an unrestricted {U} instead and still
// offers the pay.
func TestTriggeredPayNotOfferedFromRestrictedFloatingMana(t *testing.T) {
	for _, tc := range []struct {
		name    string
		chandra bool
		wantPay bool
	}{
		{"restricted floating U", true, false},
		{"unrestricted floating U", false, true},
	} {
		reg := searchTestRegistry(t)
		e, _ := searchEngine(t, reg, "Lich's Relic", "Chandra, Chill of Compliance")
		swamp := xFoldLand(t, e, "Test Swamp", "B")
		xFoldLand(t, e, "Test Swamp", "B")
		var blue state.ObjID
		if tc.chandra {
			blue = searchMoveByName(t, e, "Chandra, Chill of Compliance", state.ZBattlefield)
		} else {
			blue = xFoldLand(t, e, "Test Island", "U")
		}
		relic := searchMoveByName(t, e, "Lich's Relic", state.ZHand)
		driveToStep(t, e, e.G.Turn, e.G.Active, state.StepMain1)
		e.G.Active = 0
		e.pending = nil
		e.priorityRound()

		// Float the {U}: Chandra's +1 is a loyalty ability, not a mana
		// ability (CR 605.1a), so it resolves from the stack.
		pick := func(kind string, obj state.ObjID, label string) {
			t.Helper()
			for _, o := range e.Pending().Options {
				if o.Kind == kind && o.Obj == obj && (label == "" || o.Label == label) {
					submitChoices(t, e, o.Index)
					return
				}
			}
			t.Fatalf("%s: no %s option for %d %q: %+v", tc.name, kind, obj, label, e.Pending().Options)
		}
		if tc.chandra {
			pick("ability", blue, "Chandra, Chill of Compliance: Add {U}. Spend this mana only to cast a noncreature spell.")
			for i := 0; i < 8 && e.G.Players[0].Pool[state.ManaSlot("U")] == 0; i++ {
				d := e.Pending()
				if d == nil || d.Kind != decision.KPriority {
					t.Fatalf("%s: unexpected ask %s %q while Chandra's +1 resolves", tc.name, d.Kind, d.Prompt)
				}
				submitChoices(t, e, passIndex(t, d))
			}
		} else {
			pick("activate", blue, "")
		}
		if got := e.G.Players[0].Pool[state.ManaSlot("U")]; got != 1 {
			t.Fatalf("%s: floating U = %d, want 1", tc.name, got)
		}
		pick("activate", swamp, "")
		castFromPriority(t, e, relic)

		d := passUntilNonPriority(t, e, 40)
		if d.Kind != decision.KChoose || d.Source != relic {
			t.Fatalf("%s: ask %s %q, want the Relic's pay election", tc.name, d.Kind, d.Prompt)
		}
		pays := false
		for _, o := range d.Options {
			pays = pays || o.Kind == "trigger_cost_pay"
		}
		if pays != tc.wantPay {
			t.Fatalf("%s: pay offered = %v, want %v (options %+v)", tc.name, pays, tc.wantPay, d.Options)
		}
	}
}
