package kgen

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/adams-shaun/gorge"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// wantPools is a floor, not a count. A build finding fewer has lost pools.
const wantPools = 427

// tableCorpus is the corpus for a table build: KGEN_CARDS or the worktree's link.
func tableCorpus(t *testing.T) string {
	t.Helper()
	d := os.Getenv("KGEN_CARDS")
	if d == "" {
		var err error
		if d, err = cardset.CorpusDir(); err != nil {
			t.Skipf("no corpus (%v): link it as .cards here", err)
		}
	}
	if _, err := os.Stat(filepath.Join(d, "cardsfolder")); err != nil {
		t.Skipf("no corpus at %s (%v): link it as .cards here", d, err)
	}
	return d
}

func buildAll(t *testing.T, cards string) *tableBuild {
	t.Helper()
	tb, err := buildTables(gorge.TableSources(), cards, "testkey")
	if err != nil {
		t.Fatal(err)
	}
	if len(tb.ids) > 0 {
		t.Fatalf("building changes %d ids.tsv files, first %s: run make dirac-tables", len(tb.ids), tb.ids[0].pool)
	}
	if tb.tables < wantPools {
		t.Fatalf("%d tables, want at least %d", tb.tables, wantPools)
	}
	return tb
}

// What is published under this binary's key is what a rebuild makes. It backs the
// key, which leaves out the kernel functions kgen calls and any gorge package outside
// the embedded list: a change there alters a table without re-keying it.
func TestTablesUpToDate(t *testing.T) {
	heavyParallel(t)
	cards := tableCorpus(t)
	key, err := cardset.KeyFor(cards)
	if errors.Is(err, cardset.ErrNoCorpus) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(cards)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(resolved, cardset.TablesSubdir, key)
	if !tablesComplete(dir) {
		t.Fatalf("no tables for key %s in %s: run make dirac-tables", key, dir)
	}
	tb, err := buildTables(gorge.TableSources(), cards, key)
	if err != nil {
		t.Fatal(err)
	}
	if len(tb.ids) > 0 {
		t.Fatalf("building changes %d ids.tsv files, first %s: run make dirac-tables", len(tb.ids), tb.ids[0].pool)
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != len(tb.files) {
		t.Errorf("%s holds %d files, a build makes %d", dir, len(ents), len(tb.files))
	}
	for _, f := range tb.files {
		got, err := os.ReadFile(filepath.Join(dir, f.name))
		if err != nil {
			t.Error(err)
			continue
		}
		if !bytes.Equal(got, f.data) {
			t.Errorf("%s differs from a rebuild, under the same key: remove %s and run make dirac-tables", f.name, dir)
		}
	}
}

// Two builds, one on a single P and one through a symlinked -cards at another path,
// give the same bytes: nothing depends on scheduling or on where the corpus is.
func TestTableBuildDeterministic(t *testing.T) {
	cards := tableCorpus(t)

	old := runtime.GOMAXPROCS(1)
	one := buildAll(t, cards)
	runtime.GOMAXPROCS(old)

	abs, err := filepath.Abs(cards)
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "corpus-link")
	if err := os.Symlink(abs, link); err != nil {
		t.Fatal(err)
	}
	two := buildAll(t, link)

	if len(one.files) != len(two.files) {
		t.Fatalf("%d files vs %d", len(one.files), len(two.files))
	}
	for i := range one.files {
		a, b := one.files[i], two.files[i]
		if a.name != b.name {
			t.Fatalf("file %d: %s vs %s", i, a.name, b.name)
		}
		if !bytes.Equal(a.data, b.data) {
			t.Errorf("%s differs between the builds", a.name)
		}
	}
	t.Logf("%d tables, %d files, %d table bytes, identical", one.tables, len(one.files), one.bytes)
}

// pathLeaks names the needles in data.
func pathLeaks(data []byte, needles []string) []string {
	var out []string
	for _, n := range needles {
		if bytes.Contains(data, []byte(n)) {
			out = append(out, n)
		}
	}
	return out
}

