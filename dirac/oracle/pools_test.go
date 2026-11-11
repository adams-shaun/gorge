package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestTreeDigest pins the digest rule against the shell definition; B.json
// sorting before a.json is the byte-order proof.
func TestTreeDigest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"decks/B.json":  "{\"name\":\"B\"}\n",
		"decks/a.json":  "{\"name\":\"a\"}\n",
		"pool.tsv":      "deck\tsplit\tcolors\tmain_colors\tcards\nB\ttrain\tW\tW\t40\n",
		"pool.manifest": "m\n",
	})
	const want = "b37076c28fcce1d1a350329f483a29ece274912b5f9d31e63c5b4e48d82c761f"
	got, err := TreeDigest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("digest %s, want %s", got, want)
	}
	writeTree(t, dir, map[string]string{"pool.manifest": "edited\n"})
	if g, _ := TreeDigest(dir); g != want {
		t.Errorf("editing pool.manifest changed the digest: %s", g)
	}
	writeTree(t, dir, map[string]string{"decks/a.json": "{\"name\":\"a\"}\nx"})
	if g, _ := TreeDigest(dir); g == want {
		t.Error("appending a byte to a deck did not change the digest")
	}
}

func TestPoolRegistry(t *testing.T) {
	// not parallel: t.Setenv
	t.Setenv("DIRAC_POOL_BLB", "")
	t.Setenv("DIRAC_POOL_FDN", "")
	t.Setenv("KORACLE_BLB_DECKS", "")
	poolsRoot := t.TempDir()
	t.Setenv("GORGE_POOLS", poolsRoot)
	var names []string
	for _, p := range Pools() {
		names = append(names, p.Name)
	}
	if got := strings.Join(names, " "); got != "fdn blb pauper cmd5 standard-meta" {
		t.Fatalf("pools %q, want \"fdn blb pauper cmd5 standard-meta\"", got)
	}
	if _, err := PoolByName("dsk"); err == nil || !strings.Contains(err.Error(), "fdn, blb, pauper, cmd5, standard-meta") {
		t.Errorf("PoolByName(dsk) error %v, want one listing fdn, blb, pauper, cmd5, standard-meta", err)
	}
	if p, err := PoolByName("standard-meta"); err != nil || p.Decks != 60 || p.Tree == "" {
		t.Errorf("PoolByName(standard-meta) = %+v, %v, want 60 decks and a tree digest", p, err)
	}
	for _, name := range names {
		if p, _ := PoolByName(name); p.Inputs != filepath.Join(poolsRoot, name) {
			t.Errorf("%s inputs %q, want it under %q", name, p.Inputs, poolsRoot)
		}
	}
	t.Setenv("KORACLE_BLB_DECKS", "/legacy")
	if p, _ := PoolByName("blb"); p.Inputs != "/legacy" {
		t.Errorf("KORACLE_BLB_DECKS: inputs %q", p.Inputs)
	}
	dir := t.TempDir()
	t.Setenv("DIRAC_POOL_BLB", dir)
	if p, _ := PoolByName("blb"); p.Inputs != dir {
		t.Errorf("DIRAC_POOL_BLB: inputs %q, want %q", p.Inputs, dir)
	}
}

// TestPoolsRootPrecedence: a per-pool env beats GORGE_POOLS beats the go.mod
// walk, and with none of them nothing resolves.
func TestPoolsRootPrecedence(t *testing.T) {
	// not parallel: t.Setenv, t.Chdir
	t.Setenv("DIRAC_POOL_BLB", "")
	t.Setenv("DIRAC_POOL_FDN", "")
	t.Setenv("KORACLE_BLB_DECKS", "")
	t.Setenv("GORGE_POOLS", "")

	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// Above any go.mod: nothing resolves, and NeedDecks names the env.
	bare := filepath.Join(tmp, "bare")
	writeTree(t, bare, map[string]string{"x": ""})
	t.Chdir(bare)
	if got := PoolsRoot(); got != "" {
		t.Fatalf("no env, no go.mod: PoolsRoot %q, want \"\" (is there a go.mod above %s?)", got, bare)
	}
	if got := PoolInputs("fdn"); got != "" {
		t.Errorf("PoolInputs %q, want \"\"", got)
	}
	if got := PoolDecks("fdn"); got != "" {
		t.Errorf("PoolDecks %q, want \"\"", got)
	}
	if err := NeedDecks(PoolDecks("fdn")); err == nil || !strings.Contains(err.Error(), "GORGE_POOLS") {
		t.Errorf("NeedDecks: %v, want an error naming GORGE_POOLS", err)
	}

	// The walk: from a subdirectory up to the module root.
	mod := filepath.Join(tmp, "mod")
	writeTree(t, mod, map[string]string{"go.mod": "module x\n", "a/b/x": ""})
	t.Chdir(filepath.Join(mod, "a", "b"))
	if got, want := PoolsRoot(), filepath.Join(mod, ".pools"); got != want {
		t.Errorf("walk: PoolsRoot %q, want %q", got, want)
	}
	if got, want := PoolDecks("fdn"), filepath.Join(mod, ".pools", "fdn", "decks"); got != want {
		t.Errorf("walk: PoolDecks %q, want %q", got, want)
	}

	// GORGE_POOLS beats the walk.
	t.Setenv("GORGE_POOLS", "/env")
	if got := PoolsRoot(); got != "/env" {
		t.Errorf("env: PoolsRoot %q", got)
	}
	if got, want := PoolInputs("blb"), filepath.Join("/env", "blb"); got != want {
		t.Errorf("env: PoolInputs %q, want %q", got, want)
	}

	// A per-pool env beats GORGE_POOLS, the pool's own name first.
	t.Setenv("KORACLE_BLB_DECKS", "/legacy")
	if got := PoolInputs("blb"); got != "/legacy" {
		t.Errorf("KORACLE_BLB_DECKS: PoolInputs %q", got)
	}
	if got, want := PoolInputs("fdn"), filepath.Join("/env", "fdn"); got != want {
		t.Errorf("KORACLE_BLB_DECKS reached fdn: %q, want %q", got, want)
	}
	t.Setenv("DIRAC_POOL_BLB", "/one")
	if got := PoolInputs("blb"); got != "/one" {
		t.Errorf("DIRAC_POOL_BLB: PoolInputs %q", got)
	}
	if got, want := PoolDecks("blb"), filepath.Join("/one", "decks"); got != want {
		t.Errorf("DIRAC_POOL_BLB: PoolDecks %q, want %q", got, want)
	}
}

