package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// A mana activation made from the CR 601.2g unless-cost mana window, inside a
// tape run's resolution, whose ProduceMana event meets a choice-valued colour
// replacement (ReplaceType$ Any): the colour ask is served from the tape and
// the converted mana pays the cost. The ask used to reach the engine's ask
// choke point as an unserved engine ask (a kernel Divergence: hunt cmdr4 seed
// 88680, "replacement/replacement Choose the colour of the replacement mana").
func TestUnlessWindowManaColorReplacementIsServed(t *testing.T) {
	const gain = "\nSVar:Gain:DB$ GainLife | LifeAmount$ 2 | Defined$ You"
	const pulse = "Name:Window Pulse\nTypes:Enchantment\n" +
		"R:Event$ ProduceMana | ValidCard$ Land.Basic+YouCtrl | ReplaceWith$ AnyHue | ActiveZones$ Battlefield | " +
		"Description$ Fixture: a basic land you control tapped for mana yields one colour of its controller's choice.\n" +
		"SVar:AnyHue:DB$ ReplaceMana | ReplaceType$ Any\nOracle:x\n"
	name := "Window Pulse Tithe"
	src := tapeUnlessSorcery(name,
		"A:SP$ GainLife | LifeAmount$ 3 | UnlessCost$ 1 | UnlessPayer$ You | UnlessSwitched$ True | SubAbility$ Gain"+gain)
	asked := 0
	var life int32
	e, _ := runServedScenario(t, 2, 13200, 3, func(t *testing.T, e *Engine) {
		moveByName(t, e, 0, "Window Pulse", state.ZBattlefield)
		life = e.G.Players[0].Life
		pick := tapeUnlessPick(true)
		tapeUnlessScenario(name, "B", 2, func(d *decision.Decision) []int {
			if d.Kind == decision.KReplacement {
				asked++
			}
			return pick(d)
		})(t, e)
	}, src, pulse)
	if asked == 0 {
		t.Fatal("precondition: the colour replacement ask was never posed")
	}
	if got := e.G.Players[0].Life - life; got != 5 {
		t.Fatalf("life change %d, want 5 (the replaced mana paid the unless cost)", got)
	}
}

// The same colour ask when the source offers a choice of mana abilities (a
// painland): the window's activation opens an off-stack mana frame, and the
// replacement ask raised inside it is served from the tape too.
func TestUnlessWindowMultiAbilityManaColorReplacementIsServed(t *testing.T) {
	const gain = "\nSVar:Gain:DB$ GainLife | LifeAmount$ 2 | Defined$ You"
	const pulse = "Name:Window Pulse\nTypes:Enchantment\n" +
		"R:Event$ ProduceMana | ValidCard$ Land | ReplaceWith$ AnyHue | ActiveZones$ Battlefield | " +
		"Description$ Fixture: a land tapped for mana yields one colour of its controller's choice.\n" +
		"SVar:AnyHue:DB$ ReplaceMana | ReplaceType$ Any\nOracle:x\n"
	const wastes = "Name:Window Wastes\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Add one colourless.\n" +
		"A:AB$ Mana | Cost$ T | Produced$ Combo W U | SubAbility$ Sting | SpellDescription$ Add white or blue; it hurts.\n" +
		"SVar:Sting:DB$ DealDamage | Defined$ You | NumDmg$ 1\nOracle:x\n"
	name := "Window Pulse Tithe"
	src := tapeUnlessSorcery(name,
		"A:SP$ GainLife | LifeAmount$ 3 | UnlessCost$ 1 | UnlessPayer$ You | UnlessSwitched$ True | SubAbility$ Gain"+gain)
	asked, abilityPicks := 0, 0
	var life int32
	e, _ := runServedScenario(t, 2, 13201, 3, func(t *testing.T, e *Engine) {
		moveByName(t, e, 0, "Window Pulse", state.ZBattlefield)
		moveByName(t, e, 0, "Window Wastes", state.ZBattlefield)
		life = e.G.Players[0].Life
		pick := tapeUnlessPick(true)
		tapeUnlessScenario(name, "B", 0, func(d *decision.Decision) []int {
			if d.Kind == decision.KReplacement {
				asked++
			}
			if d.Kind == decision.KChoose && d.ResumeKind != unlessManaKind {
				abilityPicks++
			}
			return pick(d)
		})(t, e)
	}, src, pulse, wastes)
	if asked == 0 || abilityPicks == 0 {
		t.Fatalf("precondition: replacement asks %d, ability picks %d; both must be posed", asked, abilityPicks)
	}
	if got := e.G.Players[0].Life - life; got != 5 {
		t.Fatalf("life change %d, want 5 (the replaced mana paid the unless cost)", got)
	}
}
