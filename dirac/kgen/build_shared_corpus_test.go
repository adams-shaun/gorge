package kgen

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// eqPools are real pools the equivalence test lowers both ways: two small
// pools and one set pool.
var eqPools = []string{"gravereduce", "spiritguide", "fdn", "mbc"}

// TestPoolsLoadOnTheSharedCorpus pins the hoist: a pool loads against the
// process's shared corpus (one open per process, cacheFresh's staleness rule
// -- the corpus walk, the token digest and the card census -- run once for
// the whole build), not against one OpenCorpusSubset per pool.
func TestPoolsLoadOnTheSharedCorpus(t *testing.T) {
	cardsDir := ratchetCards(t)
	shared, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := loadCardNames([]string{"Grizzly Bears", "Hill Giant", "Mountain"}, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	if p.reg != shared {
		t.Fatal("loadCardNames opened its own corpus subset; want the process's shared corpus")
	}
}

// oldPathPool is the pre-hoist loadCardNames: one OpenCorpusSubset registry
// for this pool alone. The equivalence test lowers real pools through it and
// through loadCardNames and compares the tables byte for byte, so a build
// path that makes a pool's table differ from the per-pool open fails here.
func oldPathPool(names []string, cardsDir string) (*pool, error) {
	sort.Strings(names)
	p := &pool{nDecks: map[string]int{}, byName: map[string]*cards.Card{}}
	reg, err := cards.OpenCorpusSubset(cardsDir, names)
	if err != nil {
		return nil, err
	}
	if p.typeWords, err = corpusTypeWords(cardsDir); err != nil {
		return nil, err
	}
	p.reg = reg
	for _, n := range names {
		c, ok := reg.Lookup(n)
		if !ok {
			return nil, fmt.Errorf("card %q not in the corpus", n)
		}
		p.names = append(p.names, n)
		p.nDecks[n] = 1
		p.byName[n] = c
	}
	return p, nil
}

// TestSharedCorpusTablesMatchPerPoolTables lowers each pool of eqPools on its
// own subset (the pre-hoist path) and on the shared corpus (the build path)
// and requires the table, the CardID index and the coverage to come out
// byte-identical.
func TestSharedCorpusTablesMatchPerPoolTables(t *testing.T) {
	cardsDir := ratchetCards(t)
	src := os.DirFS("../..")
	const key = "equivalence-test-key"
	for _, name := range eqPools {
		lb, err := fs.ReadFile(src, path.Join("dirac/cardset/pools", name, "cards.txt"))
		if err != nil {
			t.Fatal(err)
		}
		names := parseCardList(lb)
		pOld, err := oldPathPool(slices.Clone(names), cardsDir)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		pNew, err := loadCardNames(slices.Clone(names), cardsDir)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		dOld, cvOld, idxOld, err := tableOf(pOld, name, nil)
		if err != nil {
			t.Fatalf("%s: per-pool path: %v", name, err)
		}
		dNew, cvNew, idxNew, err := tableOf(pNew, name, nil)
		if err != nil {
			t.Fatalf("%s: shared path: %v", name, err)
		}
		tOld, tNew := cardset.EncodeTable(dOld, key), cardset.EncodeTable(dNew, key)
		if !bytes.Equal(tOld, tNew) {
			at := firstDiff(tOld, tNew)
			t.Fatalf("%s: table differs: %d bytes on the per-pool path, %d on the shared path, first diff at byte %d", name, len(tOld), len(tNew), at)
		}
		if !bytes.Equal(idsBytes(idxOld), idsBytes(idxNew)) {
			t.Fatalf("%s: the CardID index differs between the paths", name)
		}
		var mOld, mNew bytes.Buffer
		writeCoverage(&mOld, name, cvOld)
		writeCoverage(&mNew, name, cvNew)
		if !bytes.Equal(mOld.Bytes(), mNew.Bytes()) {
			t.Fatalf("%s: the coverage differs between the paths", name)
		}
	}
}

// firstDiff is the first byte the two differ at, len when they only differ in
// length.
func firstDiff(a, b []byte) int {
	n := min(len(a), len(b))
	for i := range n {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}
