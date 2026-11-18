package oracle

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kgen"
)

// synthWithFixture is tail-00 widened by two cards tail-52 holds and tail-00 does
// not, as kfuzz -synth-with builds it: Snow-Covered Island (the enabler a focus line
// names) and a spell no focus line names. It returns the union, the lent names and
// the main pool's.
func synthWithFixture(t *testing.T) (union cardset.Set, extra map[string]bool, lent string, main cardset.Set) {
	t.Helper()
	main = cardsettest.Synth(t, "tail-00")
	other := cardsettest.Synth(t, "tail-52")
	reg := synthReg(t)
	const island = "Snow-Covered Island"
	if _, ok := main.Lookup(island); ok {
		t.Fatalf("tail-00 holds %s: the fixture needs a pool without it", island)
	}
	if i, ok := other.Lookup(island); !ok || other.Status(i) == cardset.StatusNotLowered {
		t.Fatalf("tail-52 does not lower %s", island)
	}
	for i, n := range other.Names()[:other.NumCards()] {
		if _, in := main.Lookup(n); in || other.Status(i) == cardset.StatusNotLowered || cardset.IsSynthBasic(n) {
			continue
		}
		if c, ok := reg.Lookup(n); ok && kindOf(c) == synthSpell {
			lent = n
			break
		}
	}
	if lent == "" {
		t.Fatal("tail-52 lends no spell")
	}
	names := slices.Clone(main.Names()[:main.NumCards()])
	names = append(names, island, lent)
	slices.Sort(names)
	dir, err := cardset.CorpusDir()
	if err != nil {
		t.Fatal(err)
	}
	d, err := kgen.SynthUnionTable(names, dir, "tail-00")
	if err != nil {
		t.Fatal(err)
	}
	return cardset.Raw(cardset.NewDataSet(d)), map[string]bool{island: true, lent: true}, lent, main
}

// focusTail is a focus list: Adarkar Windform (tail-00, needs {S}) with the repeated
// island, and a second focus card with no enabler.
func focusTail(t *testing.T, main cardset.Set, islands int) []FocusCard {
	t.Helper()
	const head = "Adarkar Windform"
	if i, ok := main.Lookup(head); !ok || main.Status(i) == cardset.StatusNotLowered {
		t.Fatalf("tail-00 does not lower %s", head)
	}
	second := ""
	reg := synthReg(t)
	for i, n := range main.Names()[:main.NumCards()] {
		if c, ok := reg.Lookup(n); ok && n != head && main.Status(i) != cardset.StatusNotLowered && kindOf(c) == synthSpell && !cardset.IsSynthBasic(n) {
			second = n
			break
		}
	}
	if second == "" {
		t.Fatal("tail-00 holds no second spell")
	}
	var with []string
	for range islands {
		with = append(with, "Snow-Covered Island")
	}
	return []FocusCard{{Name: head, With: with}, {Name: second}}
}

// TestFocusPoolSynthWith: an enabler another pool lends is dealt beside its focus card;
// without the lending the same list is the error it always was; a lent card is never
// the filler, so a deck whose focus line names no lent card holds none.
func TestFocusPoolSynthWith(t *testing.T) {
	union, extra, lent, main := synthWithFixture(t)
	reg := synthReg(t)
	const island = "Snow-Covered Island"

	if _, _, err := FocusPool(reg, main, "f", focusTail(t, main, 1), 1, 1, 4); err == nil || !strings.Contains(err.Error(), `enabler "Snow-Covered Island" of "Adarkar Windform" is not a card tail-00 plays`) {
		t.Fatalf("without -synth-with: err %v, want the enabler error", err)
	}
	if _, _, err := FocusPoolWith(reg, union, extra, "f", []FocusCard{{Name: island}}, 1, 1, 4); err == nil || !strings.Contains(err.Error(), "is not a card") {
		t.Fatalf("a lent card as a focus card: err %v, want a refusal", err)
	}

	focus := focusTail(t, main, 1)
	pool, files, err := FocusPoolWith(reg, union, extra, "f", focus, 1, 1, 40)
	if err != nil {
		t.Fatal(err)
	}
	withIsland, without := 0, 0
	for i, pd := range pool {
		f := files[pd.Name]
		n := map[string]int{}
		total := 0
		for _, e := range f.Cards {
			n[e.Name] += e.Count
			total += e.Count
		}
		if total != synthDeckSize {
			t.Errorf("%s: %d cards, want %d", pd.Name, total, synthDeckSize)
		}
		if n[lent] != 0 {
			t.Errorf("%s: dealt %q, which no focus line names", pd.Name, lent)
		}
		if i%len(focus) == 0 {
			if n["Adarkar Windform"] != synthMaxCopies || n[island] != 1 {
				t.Errorf("%s: Windform x%d, %s x%d: want %d and 1", pd.Name, n["Adarkar Windform"], island, n[island], synthMaxCopies)
			}
			withIsland++
		} else {
			if n[island] != 0 {
				t.Errorf("%s: dealt %s as filler beside no focus line naming it", pd.Name, island)
			}
			without++
		}
	}
	if withIsland == 0 || without == 0 {
		t.Fatalf("the deal did not cover both kinds of deck: %d with the island, %d without", withIsland, without)
	}
}

// TestFocusPoolRepeatedEnabler: an enabler a focus line names k times is k copies when
// it is a land, past the nonbasic land slots (synthLands), and the deck stays 40.
func TestFocusPoolRepeatedEnabler(t *testing.T) {
	union, extra, _, main := synthWithFixture(t)
	reg := synthReg(t)
	const island = "Snow-Covered Island"
	for _, k := range []int{1, 2, synthLands + 3} {
		focus := focusTail(t, main, k)
		pool, files, err := FocusPoolWith(reg, union, extra, "f", focus, 1, 1, 2)
		if err != nil {
			t.Fatal(err)
		}
		f := files[pool[0].Name]
		got, total := 0, 0
		for _, e := range f.Cards {
			total += e.Count
			if e.Name == island {
				got = e.Count
			}
		}
		if got != k || total != synthDeckSize {
			t.Errorf("%d repeats: %s x%d, deck of %d: want x%d of 40", k, island, got, total, k)
		}
	}
}
