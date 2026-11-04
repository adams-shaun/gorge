package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Make It So's first mode: "Untap target permanent you control. Target player
// draws two cards." The second target is the mode's sub-ability's own.
const charmSubTargetSpell = "Name:Order Given\nManaCost:U\nTypes:Sorcery\n" +
	"A:SP$ Charm | Choices$ DBUntap,DBOneLife\n" +
	"SVar:DBUntap:DB$ Untap | ValidTgts$ Permanent.YouCtrl | SubAbility$ DBTwoCards | SpellDescription$ x\n" +
	"SVar:DBTwoCards:DB$ Draw | ValidTgts$ Player | NumCards$ 2 | SpellDescription$ x\n" +
	"SVar:DBOneLife:DB$ GainLife | LifeAmount$ 1 | Defined$ You | SpellDescription$ x\nOracle:x\n"

// TestCharmModeSubTargetKeepsTheSpellAlive: the chosen mode's permanent target
// leaves before the spell resolves, but its sub-ability's player target is
// still legal, so the spell is not countered (CR 608.2b): the player draws
// two cards.
func TestCharmModeSubTargetKeepsTheSpellAlive(t *testing.T) {
	t.Parallel()
	e, cfg, spell := newFixtureDeck(t, 811, charmSubTargetSpell, beamBear)
	bear := putCreature(t, e, 0, beamBear)
	addMana(t, e, 0, "U")
	e.Advance()
	var cast decision.Option
	for _, o := range castOptions(t, e) {
		if o.Obj == spell && o.Kind == "cast" {
			cast = o
		}
	}
	if cast.Obj == 0 {
		t.Fatal("the charm is not offered")
	}
	submitChoices(t, e, cast.Index)
	for guard := 0; guard < 8; guard++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		pick := -1
		for _, o := range d.Options {
			switch {
			case d.Kind == decision.KModes && o.Mode == "DBUntap" || o.Label == "Untap target permanent you control.":
				pick = o.Index
			case o.Obj == bear && o.Obj != 0:
				pick = o.Index
			case o.Kind == "player" && o.Player == 0:
				pick = o.Index
			}
		}
		if pick < 0 {
			pick = d.Options[0].Index
		}
		submitChoices(t, e, pick)
	}
	if z := e.G.Obj(spell).Zone; z != state.ZStack {
		t.Fatalf("the charm is in %s, want the stack", z)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZExile})
	hand := len(e.G.Zone(state.ZHand, 0))
	resolveStack(t, e)
	if got := len(e.G.Zone(state.ZHand, 0)); got != hand+2 {
		t.Fatalf("hand %d, want %d: the sub-ability's player target is still legal", got, hand+2)
	}
	replayCheck(t, e, cfg)
}
