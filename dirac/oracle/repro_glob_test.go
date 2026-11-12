package oracle

import (
	"os"
	"path/filepath"
	"regexp"
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

// replayDecodedRes is replayReproValue over an already-decoded repro, on the
// surface the repro records and 200 decisions past its script, without its
// divergence fatal: a caller that wants the divergence (a mutation test
// needing one) reads res.Div itself.
func replayDecodedRes(t *testing.T, reg *cards.Registry, set cardset.Set, r *Repro) Result {
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
	return Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
}

// replayReproValue fails on the replay's divergence: a recorded game must
// replay clean.
func replayReproValue(t *testing.T, reg *cards.Registry, set cardset.Set, r *Repro) Result {
	t.Helper()
	res := replayDecodedRes(t, reg, set, r)
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
	stallCheck(t, r, replayReproValue(t, synthReg(t), globSet(t, r.Pool, r), r), false)
}

// stallCheck fails a replay whose kernel stall disagrees with the recording:
// a recording that declares one must end in it. A strict check also fails any
// other stall, declared capacity stalls included.
func stallCheck(t *testing.T, r *Repro, res Result, strict bool) {
	t.Helper()
	switch {
	case r.Stall && !strings.HasPrefix(res.Stall, KernelStallPrefix):
		t.Fatalf("expected a declared kernel stall, got %q", res.Stall)
	case !r.Stall && strict && res.Stall != "":
		t.Fatalf("the game stalled: %s", res.Stall)
	case !r.Stall && strings.HasPrefix(res.Stall, KernelStallPrefix):
		t.Fatalf("kernel stalled: %s", res.Stall)
	}
}

// globPools replays every recording whose pool is one of pools, one subtest
// each. A pool with no recording fails: a mis-pooled batch must be loud.
//
// strict names the recordings that must end in no stall at all.
func globPools(t *testing.T, strict map[string]bool, pools ...string) {
	t.Helper()
	want := make(map[string]int, len(pools))
	for _, p := range pools {
		want[p] = 0
	}
	reg := synthReg(t)
	sets := make(map[string]cardset.Set)
	for _, path := range reproFiles(t) {
		r, err := ReadRepro(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := want[r.Pool]; !ok {
			continue
		}
		want[r.Pool]++
		set, ok := sets[r.Pool]
		if !ok || r.Pool == "custom" {
			// custom is the table its own decks need, so it is per repro.
			set = globSet(t, r.Pool, r)
			sets[r.Pool] = set
		}
		t.Run(reproName(path), func(t *testing.T) {
			stallCheck(t, r, replayReproValue(t, reg, set, r), strict[reproName(path)])
		})
	}
	for _, p := range pools {
		if want[p] == 0 {
			t.Errorf("pool %q matched no recording in testdata/repro", p)
		}
	}
}

// stdReprosGlobPools replays every recording whose pool is one of pools: a
// new recording needs a JSON file, no Go edit.
func stdReprosGlobPools(t *testing.T, pools ...string) {
	t.Helper()
	globPools(t, nil, pools...)
}

// stdReprosGlob is stdReprosGlobPools for one pool.
func stdReprosGlob(t *testing.T, pool string) {
	t.Helper()
	globPools(t, nil, pool)
}

// stdReprosGlobStrict is stdReprosGlob where the named recordings end in no
// stall of any kind; the pool's other recordings may declare a capacity stall.
func stdReprosGlobStrict(t *testing.T, pool string, strict ...string) {
	t.Helper()
	m := make(map[string]bool, len(strict))
	for _, n := range strict {
		m[n] = true
	}
	globPools(t, m, pool)
}

// globbedPools is every pool a test in this package replays by glob, read from
// the tests' own sources.
func globbedPools(t *testing.T) map[string]bool {
	t.Helper()
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	quoted := regexp.MustCompile(`"([a-z][a-z-]*)"`)
	pools := make(map[string]bool)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(b), "\n") {
			if !strings.Contains(line, "stdReprosGlob") || strings.Contains(line, "func ") {
				continue
			}
			for _, m := range quoted.FindAllStringSubmatch(line, -1) {
				pools[m[1]] = true
			}
		}
	}
	return pools
}

// TestReproGlobMatchesTable is the one-time equality check for the move from
// the shared Go tables to metadata inside each repro. The snapshot holds every
// removed row (name and pool): each still has a recording on that pool, and a
// test globs that pool, so no row's game went unrun. New recordings may only
// add to the set.
func TestReproGlobMatchesTable(t *testing.T) {
	paths := reproFiles(t)
	if len(paths) < 969 {
		t.Fatalf("%d recordings, want at least the pre-change 969", len(paths))
	}
	table := make(map[string]string)
	b, err := os.ReadFile(filepath.Join("testdata", "repro_table_names.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		name, pool, ok := strings.Cut(line, " ")
		if !ok {
			t.Fatalf("snapshot line %q has no pool", line)
		}
		table[name] = pool
	}
	if len(table) < 1005 {
		t.Fatalf("%d table names, want at least 1005 (the fixes, stdsets, msh and trc rows)", len(table))
	}
	globbed := globbedPools(t)
	for name, pool := range table {
		r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
		if err != nil {
			t.Errorf("table row %q has no recording: %v", name, err)
			continue
		}
		if r.Pool != pool {
			t.Errorf("table row %q was pool %q, its recording says %q", name, pool, r.Pool)
		}
		if !globbed[r.Pool] {
			t.Errorf("table row %q: no test globs pool %q, so its game is never replayed", name, r.Pool)
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
	}
}

// stdReproOtherSurface replays a recording on a surface other than the one it
// records, for a game the old tables ran on a surface its own test does not.
func stdReproOtherSurface(t *testing.T, name, recorded, other string) {
	t.Helper()
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Surface != recorded {
		t.Fatalf("recorded on %q, want %q: this test is the other surface", r.Surface, recorded)
	}
	r.Surface = other
	stallCheck(t, r, replayReproValue(t, synthReg(t), globSet(t, r.Pool, r), r), false)
}

// The std-tdm table ran this game on both surfaces; the recording is on full.
func TestRegisteredEffectRememberedScopeAutopayLockstep(t *testing.T) {
	t.Parallel()
	stdReproOtherSurface(t, "registered_effect_remembered_scope", "full", "autopay")
}

// The std-ecl table ran this game on autopay; its own test replays the
// recorded full surface.
func TestStaticHexproofOwncoloursAutopayLockstep(t *testing.T) {
	t.Parallel()
	stdReproOtherSurface(t, "static_hexproof_own_colours_red_spell", "full", "autopay")
}
