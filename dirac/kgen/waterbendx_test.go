package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// kwaterbendxScript is Katara, Water Tribe's Hope's waterbend ability on a
// bare creature, its cost and the line below it swapped in per case.
func kwaterbendxScript(cost, extra string) string {
	return "Name:Waterbend X Probe\nManaCost:2 W U U\nTypes:Creature Human\nPT:3/3\n" +
		"A:AB$ AnimateAll | Cost$ " + cost + " | XMin$ 1 | ValidCards$ Creature.YouCtrl | PlayerTurn$ True | Power$ X | Toughness$ X | SpellDescription$ X/X.\n" +
		"SVar:X:Count$xPaid\n" + extra
}

// TestWaterbendXLowersWithXMin: an activated ability's own Waterbend<X>
// (Katara, Water Tribe's Hope) lowers to CostWaterbend with N 0 and the one
// {X}, floored by XMin$ 1; beside printed mana it refuses, and the RaiseCost
// spell form stays refused.
func TestWaterbendXLowersWithXMin(t *testing.T) {
	d, rs := kwardpowBuild(t, "Waterbend X Probe", kwaterbendxScript("Waterbend<X>", ""))
	if all := kpaylifexReasons(rs); all != "" {
		t.Fatalf("Waterbend<X> refused: %q", all)
	}
	if len(d.Abilities) != 1 {
		t.Fatalf("abilities = %d, want 1", len(d.Abilities))
	}
	c := d.Abilities[0].Cost
	if c.Flags&kop.CostWaterbend == 0 || c.N != 0 || c.Mana.X != 1 || c.Mana.Generic != 0 || c.XMin != 1 {
		t.Fatalf("cost = %+v, want CostWaterbend, N 0, {X} alone, XMin 1", c)
	}

	_, rs = kwardpowBuild(t, "Waterbend X Probe", kwaterbendxScript("2 Waterbend<X>", ""))
	if all := kpaylifexReasons(rs); !strings.Contains(all, "Waterbend<X> beside printed mana") {
		t.Errorf("2 Waterbend<X>: refusals %q, want the beside-printed-mana refusal", all)
	}

	_, rs = kwardpowBuild(t, "Waterbend X Probe", kwaterbendxScript("Waterbend<1> Waterbend<X>", ""))
	if all := kpaylifexReasons(rs); !strings.Contains(all, "a second waterbend part") {
		t.Errorf("Waterbend<1> Waterbend<X>: refusals %q, want the second-part refusal", all)
	}

	spell := "Name:Waterbend X Spell Probe\nManaCost:U\nTypes:Instant\n" +
		"S:Mode$ RaiseCost | ValidCard$ Card.Self | Activator$ You | Type$ Spell | Cost$ Waterbend<X> | EffectZone$ All | Description$ Waterbend {X}.\n" +
		"A:SP$ Draw | Defined$ You | NumCards$ 1 | SpellDescription$ Draw.\n"
	_, rs = kwardpowBuild(t, "Waterbend X Spell Probe", spell)
	if all := kpaylifexReasons(rs); !strings.Contains(all, "spell waterbend") || !strings.Contains(all, "a variable amount is not modelled") {
		t.Errorf("RaiseCost Waterbend<X>: refusals %q, want the spell waterbend variable-amount refusal", all)
	}
}
