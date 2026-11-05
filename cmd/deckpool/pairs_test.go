package main

import (
	"bytes"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// pairRun drafts pods TRK pods at seed and writes them under a fresh out.
// f, when set, overrides the pairing flags; validate runs in run, so an
// invalid pairing surfaces as a run error.
func pairRun(t *testing.T, pods int, seed uint64, f func(*options)) (options, string) {
	t.Helper()
	corpus := testutil.CorpusDirOrSkip(t)
	o := options{set: "TRK", pods: pods, seed: seed, drafter: "balanced", builder: "balanced",
		corpus: corpus, out: filepath.Join(t.TempDir(), "pool"),
		pairing: "roundrobin", gamesN: 1, split: "all"}
	if f != nil {
		f(&o)
	}
	if err := run(o, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	return o, o.out
}

// seatOf is a deck id's (pod, seat): the index lists decks in pod order and
// carries each deck's seat.
func seatOf(t *testing.T, out string) map[string][2]int {
	t.Helper()
	idx := readIndex(t, out)
	if len(idx)%seats != 0 {
		t.Fatalf("%d index lines, not a multiple of %d", len(idx), seats)
	}
	out2 := make(map[string][2]int, len(idx))
	for i, e := range idx {
		out2[e.ID] = [2]int{i / seats, e.Seat}
	}
	return out2
}

// readPairs reads pairs.tsv through dirac's own parser and checks every
// name is a written deck, no pair is a deck against itself, and no seed
// column is empty.
func readPairs(t *testing.T, out string) []oracle.PairSpec {
	t.Helper()
	ps, err := oracle.ExplicitPairs(filepath.Join(out, "pairs.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	decks := seatOf(t, out)
	for i, p := range ps {
		if _, ok := decks[p.A]; !ok {
			t.Fatalf("pair %d names unwritten deck %s", i, p.A)
		}
		if _, ok := decks[p.B]; !ok {
			t.Fatalf("pair %d names unwritten deck %s", i, p.B)
		}
		if p.A == p.B {
			t.Fatalf("pair %d is %s against itself", i, p.A)
		}
		if p.SeedText == "" {
			t.Fatalf("pair %d carries no seed", i)
		}
	}
	return ps
}

// TestDeckpoolPairsParse: roundrobin pairs every seat pair of every pod
// once, its seeds the pod's PCG(seed, saltPairs<<32|pod) stream in pair
// order; random draws -pairs-per-pod distinct pairs, -games-per-pair games
// each with its own seed; cross pairs seat s of pod p against seat s of pod
// p+1. The manifest records the pairing flags.
func TestDeckpoolPairsParse(t *testing.T) {
	o, out := pairRun(t, 2, 1000, nil)
	ps := readPairs(t, out)
	if len(ps) != 2*28 {
		t.Fatalf("%d pairs, want 2 pods x 28", len(ps))
	}
	decks := seatOf(t, out)
	seen := map[string]bool{}
	drawn := map[int][]uint64{}
	for i, p := range ps {
		pod := decks[p.A][0]
		if pod != decks[p.B][0] {
			t.Fatalf("pair %d %s-%s crosses pods", i, p.A, p.B)
		}
		k := []string{p.A, p.B}
		sort.Strings(k)
		if seen[k[0]+"|"+k[1]] {
			t.Fatalf("pod %d pairs %s twice", pod, k[0]+"|"+k[1])
		}
		seen[k[0]+"|"+k[1]] = true
		seed, err := parseSeed(p)
		if err != nil {
			t.Fatalf("pair %d: %v", i, err)
		}
		drawn[pod] = append(drawn[pod], seed)
	}
	for pod := 0; pod < 2; pod++ {
		if len(drawn[pod]) != 28 {
			t.Fatalf("pod %d drew %d pairs, want 28", pod, len(drawn[pod]))
		}
		// The seeds are the pod's stream, in pair order.
		rng := rand.New(rand.NewPCG(o.seed, saltPairs<<32|uint64(uint32(pod))))
		for g, want := range drawn[pod] {
			if got := rng.Uint64(); got != want {
				t.Fatalf("pod %d pair %d seed %d, want %d", pod, g, want, got)
			}
		}
	}
	m := readManifest(t, out)
	if m.Pairing != "roundrobin" || m.Pairs != 56 || m.GamesPerPair != 1 || m.PoolSplit != "all" || m.PairsPerPod != 0 {
		t.Fatalf("manifest pairing fields = %+v", m)
	}

	o, out = pairRun(t, 2, 1000, func(o *options) { o.pairing, o.pairsN, o.gamesN = "random", 5, 2 })
	ps = readPairs(t, out)
	if len(ps) != 2*5*2 {
		t.Fatalf("%d random pairs, want 2 pods x 5 x 2 games", len(ps))
	}
	seen = map[string]bool{}
	for i := 0; i < len(ps); i += 2 {
		a, b := ps[i], ps[i+1]
		if a.A != b.A || a.B != b.B {
			t.Fatalf("games %d and %d are not one pair: %s-%s vs %s-%s", i, i+1, a.A, a.B, b.A, b.B)
		}
		if a.SeedText == b.SeedText {
			t.Fatalf("pair %s-%s played two games on seed %s", a.A, a.B, a.SeedText)
		}
		k := []string{a.A, a.B}
		sort.Strings(k)
		if seen[k[0]+"|"+k[1]] {
			t.Fatalf("pod drew the pair %s twice", k[0]+"|"+k[1])
		}
		seen[k[0]+"|"+k[1]] = true
	}
	m = readManifest(t, out)
	if m.Pairing != "random" || m.PairsPerPod != 5 || m.Pairs != 20 {
		t.Fatalf("random manifest pairing fields = %+v", m)
	}

	_, out = pairRun(t, 2, 1000, func(o *options) { o.pairing = "cross" })
	ps = readPairs(t, out)
	if len(ps) != 8 {
		t.Fatalf("%d cross pairs, want 8 (2 pods x 8 seats, 1 pod of pairs)", len(ps))
	}
	decks = seatOf(t, out)
	for i, p := range ps {
		if decks[p.A][0] != i/seats || decks[p.B][0] != i/seats+1 {
			t.Fatalf("cross pair %d %s-%s: pods %v and %v", i, p.A, p.B, decks[p.A], decks[p.B])
		}
		if decks[p.A][1] != decks[p.B][1] {
			t.Fatalf("cross pair %d %s-%s pairs seats %d and %d", i, p.A, p.B, decks[p.A][1], decks[p.B][1])
		}
	}
}

// parseSeed reads a pairs.tsv seed column.
func parseSeed(p oracle.PairSpec) (uint64, error) {
	return strconv.ParseUint(p.SeedText, 10, 64)
}

// TestDeckpoolPoolTSVReadable: pool.tsv is the five-column file dirac's
// oracle.ReadPool reads, one row per deck written, split the -split value
// on every row, and dplay's deck lookup (<decks>/<deck>.json) resolves
// every row's name.
func TestDeckpoolPoolTSVReadable(t *testing.T) {
	_, out := pairRun(t, 1, 1000, func(o *options) { o.split = "train" })
	raw, err := os.ReadFile(filepath.Join(out, "pool.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if lines[0] != "deck\tsplit\tcolors\tmain_colors\tcards" {
		t.Fatalf("header %q", lines[0])
	}
	if len(lines) != 1+8 {
		t.Fatalf("%d pool.tsv lines, want header + 8 decks", len(lines))
	}
	idx := readIndex(t, out)
	byID := map[string]bool{}
	for _, e := range idx {
		byID[e.ID] = true
		if _, err := os.Stat(filepath.Join(out, e.File)); err != nil {
			t.Fatalf("%s: %v", e.File, err)
		}
	}
	for i, l := range lines[1:] {
		v := strings.Split(l, "\t")
		if len(v) != 5 {
			t.Fatalf("row %d %q: %d fields, want 5", i, l, len(v))
		}
		if v[1] != "train" || v[2] == "" || v[3] == "" || v[4] != "40" {
			t.Fatalf("row %d: split %q colors %q main %q cards %q", i, v[1], v[2], v[3], v[4])
		}
		if !byID[v[0]] {
			t.Fatalf("row %d names unwritten deck %q", i, v[0])
		}
	}
	rows, err := oracle.ReadPool(filepath.Join(out, "pool.tsv"), "train")
	if err != nil || len(rows) != 8 {
		t.Fatalf("ReadPool train: %d rows, %v", len(rows), err)
	}
	if rows, err := oracle.ReadPool(filepath.Join(out, "pool.tsv"), "eval"); err != nil || len(rows) != 0 {
		t.Fatalf("ReadPool eval: %d rows, %v", len(rows), err)
	}
	for _, r := range rows {
		if r.Colors == "" || strings.ContainsAny(r.Colors, "\t ") {
			t.Fatalf("row %s: colors %q", r.Name, r.Colors)
		}
	}
}

// TestDeckpoolDeterministic: two runs of the same pairing flags write
// byte-identical pool.tsv and pairs.tsv, and another seed writes another
// pairs.tsv.
func TestDeckpoolDeterministic(t *testing.T) {
	_, out1 := pairRun(t, 1, 1000, func(o *options) { o.pairing, o.pairsN = "random", 6 })
	_, out2 := pairRun(t, 1, 1000, func(o *options) { o.pairing, o.pairsN = "random", 6 })
	// The precondition the byte comparison depends on: the run drew its 6
	// pairs (6 data lines after the comment header).
	b, err := os.ReadFile(filepath.Join(out1, "pairs.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	data := 0
	for _, l := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
		if !strings.HasPrefix(l, "#") {
			data++
		}
	}
	if data != 6 {
		t.Fatalf("pairs.tsv holds %d data lines, want 6 pairs", data)
	}
	for _, f := range []string{"pool.tsv", "pairs.tsv"} {
		a, err := os.ReadFile(filepath.Join(out1, f))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(out2, f))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a, b) {
			t.Errorf("%s differs between two runs of the same flags:\n%s\n%s", f, a, b)
		}
	}
	_, out3 := pairRun(t, 1, 1001, func(o *options) { o.pairing, o.pairsN = "random", 6 })
	a, aerr := os.ReadFile(filepath.Join(out1, "pairs.tsv"))
	if aerr != nil {
		t.Fatal(aerr)
	}
	b, berr := os.ReadFile(filepath.Join(out3, "pairs.tsv"))
	if berr != nil {
		t.Fatal(berr)
	}
	if bytes.Equal(a, b) {
		t.Error("seed 1001 wrote seed 1000's pairs.tsv")
	}
}