// The poison gorge fixed in c09b82ba0: a cache that holds absolute paths from
// whichever worktree built it. No table, coverage report or manifest may hold one.
func TestTablesCarryNoPaths(t *testing.T) {
	heavyParallel(t)
	cards := tableCorpus(t)
	abs, err := filepath.Abs(cards)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	needles := []string{abs, resolved, wd, "/home/", "/tmp/", "/mnt/", ".worktrees"}

	// A control: the scan finds a planted path.
	if got := pathLeaks([]byte("x"+resolved+"/cardsfolder/a.txt"), needles); len(got) == 0 {
		t.Fatal("the scan misses a planted corpus path")
	}

	tb := buildAll(t, cards)
	var kinds [3]int
	for _, f := range tb.files {
		switch {
		case strings.HasSuffix(f.name, ".ktab"):
			kinds[0]++
		case strings.HasSuffix(f.name, ".coverage.md"):
			kinds[1]++
		case f.name == cardset.ManifestFile:
			kinds[2]++
		default:
			t.Errorf("unexpected file %s", f.name)
		}
		if got := pathLeaks(f.data, needles); len(got) > 0 {
			t.Errorf("%s holds %q", f.name, got)
		}
	}
	if kinds != [3]int{tb.tables, tb.tables, 1} {
		t.Errorf("scanned %v, want %d tables, %d reports and a manifest", kinds, tb.tables, tb.tables)
	}
	t.Logf("%d files scanned", len(tb.files))
}

// A pool whose ids.tsv would change publishes nothing: the build reports the change,
// and -tables stops at exit 3. A pool whose ids.tsv is right reports none.
func TestTablesIDsChangeStops(t *testing.T) {
	cards := tableCorpus(t)
	const card = "Llanowar Elves"
	src := fstest.MapFS{"dirac/cardset/pools/t/cards.txt": {Data: []byte("# a comment\n" + card + "\n")}}
	tb, err := buildTables(src, cards, "k")
	if err != nil {
		t.Fatal(err)
	}
	if len(tb.ids) != 1 || tb.ids[0].pool != "t" || tb.tables != 0 {
		t.Fatalf("no ids.tsv: %d changes, %d tables", len(tb.ids), tb.tables)
	}
	src["dirac/cardset/pools/t/ids.tsv"] = &fstest.MapFile{Data: tb.ids[0].data}
	tb, err = buildTables(src, cards, "k")
	if err != nil {
		t.Fatal(err)
	}
	if len(tb.ids) != 0 || tb.tables != 1 {
		t.Fatalf("settled ids.tsv: %d changes, %d tables", len(tb.ids), tb.tables)
	}
	d, _, err := cardset.DecodeTable(tb.files[0].data)
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "t" || d.Names[0] != card {
		t.Errorf("decoded %q, %v", d.Name, d.Names)
	}
}

// writeIDs refuses a tree newer than the binary.
func TestWriteIDsRefusesNewerTree(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, poolsDir, "t")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, idsFile)
	if err := os.WriteFile(p, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := fstest.MapFS{"dirac/cardset/pools/t/ids.tsv": {Data: []byte("old\n")}}
	change := []idsChange{{pool: "t", data: []byte("new\n")}}
	if err := writeIDs(root, src, change); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "new\n" {
		t.Fatalf("wrote %q", b)
	}
	// The tree has moved on; the binary still embeds "old".
	if err := writeIDs(root, src, change); err == nil || !strings.Contains(err.Error(), "rebuild kgen") {
		t.Fatalf("got %v", err)
	}
}

func completeFiles() []tableFile {
	return []tableFile{
		{"t.ktab", []byte("table")},
		{"t.coverage.md", []byte("report")},
		{cardset.ManifestFile, cardset.EncodeManifest([]cardset.ManifestRow{{Pool: "t", NumCards: 1, Sum: "ab"}})},
	}
}

