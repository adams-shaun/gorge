package manifest

import (
	"strings"
	"testing"
	"testing/fstest"
)

const fwordSrc = handlerSrc + `// fwFoo is a filter word.
//
//kmanifest:fword FwFoo
func (s *State) fwFoo(c *fctx, l *kop.FLit) bool { return true }
`

// fwordFixture is the tree fixture with one FilterWord entry, FwFoo.
func fwordFixture(t *testing.T) fstest.MapFS {
	m := fixture(t)
	m["dirac/manifest/FilterWord/FwFoo"] = &fstest.MapFile{Data: []byte("family: FilterWord\nname: FwFoo\nvalue: 3\ncontexts: perm,card\nstatus: active\n")}
	return m
}

// TestScanFWord: a //kmanifest:fword handler is found, and gen writes the
// switch arm that runs it and the fwHandled case.
func TestScanFWord(t *testing.T) {
	m := fwordFixture(t)
	m["dirac/filter_foo.go"] = &fstest.MapFile{Data: []byte(fwordSrc)}
	h, err := ScanKernel(m)
	if err != nil {
		t.Fatal(err)
	}
	if want := (WordHandler{Word: "FwFoo", Method: "fwFoo", File: "dirac/filter_foo.go"}); len(h.Word) != 1 || h.Word[0] != want {
		t.Fatalf("Word = %+v, want %+v", h.Word, want)
	}
	out := string(generate(t, m)[DiracGen])
	for _, want := range []string{
		"func (s *State) fwRun(c *fctx, l *kop.FLit) bool {\n\tswitch l.W {\n\tcase kop.FwFoo:\n\t\treturn s.fwFoo(c, l) // filter_foo.go\n\t}\n",
		"func fwHandled(w kop.FilterWord) bool {\n\tswitch w {\n\tcase kop.FwFoo:\n\t\treturn true\n\t}\n\treturn false\n}\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("%s lacks %q:\n%s", DiracGen, want, out)
		}
	}
}

// TestScanFWordRefuses: every misuse of the directive names its file.
func TestScanFWordRefuses(t *testing.T) {
	const fn = "func (s *State) f(c *fctx, l *kop.FLit) bool { return true }\n"
	for _, c := range []struct{ name, src, want string }{
		{"not a word", "//kmanifest:fword Foo\n" + fn, "wants one FilterWord constant name"},
		{"two names", "//kmanifest:fword FwA FwB\n" + fn, "wants one FilterWord constant name"},
		{"plain func", "//kmanifest:fword FwA\nfunc f(c *fctx, l *kop.FLit) bool { return true }\n", "is not a func (*State) (*fctx, *kop.FLit) bool"},
		{"wrong params", "//kmanifest:fword FwA\nfunc (s *State) f(k *prep, l *kop.FLit) bool { return true }\n", "is not a func (*State) (*fctx, *kop.FLit) bool"},
		{"no result", "//kmanifest:fword FwA\nfunc (s *State) f(c *fctx, l *kop.FLit) {}\n", "is not a func (*State) (*fctx, *kop.FLit) bool"},
		{"stray", "func (s *State) f() {\n\t//kmanifest:fword FwA\n}\n", "//kmanifest:fword is not in a function's doc comment"},
		{"twice", "//kmanifest:fword FwFoo\n" + fn + "\n//kmanifest:fword FwFoo\nfunc (s *State) g(c *fctx, l *kop.FLit) bool { return true }\n", "FwFoo has two handlers"},
	} {
		m := fwordFixture(t)
		m["dirac/zz_case.go"] = &fstest.MapFile{Data: []byte(handlerSrc + c.src)}
		if c.name == "twice" {
			m["dirac/filter_foo.go"] = &fstest.MapFile{Data: []byte(fwordSrc)}
		}
		_, err := ScanKernel(m)
		if err == nil || !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), "dirac/") {
			t.Errorf("%s: err %v, want one containing %q", c.name, err, c.want)
		}
	}
}

// TestFilterWordContexts: a FilterWord entry has a contexts line of known,
// distinct matcher kinds, and no other family has one.
func TestFilterWordContexts(t *testing.T) {
	for _, c := range []struct{ name, entry, want string }{
		{"missing", "family: FilterWord\nname: FwFoo\nvalue: 3\nstatus: active\n", "needs a contexts line"},
		{"unknown", "family: FilterWord\nname: FwFoo\nvalue: 3\ncontexts: perm,hand\nstatus: active\n", `context "hand" is not one of`},
		{"twice", "family: FilterWord\nname: FwFoo\nvalue: 3\ncontexts: perm,perm\nstatus: active\n", "context perm given twice"},
	} {
		m := fixture(t)
		m["dirac/manifest/FilterWord/FwFoo"] = &fstest.MapFile{Data: []byte(c.entry)}
		if _, err := Load(m); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err %v, want one containing %q", c.name, err, c.want)
		}
	}
	m := fixture(t)
	m["dirac/manifest/ExtOp/ExtZed"] = &fstest.MapFile{Data: []byte("family: ExtOp\nname: ExtZed\nvalue: 99\ncontexts: perm\nstatus: reserved\n")}
	if _, err := Load(m); err == nil || !strings.Contains(err.Error(), "a contexts line is a FilterWord's") {
		t.Errorf("ExtOp with contexts: err %v", err)
	}
	if _, err := Load(fwordFixture(t)); err != nil {
		t.Errorf("a valid FilterWord entry: %v", err)
	}
}
