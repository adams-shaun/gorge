package cardset

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge"
)

const testDigest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func testKey() string { return testDigest[:16] + "-" + gorge.TableSourceHash()[:16] }

// isolate clears the environment's corpus and leaves the cwd somewhere with no
// .cards above it (the temp root is not inside a corpus checkout).
func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("GORGE_CARDS", "")
	t.Setenv("DIRAC_TABLES", "")
	t.Chdir(t.TempDir())
	if _, err := CorpusDir(); !errors.Is(err, ErrNoCorpus) {
		t.Skipf("the temp directory has a corpus above it: %v", err)
	}
}

func writeFile(t *testing.T, path string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// makeCorpus writes a corpus with a cards.lock.
func makeCorpus(t *testing.T, dir string) string {
	t.Helper()
	writeFile(t, filepath.Join(dir, "cards.lock"), []byte(`{"commit":"abc","digest":"`+testDigest+`"}`))
	return dir
}

// writeTables writes a key directory holding pools under key.
func writeTables(t *testing.T, dir, key string, pools ...string) {
	t.Helper()
	var rows []ManifestRow
	for _, p := range pools {
		d := sampleData()
		d.Name = p
		b := EncodeTable(d, key)
		sum, err := TableSum(b)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(dir, p+".ktab"), b)
		rows = append(rows, ManifestRow{Pool: p, NumCards: d.NumCards, NumTokens: d.NumTokens, Sum: sum})
	}
	writeFile(t, filepath.Join(dir, ManifestFile), EncodeManifest(rows))
}

