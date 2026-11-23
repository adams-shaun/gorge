package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// CR 614.12: a replacement that modifies how a permanent enters may come from
// the permanent itself only if it affects that permanent alone. Kami of
// Whispered Hopes's "a permanent you control" line is a general subset, so it
// must not add its one to the counters Kami itself enters with. Two Effect-made
// enters-with-counter bodies on a counterless entrant are two puts (nothing
// stages the entry), and the self-exclusion must hold for both, as it does for
// a folded grant: each put is 1 + Hardened Scales's 1 = 2, so 4 in all, not 6.

const kamiFixtureSrc = "Name:Fixture Kami\nManaCost:2 G\nTypes:Creature Spirit\nPT:1/1\n" +
	"R:Event$ AddCounter | ActiveZones$ Battlefield | ValidCard$ Permanent.YouCtrl+inZoneBattlefield | ValidCounterType$ P1P1 | ReplaceWith$ AddOneMoreCounters | Description$ x\n" +
	"SVar:AddOneMoreCounters:DB$ ReplaceCounter | ValidCounterType$ P1P1 | ChooseCounter$ True | Amount$ X\n" +
	"SVar:X:ReplaceCount$CounterNum/Plus.1\n" +
	"Oracle:x\n"

const scalesFixtureSrc = "Name:Fixture Scales\nManaCost:2 G\nTypes:Enchantment\n" +
	"R:Event$ AddCounter | ActiveZones$ Battlefield | ValidCard$ Creature.YouCtrl | ValidCounterType$ P1P1 | ReplaceWith$ AddOneMoreCounters | Description$ x\n" +
	"SVar:AddOneMoreCounters:DB$ ReplaceCounter | ValidCounterType$ P1P1 | ChooseCounter$ True | Amount$ X\n" +
	"SVar:X:ReplaceCount$CounterNum/Plus.1\n" +
	"Oracle:x\n"

func TestEntryCountersOwnModifierExcludedUnderTwoEffects(t *testing.T) {
	t.Parallel()
	srcs := []string{kamiFixtureSrc, wildgrowthSrc, wildgrowthSrc, scalesFixtureSrc}
	e, cfg, find := etbConfig(t, seedTossSeat0(211), srcs, nil)
	putCreature(t, e, 0, wildgrowthSrc)
	putCreature(t, e, 0, wildgrowthSrc)
	putCreature(t, e, 0, scalesFixtureSrc)
	kami := find("Fixture Kami", 0)
	addMana(t, e, 0, "GGG") // one colour spent: each effect puts 1
	castOpt := castOptionFor(t, e, kami)
	submitChoices(t, e, castOpt.Index)
	for i := 0; i < 20 && !e.G.Over && len(e.G.Stack) > 0; i++ {
		if d := e.Pending(); d != nil && d.Kind == decision.KTriggerOrder {
			submitChoices(t, e, 0, 1)
			continue
		}
		passUntilStackEmpty(t, e, 1)
	}
	if got := e.G.Obj(kami).Counter("P1P1"); got != 4 {
		t.Fatalf("Kami entered with %d P1P1, want 4 (two puts of 1 + Scales's 1; Kami's own line excluded, CR 614.12)", got)
	}
	replayCheck(t, e, cfg)
}