// TestTreeDigestIgnoresReadme: only decks/** and the top-level pool.tsv are in
// scope, because a pool's notes carry things a public pin must not cover.
func TestTreeDigestIgnoresReadme(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"decks/a.json": "{\"name\":\"a\"}\n",
		"pool.tsv":     "deck\tsplit\tcolors\tmain_colors\tcards\na\ttrain\tW\tW\t40\n",
	})
	want, err := TreeDigest(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"README.md", "summary.txt", "ATTRIBUTION.txt", "pool.manifest", "other/x.json"} {
		writeTree(t, dir, map[string]string{name: "notes\n"})
		if got, _ := TreeDigest(dir); got != want {
			t.Errorf("adding %s changed the digest: %s, want %s", name, got, want)
		}
	}
	writeTree(t, dir, map[string]string{"pool.tsv": "changed\n"})
	changedTSV, _ := TreeDigest(dir)
	if changedTSV == want {
		t.Error("changing pool.tsv did not change the digest")
	}
	writeTree(t, dir, map[string]string{"decks/a.json": "{\"name\":\"b\"}\n"})
	if got, _ := TreeDigest(dir); got == changedTSV {
		t.Error("changing a deck did not change the digest")
	}
	writeTree(t, dir, map[string]string{"decks/sub/b.json": "{}\n"})
	withSub, _ := TreeDigest(dir)
	writeTree(t, dir, map[string]string{"decks/sub/b.json": "{ }\n"})
	if got, _ := TreeDigest(dir); got == withSub {
		t.Error("changing a nested deck did not change the digest")
	}
}

// TestTreeDigestFollowsRootSymlink: a pool farm's entries are symlinks, and
// filepath.WalkDir does not descend a symlinked root. Hashing nothing would pin
// the empty tree.
func TestTreeDigestFollowsRootSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	writeTree(t, real, map[string]string{"decks/a.json": "{}\n", "pool.tsv": "h\n"})
	want, err := TreeDigest(real)
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("real", link); err != nil {
		t.Fatal(err)
	}
	if got, err := TreeDigest(link); err != nil || got != want {
		t.Errorf("through a symlink: %s %v, want %s", got, err, want)
	}
}

// TestPoolFrozen: each registered pool's tree on this box still matches its
// pinned digest. CI has no pool trees and skips.
func TestPoolFrozen(t *testing.T) {
	t.Parallel()

	for _, want := range Pools() {
		t.Run(want.Name, func(t *testing.T) {
			t.Parallel()
			p, _ := PoolByName(want.Name)
			tsv := filepath.Join(p.Inputs, "pool.tsv")
			raw, err := os.ReadFile(tsv)
			if err != nil {
				t.Skipf("no pool at %s (set GORGE_POOLS)", tsv)
			}
			got, err := TreeDigest(p.Inputs)
			if err != nil {
				t.Fatal(err)
			}
			if got != want.Tree {
				t.Errorf("pool %s: tree digest %s, want %s: a re-derivation is a reviewed digest change", p.Name, got, want.Tree)
			}
			rows := strings.Count(strings.TrimRight(string(raw), "\n"), "\n") // minus the header
			if rows != want.Decks {
				t.Errorf("pool %s: pool.tsv has %d decks, want %d", p.Name, rows, want.Decks)
			}
			files, err := filepath.Glob(filepath.Join(p.Inputs, "decks", "*.json"))
			if err != nil {
				t.Fatal(err)
			}
			if len(files) != want.Decks {
				t.Errorf("pool %s: %d decks/*.json, want %d", p.Name, len(files), want.Decks)
			}
		})
	}
}
