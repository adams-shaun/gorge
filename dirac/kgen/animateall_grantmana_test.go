package kgen

import (
	"strings"
	"testing"
)

// The AnimateAll Abilities$ grant (gorge parseAnimateGrant's ag.abilities):
// Rain of Filth's "lands you control gain 'Sacrifice this land: Add {B}'"
// and Song of Freyalise's saga chapters (DB$ AnimateAll | Abilities$ |
// Duration$ UntilYourNextTurn) lower to one OverrideAll whose record's
// Grant names the granted latent mana ability of the casting card. A
// refusal probe names the shapes the pair cannot carry.

func lowerSynthOverrides(t *testing.T, src string) []lInstr {
	t.Helper()
	lc := lowerSynth(t, src)
	var out []lInstr
	for _, ab := range lc.Abilities {
		for _, in := range ab.Prog {
			if in.Op == "OverrideAll" {
				out = append(out, in)
			}
		}
	}
	return out
}

// TestAnimateAllGrantManaLow tests the Rain of Filth spelling: the sweep's
// record carries the granted sacrifice-only mana ability of the card.
func TestAnimateAllGrantManaLow(t *testing.T) {
	const src = "Name:Rain of Filth Probe\nManaCost:B\nTypes:Instant\n" +
		"A:SP$ AnimateAll | ValidCards$ Land.YouCtrl | Abilities$ ABMana | SpellDescription$ x.\n" +
		"SVar:ABMana:AB$ Mana | Cost$ Sac<1/CARDNAME> | Produced$ B | SpellDescription$ Add {B}.\n"
	lc := lowerSynth(t, src)
	if r := lc.Refusals; len(r) != 0 {
		t.Fatalf("the probe refuses: %q", keclReason(r))
	}
	ovs := lowerSynthOverrides(t, src)
	if len(ovs) != 1 {
		t.Fatalf("OverrideAll instructions %d, want 1", len(ovs))
	}
	c := ovs[0].C
	o := lc.overrides[c]
	if len(o.GrantAbility) != 1 {
		t.Fatalf("record %d grants %v, want one ability", c, o.GrantAbility)
	}
	ab := lc.Abilities[o.GrantAbility[0]]
	if !ab.Latent || ab.Kind != akMana {
		t.Fatalf("granted ability %d: kind %v latent %v, want a latent mana ability", o.GrantAbility[0], ab.Kind, ab.Latent)
	}
}

// TestAnimateAllGrantManaUntilNextTurn tests Song of Freyalise's chapter
// spelling: a DB$ AnimateAll inside the saga's chapter trigger lowers its
// sweep with the UntilYourNextTurn lifetime and the granted {T} any-colour
// ability.
func TestAnimateAllGrantManaUntilNextTurn(t *testing.T) {
	const src = "Name:Song of Freyalise Probe\nManaCost:1 G\nTypes:Enchantment Saga\n" +
		"K:Chapter:2:DBAnimateAll,DBPutCounterAll\n" +
		"SVar:DBAnimateAll:DB$ AnimateAll | ValidCards$ Creature.YouCtrl | Abilities$ AnyMana | Duration$ UntilYourNextTurn | SpellDescription$ x.\n" +
		"SVar:AnyMana:AB$ Mana | Cost$ T | Produced$ Any | Amount$ 1 | SpellDescription$ Add one mana of any color.\n" +
		"SVar:DBPutCounterAll:DB$ PutCounterAll | ValidCards$ Creature.YouCtrl | CounterType$ P1P1 | CounterNum$ 1 | SpellDescription$ y.\n"
	lc := lowerSynth(t, src)
	if r := lc.Refusals; len(r) != 0 {
		t.Fatalf("the probe refuses: %q", keclReason(r))
	}
	ovs := lowerSynthOverrides(t, src)
	if len(ovs) != 1 {
		t.Fatalf("OverrideAll instructions %d, want 1", len(ovs))
	}
	if !strings.Contains(ovs[0].CS, "UntilYourNextTurn") {
		t.Fatalf("OverrideAll CS %q, want the UntilYourNextTurn lifetime", ovs[0].CS)
	}
	o := lc.overrides[ovs[0].C]
	if len(o.GrantAbility) != 1 {
		t.Fatalf("record %d grants %v, want one ability", ovs[0].C, o.GrantAbility)
	}
	ab := lc.Abilities[o.GrantAbility[0]]
	if !ab.Latent || ab.Kind != akMana {
		t.Fatalf("granted ability %d: kind %v latent %v, want a latent mana ability", o.GrantAbility[0], ab.Kind, ab.Latent)
	}
}

// TestAnimateAllGrantManaRefused: the shapes the pair cannot carry refuse by
// name; a single-object Animate keeps refusing Abilities$ as unread.
func TestAnimateAllGrantManaRefused(t *testing.T) {
	const sv = "SVar:ABMana:AB$ Mana | Cost$ Sac<1/CARDNAME> | Produced$ B | SpellDescription$ Add {B}.\n"
	for _, tc := range []struct{ name, body, want string }{
		{"three abilities", "A:SP$ AnimateAll | ValidCards$ Creature.YouCtrl | Abilities$ ABMana,ABMana,ABMana | SpellDescription$ x.\n", "more than two granted abilities"},
		{"with a loss", "A:SP$ AnimateAll | ValidCards$ Creature.YouCtrl | Abilities$ ABMana | RemoveAllAbilities$ True | SpellDescription$ x.\n", "beside RemoveAllAbilities$"},
		{"empty", "A:SP$ AnimateAll | ValidCards$ Creature.YouCtrl | Abilities$  | SpellDescription$ x.\n", "names no ability"},
		{"unknown name", "A:SP$ AnimateAll | ValidCards$ Creature.YouCtrl | Abilities$ NoSuch | SpellDescription$ x.\n", "granted ability"},
		{"single Animate", "A:SP$ Animate | Defined$ Self | Abilities$ ABMana | SpellDescription$ x.\n", "Animate Abilities$"},
	} {
		src := "Name:Refuse Probe\nManaCost:B\nTypes:Instant\n" + tc.body + sv
		lc := lowerSynth(t, src)
		rs := lc.Refusals
		if len(rs) == 0 || !strings.Contains(keclReason(rs), tc.want) {
			t.Errorf("%s: refusals %v, want one containing %q", tc.name, keclReason(rs), tc.want)
		}
	}
}
