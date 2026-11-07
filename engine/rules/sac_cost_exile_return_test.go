package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestSacrificeCostReturnsExiledAfterAllSacrificed pins CR 701.21a and
// 610.3d: the permanents sacrificed for one cost leave simultaneously, so a
// Banishing Light among them returns the card it exiled only after every one
// of them is gone, not between two of the cost's sacrifices. Both cost sites
// are covered: an activated ability's cost and a triggered ability's
// resolution-time Cost$ (Scavenger's Talent's "sacrifice three other nonland
// permanents").
func TestSacrificeCostReturnsExiledAfterAllSacrificed(t *testing.T) {
	t.Parallel()
	const (
		trinket = "Name:Trinket\nTypes:Artifact\nOracle:x\n"
		altar   = "Name:Altar\nTypes:Enchantment\nA:AB$ GainLife | Cost$ Sac<3/Permanent.Other+nonLand/other nonland permanent> | Defined$ You | LifeAmount$ 1 | SpellDescription$ Gain 1 life.\nOracle:x\n"
		talent  = "Name:Talent\nTypes:Enchantment\nT:Mode$ SpellCast | ValidActivatingPlayer$ You | TriggerZones$ Battlefield | Execute$ TrigGain | TriggerDescription$ x\nSVar:TrigGain:AB$ GainLife | Cost$ Sac<3/Permanent.Other+nonLand/other nonland permanent> | Defined$ You | LifeAmount$ 1\nOracle:x\n"
		spark   = "Name:Spark\nManaCost:R\nTypes:Instant\nA:SP$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"
	)
	for _, tc := range []struct {
		name    string
		trigger bool
	}{{"activated", false}, {"triggered", true}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			light := card(t, "Name:Banishing Light\nManaCost:2 W\nTypes:Enchantment\n"+
				"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigExile\n"+
				"SVar:TrigExile:DB$ ChangeZone | Origin$ Battlefield | Destination$ Exile | ValidTgts$ Permanent.nonLand+OppCtrl | TgtPrompt$ Select target | Duration$ UntilHostLeavesPlay\n"+
				"Oracle:x\n")
			e := handEngine(t, light, card(t, spark))
			bear := onBoard(t, e, 1, "Name:Runeclaw Bear\nManaCost:1 G U\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
			e.G.Players[0].Pool[state.MW] = 1
			e.G.Players[0].Pool[state.MC] = 2
			e.askPriority(0)
			castFirst(t, e, "cast")
			d := passToTargetAsk(t, e)
			idx := -1
			for _, o := range d.Options {
				if o.Obj == bear {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("bear not offered as the exile target: %+v", d.Options)
			}
			submitChoices(t, e, idx)
			passUntilStackEmpty(t, e, 8)
			if z := e.G.Obj(bear).Zone; z != state.ZExile {
				t.Fatalf("bear in zone %v after Banishing Light's trigger, want exile", z)
			}
			var lightID state.ObjID
			for _, id := range e.G.Zone(state.ZBattlefield, 0) {
				if len(e.G.Obj(id).ExileReturn) > 0 {
					lightID = id
				}
			}
			if lightID == 0 || len(e.G.Obj(lightID).ExileReturn) != 1 {
				t.Fatalf("Banishing Light %d holds no until-leaves return", lightID)
			}
			sacs := map[state.ObjID]bool{lightID: true, onBoard(t, e, 0, trinket): true, onBoard(t, e, 0, trinket): true}
			mark := len(e.L.Events)
			life := e.G.Players[0].Life
			if tc.trigger {
				onBoard(t, e, 0, talent)
				e.G.Players[0].Pool[state.MR] = 1
				e.askPriority(0)
				castFirst(t, e, "cast")
			} else {
				activateAbilityOn(t, e, onBoard(t, e, 0, altar))
			}
			for i := 0; i < 40; i++ {
				d := e.Pending()
				if d == nil || e.G.Over {
					break
				}
				if d.Kind == decision.KPriority {
					if len(e.G.Stack) == 0 {
						break
					}
					castFirst(t, e, "pass")
					continue
				}
				n := d.Min
				if n == 0 {
					n = 1
				}
				choices := make([]int, 0, n)
				for j := 0; j < n && j < len(d.Options); j++ {
					choices = append(choices, d.Options[j].Index)
				}
				submitChoices(t, e, choices...)
			}
			if got := e.G.Players[0].Life; got <= life {
				t.Fatalf("the sacrifice cost's ability never resolved (life %d, was %d)", got, life)
			}
			lastSac, ret := -1, -1
			for i, ev := range e.L.Events[mark:] {
				if ev.Kind != events.MoveZone {
					continue
				}
				if sacs[ev.Obj] && ev.From == state.ZBattlefield {
					lastSac = i
				}
				if ev.Obj == bear && ev.From == state.ZExile && ev.To == state.ZBattlefield {
					ret = i
				}
			}
			for id := range sacs {
				if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
					t.Fatalf("sacrificed %d in zone %v, want graveyard", id, z)
				}
			}
			if ret < 0 {
				t.Fatal("the bear never returned from exile")
			}
			if ret < lastSac {
				t.Fatalf("the bear returned (event +%d) before the cost's last sacrifice (event +%d)", ret, lastSac)
			}
		})
	}
}
