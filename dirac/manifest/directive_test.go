package manifest

import (
	"strings"
	"testing"
	"testing/fstest"
)

// TestScanExtFixture: the fixture's one handler is found; a test file's
// directive is not a handler.
func TestScanExtFixture(t *testing.T) {
	hs, err := ScanExt(fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	want := []ExtHandler{{Op: "ExtBar", Method: "extBar", File: "dirac/act_bar.go"}}
	if len(hs) != 1 || hs[0] != want[0] {
		t.Fatalf("ScanExt = %+v, want %+v", hs, want)
	}
	files := generate(t, fixture(t))
	if !strings.Contains(string(files[DiracGen]), "\tcase kop.ExtBar:\n\t\treturn s.extBar(k, in) // act_bar.go\n") {
		t.Errorf("%s has no ExtBar case:\n%s", DiracGen, files[DiracGen])
	}
}

func TestScanExtNoKernel(t *testing.T) {
	hs, err := ScanExt(fstest.MapFS{})
	if err != nil || hs != nil {
		t.Fatalf("ScanExt of an empty tree = %v, %v", hs, err)
	}
}

const handlerSrc = "package dirac\n\nimport \"github.com/adams-shaun/gorge/dirac/kop\"\n\n"

// TestScanExtRefuses: every misuse of the directive names its file.
func TestScanExtRefuses(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{
		{"not an ExtOp", "//kmanifest:ext Foo\nfunc (s *State) f(k *prep, in *kop.Instr) (ok, done bool) { return }\n", "wants one ExtOp constant name"},
		{"two names", "//kmanifest:ext ExtA ExtB\nfunc (s *State) f(k *prep, in *kop.Instr) (ok, done bool) { return }\n", "wants one ExtOp constant name"},
		{"plain func", "//kmanifest:ext ExtA\nfunc f(k *prep, in *kop.Instr) (ok, done bool) { return }\n", "is not a func (*State)"},
		{"wrong params", "//kmanifest:ext ExtA\nfunc (s *State) f(k *prep) (ok, done bool) { return }\n", "is not a func (*State)"},
		{"one result", "//kmanifest:ext ExtA\nfunc (s *State) f(k *prep, in *kop.Instr) bool { return true }\n", "is not a func (*State)"},
		{"stray", "func (s *State) f(k *prep, in *kop.Instr) (ok, done bool) {\n\t//kmanifest:ext ExtA\n\treturn\n}\n", "is not in a function's doc comment"},
		{"hand case", "//kmanifest:ext ExtLegacy\nfunc (s *State) f(k *prep, in *kop.Instr) (ok, done bool) { return }\n", "already has a case in execExt"},
		{"twice", "//kmanifest:ext ExtBar\nfunc (s *State) f(k *prep, in *kop.Instr) (ok, done bool) { return }\n", "ExtBar has two handlers"},
	} {
		m := fixture(t)
		m["dirac/zz_case.go"] = &fstest.MapFile{Data: []byte(handlerSrc + c.src)}
		_, err := ScanExt(m)
		if err == nil || !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), "dirac/") {
			t.Errorf("%s: err %v, want one containing %q", c.name, err, c.want)
		}
	}
}

// TestScanExtIgnoresLookalikes: a longer directive word is not this one.
func TestScanExtIgnoresLookalikes(t *testing.T) {
	m := fixture(t)
	m["dirac/zz_case.go"] = &fstest.MapFile{Data: []byte(handlerSrc + "//kmanifest:extra ExtA\nfunc (s *State) f(k *prep, in *kop.Instr) (ok, done bool) { return }\n")}
	hs, err := ScanExt(m)
	if err != nil || len(hs) != 1 {
		t.Fatalf("ScanExt = %v, %v; want the fixture's one handler", hs, err)
	}
}
