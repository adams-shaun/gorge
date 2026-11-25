package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLowerNonTapCostManaAbilities: the mana abilities whose cost goes beyond
// {T} (a sacrifice of another permanent, a tap of an untapped permanent with
// no {T}) that the kernel's tap-another search activates (kernel
// mana_tapanother.go), and the no-{T} counter removal shape the kernel's
// counter gap activates repeatedly while the counters last (kernel
// mana_countercost.go, kop.CtrRemoveManaAb), lower with no refusal and every
// mana ability honoured (kop.ManaAbilityHonoured); the rest of the class stays
// refused with its measured reason (lockstep or the kernel's mana walk does not
// yet match gorge for it), so no card of it is lowered as an approximation.
func TestLowerNonTapCostManaAbilities(t *testing.T) {
	lowered := map[string]string{
		"Phyrexian Altar":             "Sacrifice a creature: Add one mana of any color (kop.SacAnyManaAb)",
		"Gilded Goose":                "{T}, Sacrifice a Food: Add one mana of any color (kop.SacTapAnyManaAb)",
		"Phyrexian Tower":             "a second {T}, Sacrifice a creature: Add {B}{B} (kop.SecondSacTapManaAb)",
		"Urza, Lord High Artificer":   "Tap an untapped artifact you control: Add {U} (kop.TapXManaOne)",
		"Meria, Scholar of Antiquity": "Tap an untapped nontoken artifact you control: Add {G} (kop.TapXManaOne)",
		"Seton, Krosan Protector":     "Tap an untapped Druid you control: Add {G}, a Druid itself (kop.TapXManaOne)",
		"Relic of Legends":            "a second Tap an untapped legendary creature you control: Add one mana of any color (kop.SecondTapXOneManaAb)",
		"Tarnished Citadel":           "{T}: any colour, 3 damage to you (f907ff6cf7; dirac/oracle/pain_anycolour_lockstep_test.go)",
		"Grand Coliseum":              "{T}: any colour, 1 damage to you (f907ff6cf7; dirac/oracle/pain_anycolour_lockstep_test.go)",
		"Pentad Prism":                "Remove a charge counter: Add one mana of any color (kop.CtrRemoveManaAb; dirac/oracle/nontapcost_mana_lockstep_test.go)",
		"Crystalline Crawler":         "Remove a +1/+1 counter: Add one mana of any color (kop.CtrRemoveManaAb; the counter gap kind)",
		"Runaway Steam-Kin":           "Remove three +1/+1 counters: Add {R}{R}{R} (kop.CtrRemoveManaAb; dirac/oracle/nontapcost_mana_lockstep_test.go)",
		// the same no-{T} counter-removal shape elsewhere in the corpus: the
		// predicate lowers them as the class, not one card at a time
		"Workhorse":          "Remove a +1/+1 counter: Add {C} (kop.CtrRemoveManaAb)",
		"Morselhoarder":      "Remove a -1/-1 counter: Add one mana of any color (kop.CtrRemoveManaAb)",
		"Iceberg":            "Remove an ice counter: Add {C} (kop.CtrRemoveManaAb)",
		"Druids' Repository": "Remove a charge counter: Add one mana of any color (kop.CtrRemoveManaAb)",
		"Gemstone Array":     "Remove a charge counter: Add one mana of any color (kop.CtrRemoveManaAb)",
	}
	// refused: the substring of each refusal measured against the corpus
	// (kgen -pool edhprobe, 2026-11-18); a card left refused is named here.
	refused := map[string]string{
		"Crystal Vein":           "a cost beyond {T} the kernel's mana walk cannot activate", // {T}, sacrifice this: {C}{C} beside {T}: {C}: a sacrifice-self alternative of the planner's unit, no pair model
		"Havenwood Battleground": "a cost beyond {T} the kernel's mana walk cannot activate", // the same shape, {G}{G}
		"Lake of the Dead":       "ETB replacement api Sacrifice",
		"Lazotep Quarry":         "a cost beyond {T} the kernel's mana walk cannot activate", // an any-colour second sacrifice ability, and a CopyPermanent SetCreatureTypes
		"Opal Palace":            "AddsCounters$ mana rider",
		"Orcish Lumberjack":      "mana ability cost T+Sac: the kernel's mana walk cannot activate it", // Combo R G, three mana
		"Heritage Druid":         "mana ability cost TapX: the kernel's mana walk cannot activate it",  // tap three Elves
	}
	var names []string
	for n := range lowered {
		names = append(names, n)
	}
	for n := range refused {
		names = append(names, n)
	}
	lw := lowerNamedPool(t, names)
	all, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for i, c := range all {
		seen[c.Name] = true
		rs := lowerRefusals(c)
		if _, ok := lowered[c.Name]; ok {
			if len(rs) > 0 {
				t.Errorf("%s refused: %v", c.Name, rs)
				continue
			}
			cd := &bs[i].def
			nmana := 0
			for j := range cd.Abilities {
				if cd.Abilities[j].Kind != kop.AbMana {
					continue
				}
				nmana++
				if !kop.ManaAbilityHonoured(cd, j) {
					t.Errorf("%s: mana ability %d (%s) not honoured", c.Name, j, kop.ManaCostShape(&cd.Abilities[j]))
				}
			}
			if nmana == 0 {
				t.Errorf("%s: no mana ability", c.Name)
			}
			continue
		}
		want, ok := refused[c.Name]
		if !ok {
			continue // a token the pool pulls in (Urza's Construct, the Food)
		}
		hit := false
		for _, r := range rs {
			hit = hit || strings.Contains(r.Reason, want)
		}
		if !hit {
			t.Errorf("%s: refusals %v, want one holding %q (a card now lowered? move it to lowered with its lockstep)", c.Name, rs, want)
		}
	}
	for _, n := range names {
		if !seen[n] {
			t.Errorf("%s not in the pool", n)
		}
	}
}
