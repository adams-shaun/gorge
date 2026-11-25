package effects

import (
	"sort"
	"testing"
)

// TestCreatureTypeWordListOffersTimeLordOnce pins CR 205.3e/205.3m on the
// sorted creature-type choice vocabulary: Time Lord is one two-word creature
// type and the list offers it once, and its words Time and Lord are not
// standalone creature types.
func TestCreatureTypeWordListOffersTimeLordOnce(t *testing.T) {
	vocabulary := CreatureTypeWordList()
	if len(vocabulary) < 2 {
		t.Fatal("precondition: the creature-type vocabulary is empty")
	}
	if !sort.StringsAreSorted(vocabulary) {
		t.Fatal("creature-type vocabulary is not sorted")
	}
	index := make(map[string]int, len(vocabulary))
	for i, word := range vocabulary {
		if _, dup := index[word]; dup {
			t.Fatalf("creature-type vocabulary offers %q more than once", word)
		}
		index[word] = i
	}
	if _, ok := index["Human"]; !ok {
		t.Fatal("precondition: the vocabulary must still contain Human")
	}
	if _, ok := index["Time Lord"]; !ok {
		t.Error(`CreatureTypeWordList does not offer "Time Lord"`)
	}
	for _, word := range []string{"Time", "Lord"} {
		if _, ok := index[word]; ok {
			t.Errorf("CreatureTypeWordList offers the non-type word %q", word)
		}
		if CreatureTypeWords(word) {
			t.Errorf("CreatureTypeWords(%q) is true, want false: it is not a creature type", word)
		}
	}
	if !CreatureTypeWords("Time Lord") {
		t.Error(`CreatureTypeWords("Time Lord") is false, want true`)
	}
}
