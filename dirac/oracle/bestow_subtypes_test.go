package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestBestowedHydraSubtypesLockstep replays a dr-mbc-bbaa game in which a
// Nyxborn Hydra, cast for its bestow cost, resolves attached to a Wall of
// Roots. Gorge's derived type list for the attached card is Enchantment Aura
// (Hydra stays a creature type), and the reference projection maps the word
// Aura onto the matchup's Aura subtype whenever the pool holds a real Aura, so
// the kernel's subtype mask must carry it (CR 702.103b, CR 303.4). The older
// TestBestowLockstep pool holds no Aura card, so its table has no such subtype.
func TestBestowedHydraSubtypesLockstep(t *testing.T) {
	t.Parallel()
	const file = "repro/bestow_aura_subtype"
	r, err := ReadRepro("testdata/" + file + ".repro.json")
	if err != nil {
		t.Fatal(err)
	}
	// the preconditions: the pool's table has an Aura subtype, and the
	// Hydra is in the game, so the compared masks differ without the fix
	set := cardsettest.Synth(t, "dr-mbc-bbaa")
	entries := func(names []string) []cardset.Entry {
		var es []cardset.Entry
		for _, n := range names {
			es = append(es, cardset.Entry{Name: n, Count: 1})
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(tbl.Subtypes, "Aura") {
		t.Fatal("the pool's table has no Aura subtype: the bestowed Hydra's mask would not differ")
	}
	if !slices.Contains(r.Cards[0], "Nyxborn Hydra") && !slices.Contains(r.Cards[1], "Nyxborn Hydra") {
		t.Fatal("the recording's decks hold no Nyxborn Hydra")
	}
	res, skipped := synthReproRun(t, file, "dr-mbc-bbaa", Config{GorgeAutopaySurface: true})
	if skipped != 0 {
		t.Errorf("%d scripted actions were not offered", skipped)
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s\n %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if len(res.Actions) < 334 {
		t.Fatalf("the replay ended after %d decisions, before the bestowed Hydra's decision 334", len(res.Actions))
	}
}
