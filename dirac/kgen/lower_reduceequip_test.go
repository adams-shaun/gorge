package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestReduceCostEquipLowers: ValidSpell$ Activated.Equip lowers as the
// equip arm of kop.StatReduceAbility (B bit kop.ReduceAbEquip), the
// target-gated shape ("equip abilities you activate that target this
// creature": Cloud, Planet's Champion; Dwarven Mauler) with its ValidTarget$
// in Filter2 under StTargetsFilter2, the source-filtered one (Auriok
// Steelshaper) without.
func TestReduceCostEquipLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, c := range []struct {
		name   string
		amount int
		self   bool // ValidTarget$ Card.Self
	}{
		{"Cloud, Planet's Champion", 2, true},
		{"Dwarven Mauler", 2, true},
		{"Auriok Steelshaper", 1, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			card, ok := reg.Lookup(c.name)
			if !ok || card == nil {
				t.Fatalf("%s not in the corpus", c.name)
			}
			p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
			lc := lowerCard(p, card.Faces[0].Name, card, false)
			for _, rf := range lc.Refusals {
				t.Errorf("refused: %s", rf.Reason)
			}
			var red []lStatic
			for _, st := range lc.Statics {
				if st.Kind == "ReduceCost" {
					red = append(red, st)
				}
			}
			if len(red) != 1 {
				t.Fatalf("%d ReduceCost statics, want 1", len(red))
			}
			st := red[0]
			if st.AbilityRed != 5 || st.Amount.Kind != vConst || st.Amount.N != c.amount {
				t.Errorf("AbilityRed %d Amount %+v, want 5 and const %d", st.AbilityRed, st.Amount, c.amount)
			}
			if st.F == nil {
				t.Error("no source filter (a missing ValidCard$ is any permanent)")
			}
			if c.self {
				if st.F2 == nil || len(st.F2.Clauses) == 0 || st.F2.Clauses[0].Flags&fSelf == 0 {
					t.Errorf("F2 %+v, want the Card.Self target filter", st.F2)
				}
			} else if st.F2 != nil {
				t.Errorf("F2 %+v, want none", st.F2)
			}
			_, all, pt, err := buildPool(&lowered{cards: []*lCard{lc}})
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			var ks []kop.Static
			for _, ab := range all[0].def.Abilities {
				if ab.Kind == kop.AbStatic && ab.Static.Kind == kop.StatReduceAbility {
					ks = append(ks, ab.Static)
				}
			}
			if len(ks) != 1 {
				t.Fatalf("%d StatReduceAbility statics built, want 1", len(ks))
			}
			k := ks[0]
			if k.B&kop.ReduceAbEquip == 0 || k.Filter == 0 {
				t.Errorf("built B %#x Filter %d, want the equip bit and a source filter", k.B, k.Filter)
			}
			if c.self {
				if k.Filter2 == 0 || k.StFlags&kop.StTargetsFilter2 == 0 || pt.filters[k.Filter2].Flags&kop.FSelf == 0 {
					t.Errorf("built Filter2 %d StFlags %#x, want a FSelf target gate", k.Filter2, k.StFlags)
				}
			} else if k.Filter2 != 0 || k.StFlags&kop.StTargetsFilter2 != 0 {
				t.Errorf("built Filter2 %d StFlags %#x, want no target gate", k.Filter2, k.StFlags)
			}
		})
	}
	t.Run("refuses a non-Equip activation class by name", func(t *testing.T) {
		src := strings.Join([]string{"Name:Probe", "ManaCost:1 W", "Types:Creature Bear", "PT:2/2",
			"S:Mode$ ReduceCost | ValidCard$ Card | ValidSpell$ Activated.Cycling | Activator$ You | Amount$ 1 | Description$ x"}, "\n")
		card, diags := cards.ParseBytes("Probe.txt", []byte(src))
		if card == nil || len(diags) != 0 {
			t.Fatalf("parse: %v", diags)
		}
		if d := card.Link(); len(d) != 0 {
			t.Fatalf("link: %v", d)
		}
		p := &pool{byName: map[string]*cards.Card{"Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
		lc := lowerCard(p, "Probe", card, false)
		var out []string
		for _, r := range lc.Refusals {
			out = append(out, r.Reason)
		}
		if got := strings.Join(out, "; "); !strings.Contains(got, "ReduceCost ValidSpell") || !strings.Contains(got, "Activated.Cycling") {
			t.Errorf("refusals %q, want the ValidSpell$ Activated.Cycling class named", got)
		}
	})
}

// TestReduceCostEquipNotInAbReduceGuard: an equip-only reducer never lowers a
// mana ability, so the matchup guard does not list it beside a mana-costed
// mana ability; a general ability reducer still is.
func TestReduceCostEquipNotInAbReduceGuard(t *testing.T) {
	red := built{def: kop.CardDef{Name: "Reducer", Abilities: []kop.Ability{
		{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatReduceAbility, A: 2, B: kop.ReduceAbEquip}}}}}
	mana := built{def: kop.CardDef{Name: "Paid Mana", Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: kop.Cost{Mana: kop.ManaCost{Generic: 1}}}}}}
	bs := []built{red, mana}
	filters := []kop.Filter{{}}
	if _, _, abRed, _, _, _, _, _ := guardLists(bs, filters); len(abRed) != 0 {
		t.Errorf("equip-only reducer listed as an ability reducer: %v", abRed)
	}
	bs[0].def.Abilities[0].Static.B = 0
	if _, _, abRed, _, _, _, _, _ := guardLists(bs, filters); len(abRed) != 1 || abRed[0] != 0 {
		t.Errorf("general reducer: abRed %v, want [0]", abRed)
	}
}
