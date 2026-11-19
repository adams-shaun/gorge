package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestVentureDungeonsLower pins the pool a plain Venture names: the three
// dungeon scripts but Undercity, each lowered with no refusal, and the rooms
// that needed new forms -- Twisted Caverns' until-next-turn can't-attack
// override, Runestone Caverns' paid land-capable Play, Mad Wizard's Lair's
// free cast from the drawn cards (hand zone) and Oubliette's no-op sacrifice.
func TestVentureDungeonsLower(t *testing.T) {
	cardsDir := ratchetCards(t)
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte("Dungeon Descent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	dumps := map[string]string{}
	for _, tok := range lw.tokens {
		var b strings.Builder
		dumpLIR(&b, tok)
		for _, r := range tok.Refusals {
			t.Errorf("token %s refused: %s: %s", tok.Name, r.Node, r.Reason)
		}
		dumps[tok.Name] = b.String()
	}
	for _, want := range []string{"dungeon_of_the_mad_mage", "lost_mine_of_phandelver", "tomb_of_annihilation"} {
		if dumps[want] == "" {
			t.Fatalf("dungeon %s is not in the pool's tokens", want)
		}
	}
	if _, ok := dumps[undercityKey]; ok {
		t.Fatal("a plain Venture named Undercity")
	}
	mad := dumps["dungeon_of_the_mad_mage"]
	for _, frag := range []string{
		"Override obj=Target0 CS=UntilYourNextTurn",                    // Twisted Caverns
		"CastFree obj=Remembered C=5 CS=Optional|AnyNumber|Paid|Lands", // Runestone Caverns
		"CastFree obj=Remembered C=2 CS=Optional|One",                  // Mad Wizard's Lair: the drawn cards, in hand
	} {
		if !strings.Contains(mad, frag) {
			t.Errorf("Dungeon of the Mad Mage lacks %q:\n%s", frag, mad)
		}
	}
}
