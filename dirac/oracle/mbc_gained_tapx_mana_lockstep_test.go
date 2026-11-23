package oracle

import (
	"errors"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// CR 613.1f, 605.1a, 602.2: a Dark Impostor with Citanul Stalwart exiled under
// it has the Stalwart's "{T}, Tap an untapped artifact or creature you control:
// Add one mana of any color". gorge's payment census pays it with the Impostor's
// {T} and a second creature's tap, so a {4}{B}{B} activation (and Windbrisk
// Heights's {W},{T} one) was offered on five lands and a spare creature; the
// kernel's grant view prices only a plain {T} and offered neither (decision 554
// of tail-48-c000 pair 4, autopay). The kernel models no gained tap-another
// source, so the matchup is a pair refusal: the recorded decks must be refused
// as a gainsof interaction, and the same decks without the tap-another creature
// must not be.
func TestMBCDarkImpostorReprosLockstep(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro("testdata/repro/gained_tapx_mana_ability_census.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	set := globSet(t, r.Pool, r)
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, 0, len(names))
		for _, n := range names {
			es = append(es, cardset.Entry{Name: n, Count: 1})
		}
		return es
	}
	for _, name := range []string{"Dark Impostor", "Citanul Stalwart"} {
		if !slices.Contains(r.Cards[0], name) && !slices.Contains(r.Cards[1], name) {
			t.Fatalf("precondition: the recorded decks hold no %s", name)
		}
	}
	_, _, _, err = set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	var ir *cardset.InteractionRefusal
	if !errors.As(err, &ir) || ir.Reason != dirac.StallName(dirac.StallGainsOf) {
		t.Fatalf("the recorded matchup: err %v, want the gainsof pair refusal", err)
	}
	without := func(names []string) []string {
		return slices.DeleteFunc(slices.Clone(names), func(n string) bool { return n == "Citanul Stalwart" })
	}
	if _, _, _, err = set.Matchup(entries(without(r.Cards[0])), entries(without(r.Cards[1]))); err != nil {
		t.Fatalf("without the tap-another creature the matchup is refused: %v", err)
	}
}
