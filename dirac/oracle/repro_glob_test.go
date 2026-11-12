package oracle

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// reproFiles is testdata/repro's recordings, sorted by path so a run's
// subtests keep a fixed order.
func reproFiles(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", "repro", "*.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		t.Fatal("testdata/repro has no recordings")
	}
	return paths
}

func reproName(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".repro.json")
}

// globSet loads a repro's pool. standard-meta is a built set; custom is the
// table built from the repro's own decks (reproSet); every other pool is
// synthetic.
func globSet(t *testing.T, pool string, r *Repro) cardset.Set {
	t.Helper()
	if pool == "standard-meta" {
		return cardsettest.Set(t, pool)
	}
	return reproSet(t, pool, r)
}

// replayReproValue is replayReproRes over an already-decoded repro, on the
// surface the repro records and 200 decisions past its script.
func replayReproValue(t *testing.T, reg *cards.Registry, set cardset.Set, r *Repro) Result {
	t.Helper()
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.Config.GorgeAutopaySurface = r.Surface != "full"
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	res := Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
	if res.Div != nil {
		t.Fatalf("divergence at %d of %d: %s %s %v", res.Div.At, len(r.Actions), res.Div.What, res.Div.Field, res.Div.Diff)
	}
	return res
}

// stdReproNamed replays one named recording on its own recorded pool and
// surface, for a test whose name states the rule the game pins.
func stdReproNamed(t *testing.T, name string) {
	t.Helper()
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	res := replayReproValue(t, synthReg(t), globSet(t, r.Pool, r), r)
	if !r.Stall && strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("kernel stalled: %s", res.Stall)
	}
	if r.Stall && !strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("expected a declared kernel stall, got %q", res.Stall)
	}
}

// stdReprosGlobPools replays every recording whose pool is one of pools: a
// new recording needs a JSON file, no Go edit.
func stdReprosGlobPools(t *testing.T, pools ...string) {
	t.Helper()
	want := make(map[string]bool, len(pools))
	for _, p := range pools {
		want[p] = true
	}
	reg := synthReg(t)
	sets := make(map[string]cardset.Set)
	for _, path := range reproFiles(t) {
		r, err := ReadRepro(path)
		if err != nil {
			t.Fatal(err)
		}
		if !want[r.Pool] {
			continue
		}
		set, ok := sets[r.Pool]
		if !ok || r.Pool == "custom" {
			// custom is the table its own decks need, so it is per repro.
			set = globSet(t, r.Pool, r)
			sets[r.Pool] = set
		}
		name := reproName(path)
		t.Run(name, func(t *testing.T) {
			res := replayReproValue(t, reg, set, r)
			if !r.Stall && strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Fatalf("kernel stalled: %s", res.Stall)
			}
			if r.Stall && !strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Fatalf("expected a declared kernel stall, got %q", res.Stall)
			}
		})
	}
}

// stdReprosGlob is stdReprosGlobPools for one pool.
func stdReprosGlob(t *testing.T, pool string) {
	t.Helper()
	stdReprosGlobPools(t, pool)
}

// TestReproGlobMatchesTable is the one-time equality check for the move from
// the shared Go tables to metadata inside each repro: every name the tables
// held still has a recording, and the corpus has at least the pre-change 969
// rows. New recordings may only add to the set.
func TestReproGlobMatchesTable(t *testing.T) {
	paths := reproFiles(t)
	if len(paths) < 969 {
		t.Fatalf("%d recordings, want at least the pre-change 969", len(paths))
	}
	table := make(map[string]bool)
	b, err := os.ReadFile(filepath.Join("testdata", "repro_table_names.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line != "" {
			table[line] = true
		}
	}
	if len(table) < 967 {
		t.Fatalf("%d table names, want at least 967", len(table))
	}
	for name := range table {
		path := filepath.Join("testdata", "repro", name+".repro.json")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("table row %q has no recording: %v", name, err)
		}
	}
	for _, path := range paths {
		r, err := ReadRepro(path)
		if err != nil {
			t.Fatal(err)
		}
		if r.Pool == "" {
			t.Errorf("%s: no pool", path)
		}
		if r.Surface == "" {
			t.Errorf("%s: no surface", path)
		}
		if table[reproName(path)] && r.Pool == "one-off" {
			t.Errorf("%s: a table row was left without its pool", path)
		}
	}
}
