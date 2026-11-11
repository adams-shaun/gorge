package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestExwStaticsLowering: the static readers of the exiled-with link
// lower with no refusal -- Keen-Eyed Curator's card-types
// count, Dawnhand Dissident's and Maralen's permission filters and Dawnhand's
// three-counter raise -- while Defined$ ExiledWith keeps its lockstep-proven
// sweep form and a target, a cost or a granted ability still refuses.
func TestExwStaticsLowering(t *testing.T) {
	cardsDir := ratchetCards(t)
	dir := t.TempDir()
	list := filepath.Join(dir, "cards.txt")
	names := []string{"Keen-Eyed Curator", "Dawnhand Dissident", "Maralen, Fae Ascendant", "Veteran Survivor", "Quilled Greatwurm", "The Princess Takes Flight"}
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	cv, err := generate(p, "kexwstat", out, filepath.Join(out, idsFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range cv.refused {
		if r.Node == supportNode {
			continue
		}
		t.Errorf("%s refused: %s", r.Card, r.Reason)
	}
	dump := map[string]string{}
	for _, c := range lowerPool(p).cards {
		var sb strings.Builder
		dumpLIR(&sb, c)
		dump[c.Name] = sb.String()
	}
	for name, wants := range map[string][]string{
		// Dawnhand: the Exile zone permission with the flag filter and three counters
		"Dawnhand Dissident": {"static MayPlayFrom F={types=0x2 owner=1 ExiledWith}", "amount=3 zone=5"},
		// Maralen: the flag composes with the mana-value bound and the Dig records
		"Maralen, Fae Ascendant": {"static MayPlayFrom F={types=0x0 not=0x1 EnteredThisTurn ExiledWith cmp(2 2 0 X )}", "zone=5", "Dig obj=Target0"},
		// Keen-Eyed: the distinct card types of the exile cards linked to it
		"Keen-Eyed Curator": {"cond=Value(op1,ExileTypes({types=0x0 ExiledWith}),4)"},
		"Veteran Survivor":  {"cond=Value(op1,ExiledWithN,3)"},
		"Quilled Greatwurm": {"amount=6 zone=3"},
		// the Defined$ ExiledWith sweep keeps the predicate form (PredN 1)
		"The Princess Takes Flight": {"ChangeZoneAll", "cmp(28 0 1"},
	} {
		for _, w := range wants {
			if !strings.Contains(dump[name], w) {
				t.Errorf("%s: LIR lacks %q:\n%s", name, w, dump[name])
			}
		}
	}

	// the word reads a source: a continuous static's own, or a resolving
	// mover's; a target, a cost or a granted ability has none
	card := &cards.Face{SVars: map[string]string{}}
	known := func(string) bool { return false }
	for _, c := range []struct {
		name string
		lc   lowerCtx
		ok   bool
	}{
		{"bare", lowerCtx{}, false},
		{"static", lowerCtx{exwStatic: true}, true},
		{"static granted", lowerCtx{exwStatic: true, granted: 1}, false},
		{"mover without an ability", lowerCtx{exwOK: true}, false},
		{"mover", lowerCtx{exwOK: true, ab: &lAbility{}}, true},
		{"mover granted", lowerCtx{exwOK: true, ab: &lAbility{}, granted: 1}, false},
	} {
		lc := c.lc
		lc.face, lc.known, lc.out = card, known, &lCard{}
		_, err := lc.filter("Card.ExiledWithSource")
		if (err == nil) != c.ok {
			t.Errorf("%s: filter err %v, want ok=%v", c.name, err, c.ok)
		}
		if c.ok && !lc.out.ExiledWith {
			t.Errorf("%s: the card is not marked as an ExiledWith reader", c.name)
		}
	}
}

// TestRemoveAnyCounterN: the raise's count is read for 1..255 of the one
// shape the kernel pays (counters from among your creatures).
func TestRemoveAnyCounterN(t *testing.T) {
	for rc, want := range map[string]int{
		"RemoveAnyCounter<6/Any/Creature>":   6,
		"RemoveAnyCounter<3/Any/Creature>":   3,
		"RemoveAnyCounter<255/Any/Creature>": 255,
		"RemoveAnyCounter<0/Any/Creature>":   0,
		"RemoveAnyCounter<03/Any/Creature>":  0,
		"RemoveAnyCounter<256/Any/Creature>": 0,
		"RemoveAnyCounter<3/P1P1/Creature>":  0,
		"RemoveAnyCounter<3/Any/Land>":       0,
		"RemoveAnyCounter</Any/Creature>":    0,
		"RemoveAnyCounter<x/Any/Creature>":   0,
		"Discard<1/Card>":                    0,
	} {
		if n, ok := removeAnyCounterN(rc); n != want || ok != (want != 0) {
			t.Errorf("%s: got %d, %v; want %d", rc, n, ok, want)
		}
	}
}
