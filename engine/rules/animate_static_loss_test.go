package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	animStaticBear = "Name:Animate Static Bear\nManaCost:1\nTypes:Creature Bear\nPT:1/4\nOracle:x\n"
	// animStaticWand gives a target creature the static ability of base power 5.
	animStaticWand = "Name:Animate Static Wand\nManaCost:1\nTypes:Artifact\n" +
		"A:AB$ Animate | Cost$ 0 | ValidTgts$ Creature | Duration$ Permanent | staticAbilities$ Five | SpellDescription$ x\n" +
		"SVar:Five:Mode$ Continuous | AffectedDefined$ Self | SetPower$ 5 | Description$ Its base power is 5.\nOracle:x\n"
	animStaticLoss = "Name:Animate Static Loss\nManaCost:1\nTypes:Enchantment Aura\nK:Enchant:Creature\n" +
		"S:Mode$ Continuous | AffectedDefined$ Enchanted | Affected$ Creature | RemoveAllAbilities$ True | Description$ x\nOracle:x\n"
)

func animStaticGrant(t *testing.T, e *Engine, wand, bear state.ObjID) {
	t.Helper()
	ppPriority(t, e)
	ppChoose(t, e, func(o decision.Option) bool { return o.Kind == "ability" && o.Obj == wand })
	answerAsk(t, e, optionIdxForObj(t, e.Pending(), bear))
	drainQueuedTriggers(t, e)
}

// A static ability a creature gained is an ability it can lose (CR 613.1f): a loss
// newer than the grant takes the base power with it, and layer 6's timestamp order
// leaves a grant newer than the loss standing.
func TestAnimateGrantedStaticFollowsAbilityLoss(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T) (e *Engine, bear, wand state.ObjID) {
		e = handEngine(t, card(t, animStaticBear), card(t, animStaticWand), card(t, animStaticLoss))
		hand := e.G.Zone(state.ZHand, 0)
		bear, wand = hand[0], hand[1]
		placeOnBattlefield(t, e, bear)
		placeOnBattlefield(t, e, wand)
		return e, bear, wand
	}
	t.Run("loss after the grant", func(t *testing.T) {
		e, bear, wand := setup(t)
		animStaticGrant(t, e, wand, bear)
		if got := e.Power(bear); got != 5 {
			t.Fatalf("granted power %d, want 5", got)
		}
		lossAttach(t, e, "Animate Static Loss", 0, bear)
		if got := e.Power(bear); got != 1 {
			t.Fatalf("power after the loss %d, want the printed 1", got)
		}
	})
	t.Run("grant after the loss", func(t *testing.T) {
		e, bear, wand := setup(t)
		lossAttach(t, e, "Animate Static Loss", 0, bear)
		animStaticGrant(t, e, wand, bear)
		if got := e.Power(bear); got != 5 {
			t.Fatalf("power after the later grant %d, want 5", got)
		}
	})
}

// Druid Class spells the same grant as a characteristic-defining static with
// EffectZone$ All. A gained ability defines nothing in the card's text, so it is a set
// like any other (CR 604.3), and a later loss ends it.
func TestAnimateGrantedStaticCDASpelling(t *testing.T) {
	t.Parallel()
	wand := "Name:Animate Static Rod\nManaCost:1\nTypes:Artifact\n" +
		"A:AB$ Animate | Cost$ 0 | ValidTgts$ Creature | Duration$ Permanent | staticAbilities$ Five | SpellDescription$ x\n" +
		"SVar:Five:Mode$ Continuous | EffectZone$ All | CharacteristicDefining$ True | SetPower$ 5 | SetToughness$ 5 | Description$ x\nOracle:x\n"
	e := handEngine(t, card(t, animStaticBear), card(t, wand), card(t, animStaticLoss))
	hand := e.G.Zone(state.ZHand, 0)
	bear, rod := hand[0], hand[1]
	placeOnBattlefield(t, e, bear)
	placeOnBattlefield(t, e, rod)
	animStaticGrant(t, e, rod, bear)
	if p, c := e.Power(bear), e.Toughness(bear); p != 5 || c != 5 {
		t.Fatalf("granted %d/%d, want 5/5", p, c)
	}
	lossAttach(t, e, "Animate Static Loss", 0, bear)
	if p, c := e.Power(bear), e.Toughness(bear); p != 1 || c != 4 {
		t.Fatalf("after the loss %d/%d, want the printed 1/4", p, c)
	}
}
