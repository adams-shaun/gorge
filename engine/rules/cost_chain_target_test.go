package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// A spell "targets" a creature through any link of its chain (CR 601.2c, 601.2f): the
// creature's cost static reads the sub-ability's target as it reads the root's.
// Dissection Practice aimed at Terror of the Peaks through its Pump link pays the 3 life.
func TestRaiseCostReadsAChainLinksTarget(t *testing.T) {
	t.Parallel()
	const spell = "Name:Chain Practice\nManaCost:B\nTypes:Instant\n" +
		"A:SP$ LoseLife | ValidTgts$ Opponent | LifeAmount$ 1 | SubAbility$ DBPump | SpellDescription$ x\n" +
		"SVar:DBPump:DB$ Pump | ValidTgts$ Creature | TargetMin$ 0 | TargetMax$ 1 | NumAtt$ +1 | NumDef$ +1 | SpellDescription$ y\nOracle:x\n"
	const dragon = "Name:Chain Dragon\nManaCost:1\nTypes:Creature Dragon\nPT:5/4\n" +
		"S:Mode$ RaiseCost | ValidTarget$ Card.Self | Activator$ Player.Opponent | Type$ Spell | Cost$ PayLife<3> | Description$ x\nOracle:x\n"
	e, _, id := newFixtureDeck(t, 9800, spell)
	e.G.Players[0].Pool[state.MB] = 1
	target := onBoard(t, e, 1, dragon)
	life := e.G.Players[0].Life
	ppPriority(t, e)
	ppChoose(t, e, func(o decision.Option) bool { return o.Kind == "cast" && o.Obj == id })
	for i := 0; i < 8 && e.Pending() != nil && e.Pending().Kind != decision.KPriority; i++ {
		d := e.Pending()
		pick := 0
		if d.Kind == decision.KTarget {
			for j, o := range d.Options {
				if o.Obj == target {
					pick = j
				}
			}
		}
		answerAsk(t, e, pick)
	}
	if got := e.G.Players[0].Life; got != life-3 {
		t.Fatalf("life %d after casting at the Dragon, want %d", got, life-3)
	}
}