func TestLoadErrors(t *testing.T) {
	t.Run("no corpus", func(t *testing.T) {
		isolate(t)
		_, err := TablesDir()
		if !errors.Is(err, ErrNoCorpus) {
			t.Errorf("TablesDir: %v", err)
		}
		if _, err := load("x"); !errors.Is(err, ErrNoCorpus) {
			t.Errorf("load: %v", err)
		}
	})
	t.Run("corpus with no cards.lock", func(t *testing.T) {
		isolate(t)
		dir := t.TempDir()
		t.Setenv("GORGE_CARDS", dir)
		if _, err := TablesDir(); !errors.Is(err, ErrNoCorpus) {
			t.Errorf("GORGE_CARDS: %v", err)
		}
		t.Setenv("GORGE_CARDS", "")
		if err := os.Mkdir(".cards", 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := TablesDir(); !errors.Is(err, ErrNoCorpus) {
			t.Errorf(".cards: %v", err)
		}
	})
	t.Run("dangling .cards link", func(t *testing.T) {
		isolate(t)
		if err := os.Symlink(filepath.Join(t.TempDir(), "gone"), ".cards"); err != nil {
			t.Fatal(err)
		}
		if _, err := TablesDir(); !errors.Is(err, ErrNoCorpus) {
			t.Errorf("%v", err)
		}
	})
	t.Run("corpus with no key directory", func(t *testing.T) {
		isolate(t)
		corpus := makeCorpus(t, t.TempDir())
		t.Setenv("GORGE_CARDS", corpus)
		_, err := TablesDir()
		if !errors.Is(err, ErrNoTables) {
			t.Fatalf("TablesDir: %v", err)
		}
		for _, want := range []string{testKey(), filepath.Join(corpus, TablesSubdir), "make dirac-tables"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not say %q", err, want)
			}
		}
		if _, err := load("x"); !errors.Is(err, ErrNoTables) {
			t.Errorf("load: %v", err)
		}
	})
	t.Run("a corpus swap leaves the old key behind", func(t *testing.T) {
		isolate(t)
		corpus := makeCorpus(t, t.TempDir())
		t.Setenv("GORGE_CARDS", corpus)
		writeTables(t, filepath.Join(corpus, TablesSubdir, "ffffffffffffffff-"+gorge.TableSourceHash()[:16]), "stale", "t")
		if _, err := TablesDir(); !errors.Is(err, ErrNoTables) {
			t.Errorf("stale key served: %v", err)
		}
	})
	t.Run("malformed lock", func(t *testing.T) {
		isolate(t)
		corpus := t.TempDir()
		t.Setenv("GORGE_CARDS", corpus)
		for _, lock := range []string{`not json`, `{}`, `{"digest":"short"}`, `{"digest":"NOTHEXNOTHEXNOTHEXNOTHEX"}`} {
			writeFile(t, filepath.Join(corpus, "cards.lock"), []byte(lock))
			_, err := TablesDir()
			if err == nil || errors.Is(err, ErrNoCorpus) || errors.Is(err, ErrNoTables) || !strings.Contains(err.Error(), "cards.lock") {
				t.Errorf("lock %q: %v", lock, err)
			}
		}
	})

	badFile := func(name string, damage func(b []byte) []byte, want string) {
		t.Run(name, func(t *testing.T) {
			isolate(t)
			corpus := makeCorpus(t, t.TempDir())
			t.Setenv("GORGE_CARDS", corpus)
			dir := filepath.Join(corpus, TablesSubdir, testKey())
			writeTables(t, dir, testKey(), "t")
			path := filepath.Join(dir, "t.ktab")
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, damage(b))
			_, err = load("t")
			if err == nil {
				t.Fatal("damaged table loaded")
			}
			if errors.Is(err, ErrNoCorpus) || errors.Is(err, ErrNoTables) {
				t.Errorf("a damaged table must be a plain error, so a test fails: %v", err)
			}
			if !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not name %s and %q", err, path, want)
			}
		})
	}
	badFile("header mismatch", func(b []byte) []byte { b[0] ^= 1; return b }, "bad magic")
	badFile("checksum mismatch", func(b []byte) []byte { b[len(b)-1] ^= 1; return b }, "checksum")
	badFile("table of another key", func([]byte) []byte {
		d := sampleData()
		d.Name = "t"
		return EncodeTable(d, "another-key")
	}, "another-key")
	badFile("table the manifest did not list", func([]byte) []byte {
		d := sampleData()
		d.Name = "t"
		d.NumIDs++
		return EncodeTable(d, testKey())
	}, "manifest says")
	badFile("table of another pool", func(b []byte) []byte {
		d := sampleData()
		d.Name = "other"
		return EncodeTable(d, testKey())
	}, "manifest says") // the manifest's checksum sees it first; the name check backs it

	t.Run("pool not in the manifest", func(t *testing.T) {
		isolate(t)
		corpus := makeCorpus(t, t.TempDir())
		t.Setenv("GORGE_CARDS", corpus)
		writeTables(t, filepath.Join(corpus, TablesSubdir, testKey()), testKey(), "alpha", "beta")
		_, err := load("gamma")
		if err == nil || errors.Is(err, ErrNoTables) || errors.Is(err, ErrNoCorpus) {
			t.Fatalf("got %v", err)
		}
		for _, want := range []string{"gamma", "alpha", "beta"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not say %q", err, want)
			}
		}
	})
	t.Run("missing table file", func(t *testing.T) {
		isolate(t)
		corpus := makeCorpus(t, t.TempDir())
		t.Setenv("GORGE_CARDS", corpus)
		dir := filepath.Join(corpus, TablesSubdir, testKey())
		writeTables(t, dir, testKey(), "t")
		if err := os.Remove(filepath.Join(dir, "t.ktab")); err != nil {
			t.Fatal(err)
		}
		if _, err := load("t"); err == nil || errors.Is(err, ErrNoTables) {
			t.Errorf("got %v", err)
		}
	})
}