func TestPublishTables(t *testing.T) {
	root := filepath.Join(t.TempDir(), cardset.TablesSubdir)
	dir, err := publishTables(root, "k1", completeFiles())
	if err != nil {
		t.Fatal(err)
	}
	if !tablesComplete(dir) {
		t.Fatal("published directory is not complete")
	}
	// A peer got there first: the temp goes, the peer's directory stays.
	if _, err := publishTables(root, "k1", completeFiles()); err != nil {
		t.Fatal(err)
	}
	ents, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 || ents[0].Name() != "k1" {
		t.Fatalf("root holds %v", ents)
	}
	// A partial directory, which no publish leaves, is not complete.
	if err := os.Remove(filepath.Join(dir, "t.coverage.md")); err != nil {
		t.Fatal(err)
	}
	if tablesComplete(dir) {
		t.Error("a directory missing a report counts as complete")
	}
}

func TestPruneTables(t *testing.T) {
	root := t.TempDir()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mkdir := func(name string, mod time.Time) {
		t.Helper()
		p := filepath.Join(root, name)
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	// k00 is the oldest. The one published has the oldest mtime of all, because a
	// peer published it first: it must survive anyway.
	mkdir("published", base.Add(-time.Hour))
	for i := range keepKeyDirs + 3 {
		mkdir(fmt.Sprintf("k%02d", i), base.Add(time.Duration(i)*time.Minute))
	}
	mkdir("kdead.tmp.1", base)
	live := "klive.tmp.2"
	mkdir(live, time.Now())
	if err := pruneTables(root, "published"); err != nil {
		t.Fatal(err)
	}
	var got []string
	ents, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		got = append(got, e.Name())
	}
	var want []string
	for i := 4; i < keepKeyDirs+3; i++ {
		want = append(want, fmt.Sprintf("k%02d", i))
	}
	want = append(want, live, "published")
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("kept %v\nwant %v", got, want)
	}
}

// runTables' second run does not lower anything: an unmodified key directory answers.
func TestRunTablesUpToDate(t *testing.T) {
	corpus := t.TempDir()
	if err := os.WriteFile(filepath.Join(corpus, "cards.lock"), []byte(`{"digest":"0123456789abcdef0123"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	key, err := cardset.KeyFor(corpus)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := publishTables(filepath.Join(corpus, cardset.TablesSubdir), key, completeFiles())
	if err != nil {
		t.Fatal(err)
	}
	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(dir, old, old); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if rc := runTables(corpus, &out, io.Discard); rc != 0 {
		t.Fatalf("exit %d", rc)
	}
	if !strings.Contains(out.String(), "up to date") || !strings.Contains(out.String(), key) {
		t.Errorf("printed %q", out.String())
	}
	fi, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !fi.ModTime().After(old) {
		t.Error("a key asked for again keeps its old mtime, so a peer's prune takes it first")
	}
}

// With no corpus there is nothing to build from: -tables says so.
func TestRunTablesNoCorpus(t *testing.T) {
	t.Setenv("GORGE_CARDS", "")
	t.Chdir(t.TempDir())
	var errOut bytes.Buffer
	if rc := runTables("", io.Discard, &errOut); rc != 1 {
		t.Fatalf("exit %d", rc)
	}
	if !strings.Contains(errOut.String(), "no card corpus") {
		t.Errorf("printed %q", errOut.String())
	}
}

// -show prints every card of a pool (the printer panics on a value it has no name
// for), and -supported lists each entry once.
func TestShowAndSupported(t *testing.T) {
	for _, pool := range []string{"fdn", "blb"} {
		s := cardsettest.Set(t, pool)
		for _, name := range s.Names() {
			var b bytes.Buffer
			if err := runShow(pool, name, &b); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if !strings.HasPrefix(b.String(), "kop.CardDef{") {
				t.Fatalf("%s: printed %q", name, b.String())
			}
		}
		var b bytes.Buffer
		if err := runSupported(pool, &b); err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSpace(b.String()), "\n")
		if len(lines) != len(s.Names()) {
			t.Errorf("%s: %d lines, %d names", pool, len(lines), len(s.Names()))
		}
		if want := fmt.Sprintf("%s\t%v", s.Names()[0], s.Supported()[0]); lines[0] != want {
			t.Errorf("%s: first line %q, want %q", pool, lines[0], want)
		}
	}
	if err := runShow("fdn", "No Such Card", io.Discard); err == nil || !strings.Contains(err.Error(), "No Such Card") {
		t.Errorf("unknown card: %v", err)
	}
}
