package kgen

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestSynthListsAreDerivedAndFrontNamed: each Standard list resolves to the corpus's
// own names, so a Room deals, and no pool name needs a file in the tree.
func TestSynthListsAreDerivedAndFrontNamed(t *testing.T) {
	cardsDir := ratchetCards(t)
	lists, err := synthStandardLists(gorge.TableSources(), cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(lists) != 22 {
		t.Errorf("%d Standard lists, want the 22 of standard.txt", len(lists))
	}
	var dsk []string
	for _, l := range lists {
		if !strings.HasPrefix(l.name, "std-") || !slices.IsSorted(l.names) {
			t.Errorf("%s: bad name or order", l.name)
		}
		if l.name == "std-dsk" {
			dsk = l.names
		}
		for _, n := range l.names {
			if strings.Contains(n, " // ") { // SP//dr is a real card's name
				t.Errorf("%s holds %q: a list name, not the corpus's", l.name, n)
			}
		}
	}
	if !slices.Contains(dsk, "Bottomless Pool") {
		t.Errorf("DSK lost its Room: %d names", len(dsk))
	}
}

// TestSynthSplitsTheListsThatDoNotFit: a tail bucket over a table's subtype masks
// builds as two pools, which together hold the whole list and are stable.
func TestSynthSplitsTheListsThatDoNotFit(t *testing.T) {
	cardsDir := ratchetCards(t)
	std, err := synthStandardLists(gorge.TableSources(), cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	tail, _, err := synthTail(std, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	var over synthList
	for _, l := range tail {
		if _, err := buildOneTable(l.name, l.names, cardsDir, "k"); isSizeRefusal(err) {
			over = l
			break
		}
	}
	if over.name == "" {
		t.Skip("every tail bucket fits one table: nothing to split")
	}
	build := func() []synthTable {
		ts, err := buildSynthList(over, "", cardsDir, "k")
		if err != nil {
			t.Fatal(err)
		}
		return ts
	}
	ts := build()
	if len(ts) != 2 || ts[0].name != over.name+"-a" || ts[1].name != over.name+"-b" {
		var names []string
		for _, x := range ts {
			names = append(names, x.name)
		}
		t.Fatalf("%s split into %v, want its -a and -b", over.name, names)
	}
	var cards []string
	for _, x := range ts {
		pd, _, err := cardset.DecodeTable(x.data.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range cardset.SynthBasics {
			if !slices.Contains(pd.Names[:pd.NumCards], b) {
				t.Errorf("%s lacks %s", x.name, b)
			}
		}
		for _, n := range pd.Names[:pd.NumCards] {
			if !cardset.IsSynthBasic(n) {
				cards = append(cards, n)
			}
		}
	}
	slices.Sort(cards)
	want := slices.DeleteFunc(slices.Clone(over.names), cardset.IsSynthBasic)
	if !slices.Equal(cards, want) {
		t.Errorf("the halves hold %d cards, the list %d", len(cards), len(want))
	}
	for i, x := range build() {
		if !slices.Equal(x.data.Bytes(), ts[i].data.Bytes()) {
			t.Errorf("%s: two builds differ", x.name)
		}
	}
}

func TestSizeRefusalReadsTheLimit(t *testing.T) {
	for err, want := range map[error]bool{
		nil:                              false,
		errors.New("some other failure"): false,
		fmt.Errorf("refusing the pool: %s 200 subtypes", tableLimitMark): true,
	} {
		if got := isSizeRefusal(err); got != want {
			t.Errorf("isSizeRefusal(%v) = %v", err, got)
		}
	}
}

// TestNoSynthPoolIsTracked: the lists come from the set lists and the corpus, so the
// embedded tree holds no pool directory of their names.
func TestNoSynthPoolIsTracked(t *testing.T) {
	for _, pattern := range []string{"dirac/cardset/pools/std-*/*", "dirac/cardset/pools/tail-*/*"} {
		got, err := fs.Glob(gorge.TableSources(), pattern)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("%s matches %v: a synthetic pool's list or table is committed", pattern, got)
		}
	}
}

// TestSynthDraftPoolBuildsSplit: draftpools.txt's MBC list becomes dr-mbc, too big
// for one table's subtype masks, so it builds as hash halves that each fit and
// together hold every name of the list.
func TestSynthDraftPoolBuildsSplit(t *testing.T) {
	cardsDir := ratchetCards(t)
	lists, err := synthDraftLists(gorge.TableSources(), cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(lists) != 1 || lists[0].name != "dr-mbc" || !cardset.IsSynthName(lists[0].name) {
		t.Fatalf("draft lists %v, want dr-mbc alone", lists)
	}
	ts, err := buildSynthList(lists[0], "", cardsDir, "k")
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) < 2 {
		t.Fatalf("dr-mbc built as %d table, want a split", len(ts))
	}
	var cards []string
	for _, x := range ts {
		if !strings.HasPrefix(x.name, "dr-mbc-") {
			t.Errorf("%s: want a dr-mbc-<half> name", x.name)
		}
		pd, _, err := cardset.DecodeTable(x.data.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range pd.Names[:pd.NumCards] {
			if !cardset.IsSynthBasic(n) {
				cards = append(cards, n)
			}
		}
		t.Logf("%s: %d cards", x.name, pd.NumCards)
	}
	slices.Sort(cards)
	if want := slices.DeleteFunc(slices.Clone(lists[0].names), cardset.IsSynthBasic); !slices.Equal(cards, want) {
		t.Errorf("the halves hold %d cards, the list %d", len(cards), len(want))
	}
}