func TestTablesDirOrder(t *testing.T) {
	isolate(t)

	// 3: the first .cards walking up from the cwd.
	top := t.TempDir()
	walked := makeCorpus(t, filepath.Join(top, ".cards"))
	walkedDir := filepath.Join(walked, TablesSubdir, testKey())
	writeTables(t, walkedDir, testKey(), "t")
	deep := filepath.Join(top, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(deep)
	assertDir(t, "walk up", walkedDir)

	// 2: $GORGE_CARDS over the walk.
	env := makeCorpus(t, t.TempDir())
	envDir := filepath.Join(env, TablesSubdir, testKey())
	writeTables(t, envDir, testKey(), "t")
	t.Setenv("GORGE_CARDS", env)
	assertDir(t, "GORGE_CARDS", envDir)

	// 1: $DIRAC_TABLES/<key> over both.
	pinned := t.TempDir()
	pinnedDir := filepath.Join(pinned, testKey())
	writeTables(t, pinnedDir, testKey(), "t")
	t.Setenv("DIRAC_TABLES", pinned)
	assertDir(t, "DIRAC_TABLES", pinnedDir)

	// DIRAC_TABLES without this key falls through; first match wins, it is no veto.
	t.Setenv("DIRAC_TABLES", t.TempDir())
	assertDir(t, "DIRAC_TABLES without the key", envDir)
}

func assertDir(t *testing.T, what, want string) {
	t.Helper()
	wantResolved, err := filepath.EvalSymlinks(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := TablesDir()
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	if got != wantResolved {
		t.Errorf("%s: got %s, want %s", what, got, wantResolved)
	}
}

// A symlinked corpus resolves before opening, so go test does not track the file
// and the artifact never hears of the link's path.
func TestTablesDirResolvesSymlinks(t *testing.T) {
	isolate(t)
	real := makeCorpus(t, t.TempDir())
	dir := filepath.Join(real, TablesSubdir, testKey())
	writeTables(t, dir, testKey(), "t")
	if err := os.Symlink(real, ".cards"); err != nil {
		t.Fatal(err)
	}
	assertDir(t, "symlinked .cards", dir)
	s, err := load("t")
	if err != nil {
		t.Fatal(err)
	}
	if s.Name() != "t" {
		t.Errorf("loaded %q", s.Name())
	}
}

// A deployed binary has DIRAC_TABLES and no corpus, so no lock to read: it serves
// the directory for its own sources, refuses to guess between two, and a directory
// for other sources is ErrNoTables, never a skip and never served.
func TestDiracTablesWithoutCorpus(t *testing.T) {
	isolate(t)
	pinned := t.TempDir()
	t.Setenv("DIRAC_TABLES", pinned)
	if _, err := TablesDir(); !errors.Is(err, ErrNoTables) {
		t.Errorf("empty: %v", err)
	}
	writeTables(t, filepath.Join(pinned, "aaaaaaaaaaaaaaaa-bbbbbbbbbbbbbbbb"), "x", "t")
	if _, err := TablesDir(); !errors.Is(err, ErrNoTables) {
		t.Errorf("other sources: %v", err)
	}
	one := filepath.Join(pinned, testKey())
	writeTables(t, one, testKey(), "t")
	assertDir(t, "one match", one)
	if _, err := load("t"); err != nil {
		t.Errorf("load: %v", err)
	}
	writeTables(t, filepath.Join(pinned, "cccccccccccccccc-"+gorge.TableSourceHash()[:16]), "y", "t")
	_, err := TablesDir()
	if err == nil || errors.Is(err, ErrNoTables) || errors.Is(err, ErrNoCorpus) || !strings.Contains(err.Error(), "GORGE_CARDS") {
		t.Errorf("two matches: %v", err)
	}
}

// load decodes once per pool per directory, in parallel, and the same process can
// follow a change of directory.
func TestLoadCachesPerPoolAndDirectory(t *testing.T) {
	isolate(t)
	corpus := makeCorpus(t, t.TempDir())
	t.Setenv("GORGE_CARDS", corpus)
	dir := filepath.Join(corpus, TablesSubdir, testKey())
	writeTables(t, dir, testKey(), "a", "b")

	first, err := load("a")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	got := make([]Set, 16)
	errs := make([]error, 16)
	for i := range got {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i], errs[i] = load([]string{"a", "b"}[i%2])
		}()
	}
	wg.Wait()
	for i := range got {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if i%2 == 0 && got[i] != first {
			t.Errorf("load %d decoded again", i)
		}
		if want := []string{"a", "b"}[i%2]; got[i].Name() != want {
			t.Errorf("load %d returned %q", i, got[i].Name())
		}
	}

	other := makeCorpus(t, t.TempDir())
	writeTables(t, filepath.Join(other, TablesSubdir, testKey()), testKey(), "a")
	t.Setenv("GORGE_CARDS", other)
	s, err := load("a")
	if err != nil {
		t.Fatal(err)
	}
	if s == first {
		t.Error("a different directory served the old directory's set")
	}
	if _, err := load("b"); err == nil {
		t.Error("pool b exists only in the old directory")
	}
}

func TestManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	rows := []ManifestRow{{"a", 1, 0, "ab"}, {"b", 2, 3, "cd"}}
	writeFile(t, filepath.Join(dir, ManifestFile), EncodeManifest(rows))
	got, err := ReadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got) != fmt.Sprint(rows) {
		t.Errorf("got %v", got)
	}
	for _, bad := range []string{"a\t1\n", "a\tx\t0\tab\n", "a\t1\t0\tzz\n", "\t1\t0\tab\n"} {
		writeFile(t, filepath.Join(dir, ManifestFile), []byte(bad))
		if _, err := ReadManifest(dir); err == nil {
			t.Errorf("manifest %q read cleanly", bad)
		}
	}
}
