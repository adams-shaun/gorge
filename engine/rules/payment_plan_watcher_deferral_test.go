package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/rules/pay"
)

// The census-gap second pass funds only a source every deferring object merely
// watches (paymentPlanSourceInterference's watcher verdict): a trigger queued
// after the cast (Manabarbs) is one, a ProduceMana replacement (Mana
// Reflection) and an unfoldable Static$ True triggered mana ability act on the
// payment and are not.
func TestPaymentPlanWatcherDeferralClass(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		onBoard func(t *testing.T, e *Engine)
		watcher bool
	}{
		{"manabarbs trigger", func(t *testing.T, e *Engine) { onBoardCard(t, e, 1, corpusCard(t, "Manabarbs")) }, true},
		{"mana reflection replacement", func(t *testing.T, e *Engine) { onBoardCard(t, e, 0, corpusCard(t, "Mana Reflection")) }, false},
		{"unfoldable static trigger", func(t *testing.T, e *Engine) {
			onBoard(t, e, 1, "Name:Flare Test\nTypes:Enchantment\nT:Mode$ TapsForMana | ValidCard$ Land | Execute$ OnMana | TriggerZones$ Battlefield | Static$ True | TriggerDescription$ x\nSVar:OnMana:DB$ ManaReflected | ColorOrType$ Type | ReflectProperty$ Produced | Defined$ TriggeredActivator | SubAbility$ OnGain\nSVar:OnGain:DB$ GainLife | LifeAmount$ 1 | Defined$ You\nOracle:x\n")
		}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, _, _ := newFixtureDeck(t, 9960, interferenceBlueInstant)
			island := onBoard(t, e, 0, interferenceIsland)
			c.onBoard(t, e)
			ma := e.G.Obj(island).Face().ManaAbilities()
			if len(ma) == 0 {
				// basic lands carry the intrinsic ability only through the census
				units := e.paymentPlanManaUnits(0)
				for _, u := range units {
					if u.ID == island && len(u.Alts) > 0 {
						ma = append(ma, u.Alts[0].Ma)
					}
				}
			}
			if len(ma) == 0 || ma[0] == nil {
				t.Fatal("precondition: the Island has no mana ability to classify")
			}
			tier, _, detail, _, watcher := asEval(e).SourceInterference(island, ma[0], "")
			if tier != pay.TierDeferred {
				t.Fatalf("precondition: Island tier = %d %q, want deferred", tier, detail)
			}
			if watcher != c.watcher {
				t.Fatalf("watcher verdict = %v (%q), want %v", watcher, detail, c.watcher)
			}
		})
	}
}
