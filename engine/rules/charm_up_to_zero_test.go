package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// upToZeroCharmSrc is an inline Charm "choose up to X" (MinCharmNum$ 0)
// whose count reads 0 on the plain cast: the count is Count$Kicked.2.0 with
// no kicker paid.
const upToZeroCharmSrc = "Name:Up To Zero Probe\nManaCost:R\nTypes:Instant\n" +
	"A:SP$ Charm | MinCharmNum$ 0 | CharmNum$ X | Choices$ DBOne,DBTwo\n" +
	"SVar:DBOne:DB$ GainLife | Defined$ You | LifeAmount$ 1 | SpellDescription$ Gain 1 life.\n" +
	"SVar:DBTwo:DB$ GainLife | Defined$ You | LifeAmount$ 2 | SpellDescription$ Gain 2 life.\n" +
	"SVar:X:Count$Kicked.2.0\nOracle:x\n"

// TestCharmUpToZeroChoosesNoMode pins CR 700.2a: "choose up to X" with X
// at 0 announces no mode, so no mode ask is posed and the spell is on the
// stack with the empty announcement.
func TestCharmUpToZeroChoosesNoMode(t *testing.T) {
	t.Parallel()
	e, _, id := newFixtureDeck(t, 31, upToZeroCharmSrc)
	addMana(t, e, 0, "R")
	idx := -1
	for _, o := range castOptions(t, e) {
		if o.Obj == id {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option: %+v", castOptions(t, e))
	}
	submitChoices(t, e, idx)
	if d := e.Pending(); d != nil && d.Kind == decision.KModes {
		t.Fatalf("a count of 0 posed a mode ask: %+v", d)
	}
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZStack || o.ChosenModes == nil || len(o.ChosenModes) != 0 {
		t.Fatalf("want the spell on the stack with the empty announcement, got %+v", o)
	}
}
