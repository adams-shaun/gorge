package manifest

import (
	"bytes"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// goldenDir holds Generate's output for testdata/tree, one file per
// generated path, slashes as underscores. KMANIFEST_UPDATE=1 rewrites it.
const goldenDir = "testdata/gen"

func generate(t *testing.T, m fstest.MapFS) map[string][]byte {
	t.Helper()
	tr, err := LoadGen(m)
	if err != nil {
		t.Fatal(err)
	}
	files, err := Generate(tr)
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestGenerateGolden(t *testing.T) {
	files := generate(t, fixture(t))
	for _, p := range GenFiles {
		golden := filepath.Join(goldenDir, strings.ReplaceAll(p, "/", "_")+".golden")
		if os.Getenv("KMANIFEST_UPDATE") != "" {
			if err := os.WriteFile(golden, files[p], 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(files[p], want) {
			t.Errorf("%s differs from %s (KMANIFEST_UPDATE=1 rewrites it):\n%s", p, golden, files[p])
		}
	}
}

// TestGenerateIsGofmtStable: gofmt leaves every generated file as it is, so
// a gofmt pass over the tree never rewrites one.
func TestGenerateIsGofmtStable(t *testing.T) {
	for p, b := range generate(t, fixture(t)) {
		f, err := format.Source(b)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if !bytes.Equal(f, b) {
			t.Errorf("%s: gofmt changes it", p)
		}
		if !bytes.HasPrefix(b, []byte(GenHeader+"\n")) {
			t.Errorf("%s: no %q header", p, GenHeader)
		}
	}
}

// TestNewEntryRewritesNoNeighbour is the reason for the generated format:
// adding an entry between two others inserts lines into each generated file
// and changes none of the lines around them.
func TestNewEntryRewritesNoNeighbour(t *testing.T) {
	before := generate(t, fixture(t))
	m := fixture(t)
	set(Dir+"/ExtOp/ExtMiddlingLongName", "family: ExtOp\nname: ExtMiddlingLongName\nvalue: 4\nforge: Mid\nstatus: reserved\n")(m)
	set(Dir+"/CounterKind/CtrMid", "family: CounterKind\nname: CtrMid\nvalue: 100\nforge: MID\nstatus: reserved\n")(m)
	after := generate(t, m)
	for _, p := range GenFiles {
		old := strings.Split(string(before[p]), "\n")
		cur := strings.Split(string(after[p]), "\n")
		pre := 0
		for pre < len(old) && pre < len(cur) && old[pre] == cur[pre] {
			pre++
		}
		suf := 0
		for suf < len(old)-pre && suf < len(cur)-pre && old[len(old)-1-suf] == cur[len(cur)-1-suf] {
			suf++
		}
		if p == DiracGen {
			if pre != len(old) || len(cur) != len(old) {
				t.Errorf("%s changed with no dirac entry added", p)
			}
			continue
		}
		// Two entries went in (kgen registers only the counter): the old
		// file's lines must all survive, in order, around the inserted ones.
		if !isSubsequence(old, cur) {
			t.Errorf("%s: an old line changed:\n--- before\n%s\n--- after\n%s", p, before[p], after[p])
		}
		if len(cur) <= len(old) {
			t.Errorf("%s: did not grow", p)
		}
	}
}

func isSubsequence(old, cur []string) bool {
	i := 0
	for _, l := range cur {
		if i < len(old) && old[i] == l {
			i++
		}
	}
	return i == len(old)
}

// TestActivationChangesNoGeneratedByte: status is never emitted, so two
// changes that activate different entries touch different files only.
func TestActivationChangesNoGeneratedByte(t *testing.T) {
	before := generate(t, fixture(t))
	m := fixture(t)
	p := Dir + "/CounterKind/CtrTicket"
	m[p].Data = bytes.Replace(m[p].Data, []byte("status: reserved"), []byte("status: active\nticket: example-2"), 1)
	after := generate(t, m)
	for _, f := range GenFiles {
		if !bytes.Equal(before[f], after[f]) {
			t.Errorf("%s changed when an entry was activated", f)
		}
	}
}

func TestGenerateEmptyManifest(t *testing.T) {
	files := generate(t, fstest.MapFS{OrdinalsPath: {Data: []byte("OpNop\tOp\t0\n")}})
	for p, want := range map[string]string{
		KopGen:   GenHeader + "\n\npackage kop\n",
		DiracGen: GenHeader + "\n\npackage dirac\n\nimport \"" + kopImport + "\"\n" + genExtDispatch(nil),
		KgenGen:  GenHeader + "\n\npackage kgen\n",
	} {
		if string(files[p]) != want {
			t.Errorf("%s = %q, want %q", p, files[p], want)
		}
	}
}

// copyTree copies testdata/tree into a temporary module root.
func copyTree(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	src := os.DirFS("testdata/tree")
	err := fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		out := filepath.Join(dst, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		b, err := fs.ReadFile(src, p)
		if err != nil {
			return err
		}
		return os.WriteFile(out, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

// TestWriteGenSecondRunIsNoOp: the first run writes the three files, the
// second finds nothing to write, and check reports exactly what is stale.
func TestWriteGenSecondRunIsNoOp(t *testing.T) {
	root := copyTree(t)
	changed, err := WriteGen(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(changed, " ") != strings.Join(GenFiles, " ") {
		t.Errorf("first run wrote %v, want %v", changed, GenFiles)
	}
	if changed, err = WriteGen(root, false); err != nil || len(changed) != 0 {
		t.Errorf("second run wrote %v (%v), want nothing", changed, err)
	}
	if changed, err = WriteGen(root, true); err != nil || len(changed) != 0 {
		t.Errorf("check after a run: %v (%v), want nothing stale", changed, err)
	}
	entry := filepath.Join(root, filepath.FromSlash(Dir+"/ExtOp/ExtBar"))
	if err := os.WriteFile(entry, []byte("family: ExtOp\nname: ExtBar\nvalue: 9\nstatus: active\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	kop := filepath.Join(root, filepath.FromSlash(KopGen))
	was, _ := os.ReadFile(kop)
	if changed, err = WriteGen(root, true); err != nil || strings.Join(changed, " ") != KopGen {
		t.Errorf("check after an edit: %v (%v), want [%s]", changed, err, KopGen)
	}
	if now, _ := os.ReadFile(kop); !bytes.Equal(now, was) {
		t.Errorf("check wrote %s", KopGen)
	}
}
