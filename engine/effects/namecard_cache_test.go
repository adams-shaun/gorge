package effects

import (
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestNameChoicesCacheMatchesUncachedOnCorpus pins the memo's soundness on
// the REAL corpus: for every spec pureNameSpec admits (the corpus's NameCard
// ValidCards$ forms plus the description fallbacks), the memoised list equals
// the uncached filter walk, a second call returns the same shared list, and
// the memoised options are exactly the per-ask shape.
func TestNameChoicesCacheMatchesUncachedOnCorpus(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame(names(2))
	g.NameUniverse = reg.Universe()
	g.NameUniverseNames = NameUniverseNames(reg.Universe())
	if again := NameUniverseNames(reg.Universe()); &again[0] != &g.NameUniverseNames[0] {
		t.Fatal("NameUniverseNames recomputed for the same universe")
	}
	if !slices.Equal(g.NameUniverseNames, cards.UniverseOf(reg.AllCards()).Names()) {
		t.Fatal("memoised universe names differ from a fresh build")
	}
	specs := []string{"", "Card.nonLand", "Card.nonBasic", "Creature", "Card.Creature",
		"Land", "Card.nonLand+nonCreature", "Card.nonLand+nonArtifact",
		"Card.Land+nonBasic", "Card.Artifact"}
	for _, spec := range specs {
		if !pureNameSpec(spec) {
			t.Fatalf("pureNameSpec(%q) = false", spec)
		}
		for _, list := range []string{"", "Island,Forest,Black Lotus"} {
			want := nameChoicesFiltered(g, spec, list, false, nil)
			got := NameChoicesFromList(g, spec, "", list)
			if !slices.Equal(got, want) {
				t.Fatalf("spec %q list %q: cached %d names, uncached %d", spec, list, len(got), len(want))
			}
			again := NameChoicesFromList(g, spec, "", list)
			if len(got) > 0 && &again[0] != &got[0] {
				t.Fatalf("spec %q list %q: second call did not hit the memo", spec, list)
			}
			opts := NameOptions(got, 1)
			if len(opts) != len(got) {
				t.Fatalf("spec %q: %d options for %d names", spec, len(opts), len(got))
			}
			for i, o := range opts {
				if !reflect.DeepEqual(o, decision.Option{Index: i, Kind: "name", Label: got[i], Player: 1}) {
					t.Fatalf("spec %q option %d = %+v", spec, i, o)
				}
			}
		}
	}
	for _, impure := range []string{"Creature.cmcEQX", "Card.ManaCost=Imprinted", "Creature.ManaCost=Equipped", "Card.nonLand,Creature", "Card.!Land"} {
		if pureNameSpec(impure) {
			t.Fatalf("pureNameSpec(%q) = true, want false", impure)
		}
	}
}

// TestNameChoicesCacheConcurrent exercises the process-wide memo from
// parallel games (botbench's shape); run under -race.
func TestNameChoicesCacheConcurrent(t *testing.T) {
	universe := cards.UniverseOf(namecardUniverse(t))
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				g := state.NewGame(names(2))
				g.NameUniverse = universe
				g.NameUniverseNames = NameUniverseNames(universe)
				got := NameChoices(g, "Card.nonLand", "")
				if len(got) != 1 || got[0] != "Bear" {
					t.Errorf("NameChoices = %v", got)
					return
				}
				if opts := NameOptions(got, state.PlayerID(i%2)); len(opts) != 1 || opts[0].Player != state.PlayerID(i%2) {
					t.Errorf("NameOptions = %+v", opts)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// TestInternNameUniverseNamesSharesEqualLists pins the identity a replayed
// match's genesis gets for its persisted name list. A host rebuilds an
// archived match from its sidecar, which decodes a fresh copy of the list
// every time; before interning, each copy missed every identity-keyed memo
// and the demo held gigabytes of duplicate option slices.
func TestInternNameUniverseNamesSharesEqualLists(t *testing.T) {
	universe := cards.UniverseOf(namecardUniverse(t))
	live := NameUniverseNames(universe)
	if got := InternNameUniverseNames(universe, nil); &got[0] != &live[0] {
		t.Fatal("empty pin did not take the universe's own list")
	}
	if got := InternNameUniverseNames(universe, slices.Clone(live)); &got[0] != &live[0] {
		t.Fatal("a decoded copy of the current list did not intern to the universe's list")
	}
	// A list pinned under an older corpus differs from the live universe, so
	// every rebuild of such a match shares one pinned copy instead.
	old := []string{"Bear", "Forest", "Grizzly Bears"}
	first := InternNameUniverseNames(universe, slices.Clone(old))
	second := InternNameUniverseNames(universe, slices.Clone(old))
	if !slices.Equal(first, old) || &first[0] != &second[0] {
		t.Fatalf("pinned copies not shared: %v %p vs %p", first, &first[0], &second[0])
	}
	if a, b := NameOptions(first, 1), NameOptions(second, 1); &a[0] != &b[0] {
		t.Fatal("options for two rebuilds of one pinned list were not memoised once")
	}
	if got := InternNameUniverseNames(nil, nil); got != nil {
		t.Fatalf("no universe and no pin = %v, want nil", got)
	}
}

// TestNameChoicesPureMatchesFiltered pins nameChoicesPure (the image-row,
// type-signature walk) to the per-card matcher walk on the real corpus, for
// the imaged universe and a plain-slice one, every pure spec word, strict
// and not, with and without a ChooseFromList$.
//
// nameChoicesPureChunks is how many parallel tests that sweep (2 universes x
// 108 specs x 2 lists x strict/loose) is sharded into. Run as one serial test
// it was 54 s under gate contention -- over half the package's wall and at the
// one-minute per-test budget -- so chunk c owns the strided spec slice
// specs[c], specs[c+K], ... in every universe. Every comparison is kept; only
// the sharding is new.
const nameChoicesPureChunks = 8

// nameChoicesPureSpecs is the pure spec word list crossed with the modifier
// list the walker must agree on.
func nameChoicesPureSpecs() []string {
	words := []string{"Card", "Creature", "Land", "Artifact", "Enchantment", "Planeswalker", "Instant", "Sorcery", "Battle"}
	mods := []string{"", ".nonLand", ".nonBasic", ".Basic", ".Legendary", ".Snow", ".nonCreature", ".nonArtifact+nonLand",
		".Land+nonBasic", ".Artifact+Creature", ".nonLegendary+Creature", ".nonSnow+Land"}
	var specs []string
	for _, w := range words {
		for _, m := range mods {
			specs = append(specs, w+m)
		}
	}
	return specs
}

// nameChoicesPureChunk returns the indices of chunk c's specs.
func nameChoicesPureChunk(n, c int) []int {
	var out []int
	for i := c; i < n; i += nameChoicesPureChunks {
		out = append(out, i)
	}
	return out
}

// runNameChoicesPureChunk pins one shard of the pure-vs-filtered sweep.
func runNameChoicesPureChunk(t *testing.T, c int) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	specs := nameChoicesPureSpecs()
	idx := nameChoicesPureChunk(len(specs), c)
	for _, u := range []*cards.Universe{reg.Universe(), cards.UniverseOf(reg.AllCards())} {
		g := state.NewGame(names(2))
		g.NameUniverse = u
		g.NameUniverseNames = NameUniverseNames(u)
		for _, i := range idx {
			spec := specs[i]
			if !pureNameSpec(spec) {
				t.Errorf("pureNameSpec(%q) = false", spec)
				continue
			}
			for _, list := range []string{"", "Island,Forest,Black Lotus"} {
				for _, strict := range []bool{false, true} {
					want := nameChoicesFiltered(g, spec, list, strict, nil)
					got := nameChoicesPure(g, spec, list, strict, nil)
					if !slices.Equal(got, want) {
						t.Errorf("spec %q list %q strict %v: pure %d names, filtered %d", spec, list, strict, len(got), len(want))
					}
				}
			}
		}
	}
}

func TestNameChoicesPureMatchesFilteredChunk00(t *testing.T) { runNameChoicesPureChunk(t, 0) }
func TestNameChoicesPureMatchesFilteredChunk01(t *testing.T) { runNameChoicesPureChunk(t, 1) }
func TestNameChoicesPureMatchesFilteredChunk02(t *testing.T) { runNameChoicesPureChunk(t, 2) }
func TestNameChoicesPureMatchesFilteredChunk03(t *testing.T) { runNameChoicesPureChunk(t, 3) }
func TestNameChoicesPureMatchesFilteredChunk04(t *testing.T) { runNameChoicesPureChunk(t, 4) }
func TestNameChoicesPureMatchesFilteredChunk05(t *testing.T) { runNameChoicesPureChunk(t, 5) }
func TestNameChoicesPureMatchesFilteredChunk06(t *testing.T) { runNameChoicesPureChunk(t, 6) }
func TestNameChoicesPureMatchesFilteredChunk07(t *testing.T) { runNameChoicesPureChunk(t, 7) }

// TestNameChoicesPureChunksPartitionTheSpecs holds the sharding to full
// coverage: one TestNameChoicesPureMatchesFilteredChunkN per chunk, and every
// spec index in exactly one chunk.
func TestNameChoicesPureChunksPartitionTheSpecs(t *testing.T) {
	const n = 1003 // any size: the stride covers every index for every n
	for _, size := range []int{0, 1, nameChoicesPureChunks - 1, nameChoicesPureChunks, n} {
		seen := make([]int, size)
		for c := 0; c < nameChoicesPureChunks; c++ {
			for _, i := range nameChoicesPureChunk(size, c) {
				seen[i]++
			}
		}
		for i, k := range seen {
			if k != 1 {
				t.Fatalf("n=%d: spec %d is in %d chunks", size, i, k)
			}
		}
	}
	if tests := len([]func(*testing.T){TestNameChoicesPureMatchesFilteredChunk00,
		TestNameChoicesPureMatchesFilteredChunk01, TestNameChoicesPureMatchesFilteredChunk02,
		TestNameChoicesPureMatchesFilteredChunk03, TestNameChoicesPureMatchesFilteredChunk04,
		TestNameChoicesPureMatchesFilteredChunk05, TestNameChoicesPureMatchesFilteredChunk06,
		TestNameChoicesPureMatchesFilteredChunk07}); tests != nameChoicesPureChunks {
		t.Fatalf("%d chunk tests for %d chunks", tests, nameChoicesPureChunks)
	}
}
