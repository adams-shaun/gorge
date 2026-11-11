package kgen

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// manaCostCensus lists, per cost shape, the accepted ALL cards whose mana
// ability the kernel cannot activate.
func manaCostCensus(t *testing.T) map[string][]string {
	t.Helper()
	p, err := loadCardList(testSetlistPath(t, "ALL"), ratchetCards(t))
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	all, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	for i, c := range all {
		if len(lowerRefusals(c)) > 0 {
			continue
		}
		cd := &bs[i].def
		for j := range cd.Abilities {
			if !kop.ManaAbilityHonoured(cd, j) {
				shape := kop.ManaCostShape(&cd.Abilities[j])
				out[shape] = append(out[shape], c.Name)
			}
		}
	}
	return out
}

// TestManaAbilityCostsAreHonoured: every card kgen
// accepts has only mana abilities the kernel activates. A shape the kernel's
// mana walk cannot pay (a tap-another-creature cost, a sacrifice, life, a
// counter, mana in the cost) is refused, never lowered and silently dropped.
func TestManaAbilityCostsAreHonoured(t *testing.T) {
	got := manaCostCensus(t)
	var shapes []string
	n := 0
	for s, names := range got {
		shapes = append(shapes, s)
		n += len(names)
	}
	sort.Strings(shapes)
	for _, s := range shapes {
		sort.Strings(got[s])
		t.Errorf("%-24s %3d accepted cards with a mana ability the kernel cannot activate: %v", s, len(got[s]), got[s])
	}
	if n != 0 {
		t.Errorf("%d mana abilities over %d shapes", n, len(shapes))
	}
}

// TestManaCostPoolsHaveNoDroppedAbility: no generated kernel pool
// (dirac/cardset/pools) lists a card with a mana ability the kernel cannot
// activate, so no lockstep pool silently plays one.
func TestManaCostPoolsHaveNoDroppedAbility(t *testing.T) {
	bad := map[string]string{}
	for shape, names := range manaCostCensus(t) {
		for _, n := range names {
			bad[n] = shape
		}
	}
	lists, err := filepath.Glob("../cardset/pools/*/cards.txt")
	fixtures, _ := filepath.Glob("../cardset/pools/fixtures/*/cards.txt")
	lists = append(lists, fixtures...)
	if err != nil || len(lists) == 0 {
		t.Fatalf("no pool card lists (%v)", err)
	}
	for _, l := range lists {
		b, err := os.ReadFile(l)
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			if shape, ok := bad[strings.TrimSpace(n)]; ok {
				t.Errorf("%s lists %q, whose mana ability (%s) the kernel cannot activate", filepath.Base(filepath.Dir(l)), n, shape)
			}
		}
	}
}

// TestManaCostShapesAcceptedAndRefused names the carriers: "{T}, tap an
// untapped permanent you control: Add one mana of any color" lowers (the
// kernel's tap-another-permanent gap), so does a filter as the first mana
// ability (kop.FilterManaAb), every other unread cost shape is refused with
// the named reason.
func TestManaCostShapesAcceptedAndRefused(t *testing.T) {
	accept := []string{"Saruli Caretaker", "Springleaf Drum", "Citanul Stalwart", "Dragonbroods' Relic",
		"Gene Pollinator", "Jaspera Sentinel", "Loam Dryad", "Black Lotus", "Wall of Roots", "Llanowar Elves", "Baylen, the Haymaker", "Birchlore Rangers",
		"Three Tree Mascot", "Abzan Devotee", "Prophetic Prism", "Giant's Boulder", "Omni-Cheese Pizza", // (the last five: kop.FilterManaAb)
		"Jeskai Devotee", "Mardu Devotee", "Sultai Devotee", "Temur Devotee", // (kop.Ability.ManaColOrd)
		"Mana Confluence"} // (a sole {T}, Pay N life ability: kop.SoleLifeManaAb)
	refuse := map[string]string{
		"Heritage Druid": "TapX", "Ashnod's Altar": "Sac",
		"Cabal Coffers": "T+Mana", "Agent of Stromgald": "Mana", "Gilded Goose": "T+Sac", "Druids' Repository": "RemoveCtr",
		"Moonsnare Prototype": "T+TapX",
	}
	named := map[string]string{}
	names := append([]string(nil), accept...)
	for n := range refuse {
		names = append(names, n)
	}
	for n := range named {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerPoolBuilt(t, names) {
		var reason string
		for _, r := range c.refusals {
			if strings.Contains(r.Reason, "cannot activate it") {
				reason = r.Reason
			}
		}
		if why, ok := named[c.name]; ok {
			seen++
			if len(c.refusals) != 1 || !strings.Contains(c.refusals[0].Reason, why) {
				t.Errorf("%s: want the refusal %q, got %v", c.name, why, c.refusals)
			}
			continue
		}
		if shape, ok := refuse[c.name]; ok {
			seen++
			if !strings.Contains(reason, "mana ability cost "+shape+":") {
				t.Errorf("%s: want a refusal for mana ability cost %s, got %v", c.name, shape, c.refusals)
			}
			continue
		}
		seen++
		if len(c.refusals) > 0 {
			t.Errorf("%s refused: %v", c.name, c.refusals)
		}
	}
	if seen != len(names) {
		t.Errorf("built %d of %d cards", seen, len(names))
	}
}

type builtCard struct {
	name     string
	refusals []refusal
}

// lowerPoolBuilt lowers and builds names as one pool and returns each card
// with its lowering and build refusals.
func lowerPoolBuilt(t *testing.T, names []string) []builtCard {
	t.Helper()
	lw := lowerNamedPool(t, names)
	all, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	var out []builtCard
	for i, c := range all {
		if c.Token {
			continue
		}
		out = append(out, builtCard{c.Name, lowerRefusals(c)})
		_ = bs[i]
	}
	return out
}
