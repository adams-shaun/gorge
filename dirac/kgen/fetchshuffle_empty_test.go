package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The empty Defined$ fetch leg pays a deferred search shuffle only when no
// later explicit DB$ Shuffle closes the chain (gorge zone_search.go
// emptyDefinedFetch / chainPaysDeferredShuffle). kgen marks the paying leg
// with W=You, which the kernel's emptyFetchShuffle reads; the explicit-tail
// cards (Celestial Reunion, Archdruid's Charm, Guidelight Pathmaker) must not
// carry it or they shuffle once at the empty leg and once at DB$ Shuffle.
func TestFetchShuffleEmptyLegMarker(t *testing.T) {
	cardsDir := ratchetCards(t)
	dir := t.TempDir()
	list := filepath.Join(dir, "cards.txt")
	names := []string{"Nissa's Pilgrimage", "Celestial Reunion", "Archdruid's Charm", "Guidelight Pathmaker"}
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	byName := make(map[string]*lCard)
	for _, c := range lw.cards {
		byName[c.Name] = c
	}
	nissa := byName["Nissa's Pilgrimage"]
	if nissa == nil {
		t.Fatal("Nissa's Pilgrimage: not lowered")
	}
	for _, r := range nissa.Refusals {
		t.Fatalf("Nissa's Pilgrimage: refused: %s: %s", r.Node, r.Reason)
	}
	lir := func(name string) string {
		var sb strings.Builder
		dumpLIR(&sb, byName[name])
		return sb.String()
	}
	if nissa := lir("Nissa's Pilgrimage"); !strings.Contains(nissa, "ShuffleOwed W=You") {
		t.Fatalf("Nissa's Pilgrimage: empty DBHand leg lacks the empty-shuffle marker:\n%s", nissa)
	}
	checked := 0
	for _, name := range []string{"Celestial Reunion", "Archdruid's Charm", "Guidelight Pathmaker"} {
		c := byName[name]
		if c == nil || len(c.Refusals) != 0 {
			continue // kgen does not lower this card; there is no kernel parity to keep
		}
		checked++
		for _, line := range strings.Split(lir(name), "\n") {
			if strings.Contains(line, "ShuffleOwed") && strings.Contains(line, "W=You") {
				t.Fatalf("%s: explicit DB$ Shuffle tail must own the shuffle, but the fetch carries the empty-leg marker:\n%s", name, line)
			}
		}
	}
	if checked == 0 {
		t.Fatal("precondition: no explicit-tail carrier lowered, marker test would be vacuous")
	}
}
