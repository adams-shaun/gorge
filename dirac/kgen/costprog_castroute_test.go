package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// costProgCastBuild lowers and builds a card list and returns the lowered
// cards and their built defs.
func costProgCastBuild(t *testing.T, names []string) ([]*lCard, []built) {
	t.Helper()
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, ratchetCards(t))
	if err != nil {
		t.Fatal(err)
	}
	all, bs, _, err := buildPool(lowerPool(p))
	if err != nil {
		t.Fatal(err)
	}
	return all, bs
}

// spellAbility is the built card's AbSpell ability, or nil.
func spellAbility(d kop.CardDef) *kop.Ability {
	for i := range d.Abilities {
		if d.Abilities[i].Kind == kop.AbSpell {
			return &d.Abilities[i]
		}
	}
	return nil
}

// costProgCastRouteReason joins a lowered card's refusal reasons.
func costProgCastRouteReason(c *lCard) string {
	var out []string
	for _, r := range c.Refusals {
		out = append(out, r.Reason)
	}
	return strings.Join(out, " | ")
}

// TestCostProgCastRoute: the cast-route shapes the per-shape kop.Cost fields
// can express lower through them; the shapes that need a kernel pick the
// kernel does not pose stay refused.
//
//   - a kicker's single sacrifice, life or discard part widens the existing
//     KickerCost field;
//   - a RaiseCost Cost$ of pure mana widens the IncreaseCost field, including
//     its coloured pips (Jade Leech);
//   - a self RaiseCost Cost$ of a pick part rides the spell's own additional
//     cost field (Grafted Identity);
//   - an alternative cost's mana beside a pick part rides the alternative's
//     pick stages (a Borderpost, Sephara, Force of Virtue);
//   - an AlternateAdditionalCost or morph Reveal<1/Spec> stays refused: gorge
//     asks "Choose cards to reveal" over two or more candidates, and the
//     kernel has no such pick on a cast or a turn-up.
func TestCostProgCastRoute(t *testing.T) {
	all, bs := costProgCastBuild(t, []string{
		"Daring Buccaneer", "Dragon's Eye Savants", "Bog Down",
		"Phyrexian Scuta", "Jade Leech", "Grafted Identity",
		"Fieldmist Borderpost", "Sephara, Sky's Blade", "Force of Virtue",
	})
	byName := map[string]int{}
	for i, c := range all {
		byName[c.Name] = i
	}
	index := func(name string) (int, built) {
		t.Helper()
		i, ok := byName[name]
		if !ok {
			t.Fatalf("%s: not lowered", name)
		}
		return i, bs[i]
	}

	t.Run("reveal routes stay refused", func(t *testing.T) {
		for _, c := range []struct{ name, reason string }{
			{"Daring Buccaneer", "alternate additional cost"},
			{"Dragon's Eye Savants", "morph cost"},
		} {
			i, _ := index(c.name)
			if got := costProgCastRouteReason(all[i]); !strings.Contains(got, c.reason) {
				t.Errorf("%s: refusals %q, want one naming %q (no kernel reveal pick)", c.name, got, c.reason)
			}
		}
	})

	t.Run("kicker sac two", func(t *testing.T) {
		i, b := index("Bog Down")
		if got := costProgCastRouteReason(all[i]); strings.Contains(got, "kicker cost") {
			t.Fatalf("refused: %s", got)
		}
		c := b.def.KickerCost
		if c.Flags&kop.CostSac == 0 || c.N != 2 || c.Filter == 0 {
			t.Fatalf("KickerCost %+v, want a sacrifice of two lands", c)
		}
	})

	t.Run("kicker pay life", func(t *testing.T) {
		i, b := index("Phyrexian Scuta")
		if got := costProgCastRouteReason(all[i]); strings.Contains(got, "kicker cost") {
			t.Fatalf("refused: %s", got)
		}
		c := b.def.KickerCost
		if c.Flags&kop.CostPayLife == 0 || c.N != 3 {
			t.Fatalf("KickerCost %+v, want pay 3 life", c)
		}
	})

	t.Run("raise cost colored mana", func(t *testing.T) {
		i, b := index("Jade Leech")
		if got := costProgCastRouteReason(all[i]); strings.Contains(got, "RaiseCost Cost$") {
			t.Fatalf("refused: %s", got)
		}
		found := false
		for _, a := range b.def.Abilities {
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatIncreaseCost {
				continue
			}
			if a.Static.B == 0 || a.Static.StFlags&kop.StReduceColor == 0 {
				continue
			}
			found = true
			want := int16(1) << (kop.ReduceColorBits * int(kop.G))
			if a.Static.B != want {
				t.Fatalf("raise pips %#x, want %#x", a.Static.B, want)
			}
		}
		if !found {
			t.Fatal("no IncreaseCost with green pips")
		}
	})

	t.Run("alternative cost mana plus pick", func(t *testing.T) {
		for _, name := range []string{"Fieldmist Borderpost", "Sephara, Sky's Blade", "Force of Virtue"} {
			i, _ := index(name)
			if got := costProgCastRouteReason(all[i]); strings.Contains(got, "alternative cost with a non-mana part") {
				t.Errorf("%s: refused: %s", name, got)
			}
		}
	})

	t.Run("self raise pick", func(t *testing.T) {
		i, b := index("Grafted Identity")
		if got := costProgCastRouteReason(all[i]); strings.Contains(got, "RaiseCost Cost$") {
			t.Fatalf("refused: %s", got)
		}
		if b.def.SpellCost.Flags&kop.CostSac == 0 || b.def.SpellCost.N != 1 {
			t.Fatalf("SpellCost %+v, want sacrifice one creature", b.def.SpellCost)
		}
	})
}
