package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// dbManaChooseBuild builds one inline script and returns its card, its
// refusals and the pool's expression table.
func dbManaChooseBuild(t *testing.T, name, src string) (kop.CardDef, []refusal, []kop.Expr) {
	t.Helper()
	card, diags := cards.ParseBytes("dbmana_choosecolor_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lw := lowerPool(p)
	_, bs, pt, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	return bs[0].def, lw.cards[0].Refusals, pt.exprs
}

const dbManaNyxLotus = "Name:Test Nyx Lotus\nManaCost:4\nTypes:Legendary Artifact\n" +
	"A:AB$ ChooseColor | Cost$ T | AILogic$ MostProminentComputerControls | AINoRecursiveCheck$ True | SubAbility$ ProdStep | SpellDescription$ Probe: ask a colour, add devotion mana of it.\n" +
	"SVar:ProdStep:DB$ Mana | Produced$ Chosen | Amount$ X | SubAbility$ ClearStep\n" +
	"SVar:ClearStep:DB$ Cleanup | ClearChosenColor$ True\n" +
	"SVar:X:Count$Devotion.Chosen\n"

const dbManaNykthos = "Name:Test Nykthos\nManaCost:no cost\nTypes:Legendary Land\n" +
	"A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Probe colourless.\n" +
	"A:AB$ ChooseColor | Cost$ 2 T | SubAbility$ ProdStep | AILogic$ MostProminentComputerControls | AINoRecursiveCheck$ True | SpellDescription$ Probe.\n" +
	"SVar:ProdStep:DB$ Mana | Produced$ Chosen | Amount$ X | SubAbility$ ClearStep\n" +
	"SVar:ClearStep:DB$ Cleanup | ClearChosenColor$ True\n" +
	"SVar:X:Count$Devotion.Chosen\n"

// TestDbManaChooseColorLowers: the chain AB$ ChooseColor -> DB$ Mana
// (Produced$ Chosen, Amount$ Count$Devotion.Chosen) -> DB$ Cleanup
// (ClearChosenColor$ True) is a CR 605.1a mana ability gorge resolves off the
// stack (lower_dbmanacontext.go); it lowers as ONE mana ability with the
// colour-asking flag, the devotion-of-chosen amount and no program (the kernel
// records and clears the colour itself, dirac/mana_askcolour.go). Every other
// shape of the chain refuses, naming the part.
func TestDbManaChooseColorLowers(t *testing.T) {
	askAb := func(d *kop.CardDef) (n int, ab *kop.Ability) {
		for i := range d.Abilities {
			if d.Abilities[i].Kind == kop.AbMana && d.Abilities[i].ManaAskColour {
				n++
				ab = &d.Abilities[i]
			}
		}
		return n, ab
	}
	for _, c := range []struct {
		name, src string
		mana      int
	}{
		{"Test Nyx Lotus", dbManaNyxLotus, 1},
	} {
		def, rs, exprs := dbManaChooseBuild(t, c.name, c.src)
		if len(rs) != 0 {
			t.Errorf("%s: refused %v", c.name, rs)
			continue
		}
		n, ab := askAb(&def)
		if n != 1 {
			t.Errorf("%s: %d colour-asking mana abilities, want 1", c.name, n)
			continue
		}
		total := 0
		for _, a := range def.Abilities {
			if a.Kind == kop.AbMana {
				total++
			}
		}
		if total != c.mana {
			t.Errorf("%s: %d mana abilities, want %d", c.name, total, c.mana)
		}
		if ab.AFlags != 0 || ab.ManaV.K != kop.VExpr || exprs[ab.ManaV.N].Kind != kop.ExDevotionChosen || ab.ManaColors != kop.ColAny || ab.ManaN != 1 {
			t.Errorf("%s: ManaV %+v flags %#x colours %#x N %d, want the devotion-of-chosen value, no flag, any colour", c.name, ab.ManaV, ab.AFlags, ab.ManaColors, ab.ManaN)
		}
		if len(ab.Prog) != 0 || ab.Cost.Flags&kop.CostTap == 0 {
			t.Errorf("%s: %+v, want no program, a {T} cost", c.name, ab)
		}
	}
	// every other shape of the chain is not matched and refuses where it
	// always did, naming the ChooseColor head
	for _, c := range []struct{ name, src string }{
		// Nykthos's "{2}, {T}" ask beside a plain first ability: the kernel prices it as a filter
		{"Nykthos", strings.Replace(dbManaNykthos, "Test Nykthos", "Test Nyx Lotus", 1)},
		{"a node after the cleanup", dbManaNyxLotus + "SVar:ClearStep:DB$ Cleanup | ClearChosenColor$ True | SubAbility$ DBMore\nSVar:DBMore:DB$ GainLife | LifeAmount$ 1\n"},
		{"a target", strings.Replace(dbManaNyxLotus, "Cost$ T |", "Cost$ T | ValidTgts$ Creature |", 1)},
		{"a loyalty marker", strings.Replace(dbManaNyxLotus, "Cost$ T |", "Cost$ T | Planeswalker$ True |", 1)},
		{"a mill cost", strings.Replace(dbManaNyxLotus, "Cost$ T |", "Cost$ T Mill<1> |", 1)},
		{"another amount", strings.Replace(dbManaNyxLotus, "Count$Devotion.Chosen", "Count$Devotion.Red", 1)},
		{"a constant amount", strings.Replace(dbManaNyxLotus, "Amount$ X", "Amount$ 2", 1)},
		{"a fixed produced colour", strings.Replace(dbManaNyxLotus, "Produced$ Chosen", "Produced$ R", 1)},
		{"an activation limit", strings.Replace(dbManaNyxLotus, "Cost$ T |", "Cost$ T | ActivationLimit$ 1 |", 1)},
		{"an unless cost", strings.Replace(dbManaNyxLotus, "Produced$ Chosen |", "Produced$ Chosen | UnlessCost$ 1 | UnlessPayer$ Player |", 1)},
		{"a cost beyond a fee and tap", strings.Replace(dbManaNyxLotus, "Cost$ T |", "Cost$ T PayLife<1> |", 1)},
		{"no tap", strings.Replace(dbManaNyxLotus, "Cost$ T |", "Cost$ 1 |", 1)},
		{"an animate rider", strings.Replace(dbManaNyxLotus, "SubAbility$ ClearStep\n", "SubAbility$ DBAnimate\nSVar:DBAnimate:DB$ Animate | Defined$ Self | Colors$ ChosenColor | SubAbility$ ClearStep\n", 1)},
	} {
		def, rs, _ := dbManaChooseBuild(t, "Test Nyx Lotus", c.src)
		var all []string
		for _, r := range rs {
			all = append(all, r.Reason)
		}
		if n, _ := askAb(&def); len(rs) == 0 || n != 0 || !strings.Contains(strings.Join(all, "; "), "ChooseColor") {
			t.Errorf("%s: %d asking abilities, refusals %q, want none and a ChooseColor refusal", c.name, n, all)
		}
	}
}
