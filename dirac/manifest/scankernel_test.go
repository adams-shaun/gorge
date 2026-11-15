package manifest

import (
	"strings"
	"testing"
	"testing/fstest"
)

const kernelSrc = handlerSrc + `// opFoo runs OpHot.
//
//kmanifest:op OpHot
func (s *State) opFoo(k *prep, in *kop.Instr) (ok, done bool) { return true, true }

//kmanifest:askopts raQuux
func (s *State) quuxOptions(k *prep, d *Decision) {}

//kmanifest:askapply raQuux
func (s *State) quuxAnswer(k *prep, a Action) {}
`

// TestScanKernelFindsEveryKind: the op handler and the ask's two handlers are
// found, sorted, and the generated file dispatches each.
func TestScanKernelFindsEveryKind(t *testing.T) {
	m := fixture(t)
	m["dirac/act_foo.go"] = &fstest.MapFile{Data: []byte(kernelSrc)}
	h, err := ScanKernel(m)
	if err != nil {
		t.Fatal(err)
	}
	if want := (OpHandler{Op: "OpHot", Method: "opFoo", File: "dirac/act_foo.go"}); len(h.Op) != 1 || h.Op[0] != want {
		t.Errorf("Op = %+v, want %+v", h.Op, want)
	}
	if want := (AskHandler{Ask: "raQuux", Options: "quuxOptions", Apply: "quuxAnswer", OptionsFile: "dirac/act_foo.go", ApplyFile: "dirac/act_foo.go"}); len(h.Ask) != 1 || h.Ask[0] != want {
		t.Errorf("Ask = %+v, want %+v", h.Ask, want)
	}
	if len(h.Ext) != 1 || h.Ext[0].Op != "ExtBar" {
		t.Errorf("Ext = %+v, want the fixture's ExtBar", h.Ext)
	}
	out := string(generate(t, m)[DiracGen])
	for _, want := range []string{
		"\tcase kop.OpHot:\n\t\treturn s.opFoo(k, in) // act_foo.go\n",
		"\tcase raQuux:\n\t\ts.quuxOptions(k, d) // act_foo.go\n",
		"\tcase raQuux:\n\t\ts.quuxAnswer(k, a) // act_foo.go\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("%s lacks %q:\n%s", DiracGen, want, out)
		}
	}
}

// TestScanKernelRefuses: every misuse of the op and ask directives names its file.
func TestScanKernelRefuses(t *testing.T) {
	const op = "func (s *State) f(k *prep, in *kop.Instr) (ok, done bool) { return }\n"
	for _, c := range []struct{ name, src, want string }{
		{"op not an Op", "//kmanifest:op Foo\n" + op, "wants one Op constant name"},
		{"op two names", "//kmanifest:op OpA OpB\n" + op, "wants one Op constant name"},
		{"op wrong params", "//kmanifest:op OpA\nfunc (s *State) f(k *prep) (ok, done bool) { return }\n", "is not a func (*State)"},
		{"op stray", "func (s *State) f() {\n\t//kmanifest:op OpA\n}\n", "//kmanifest:op is not in a function's doc comment"},
		{"op twice", "//kmanifest:op OpHot\n" + op + "\n//kmanifest:op OpHot\nfunc (s *State) g(k *prep, in *kop.Instr) (ok, done bool) { return }\n", "OpHot has two handlers"},
		{"op hand case", "func (s *State) execCold9(k *prep, in *kop.Instr) (ok, done bool) {\n\tswitch in.Op {\n\tcase kop.OpHot:\n\t}\n\treturn\n}\n\n//kmanifest:op OpHot\n" + op, "OpHot already has a case in execCold9"},
		{"ask not an ask", "//kmanifest:askopts Quux\nfunc (s *State) f(k *prep, d *Decision) {}\n", "wants one ask constant name"},
		{"ask options result", "//kmanifest:askopts raA\nfunc (s *State) f(k *prep, d *Decision) bool { return true }\n", "is not a func (*State) (*prep, *Decision)"},
		{"ask apply params", "//kmanifest:askapply raA\nfunc (s *State) f(k *prep, d *Decision) {}\n", "is not a func (*State) (*prep, Action)"},
		{"ask options alone", "//kmanifest:askopts raA\nfunc (s *State) f(k *prep, d *Decision) {}\n", "raA has //kmanifest:askopts f but no //kmanifest:askapply"},
		{"ask apply alone", "//kmanifest:askapply raA\nfunc (s *State) f(k *prep, a Action) {}\n", "raA has //kmanifest:askapply f but no //kmanifest:askopts"},
		{"ask options twice", "//kmanifest:askopts raA\nfunc (s *State) f(k *prep, d *Decision) {}\n\n//kmanifest:askopts raA\nfunc (s *State) g(k *prep, d *Decision) {}\n", "raA has two handlers"},
		{"ask stray", "var x = 1 //kmanifest:askapply raA\n", "//kmanifest:askapply is not in a function's doc comment"},
		{"ask hand case", "func (s *State) askOptionsColdN(k *prep, d *Decision) bool {\n\tswitch s.res.ask {\n\tcase raA:\n\t}\n\treturn true\n}\n\n//kmanifest:askopts raA\nfunc (s *State) f(k *prep, d *Decision) {}\n\n//kmanifest:askapply raA\nfunc (s *State) g(k *prep, a Action) {}\n", "raA already has a case in askOptionsColdN"},
		{"ask apply hand case", "func (s *State) applyResolveX(k *prep, a Action) {\n\tswitch s.res.ask {\n\tcase raA:\n\t}\n}\n\n//kmanifest:askopts raA\nfunc (s *State) f(k *prep, d *Decision) {}\n\n//kmanifest:askapply raA\nfunc (s *State) g(k *prep, a Action) {}\n", "raA already has a case in applyResolveX"},
	} {
		m := fixture(t)
		m["dirac/zz_case.go"] = &fstest.MapFile{Data: []byte(handlerSrc + c.src)}
		_, err := ScanKernel(m)
		if err == nil || !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), "dirac/") {
			t.Errorf("%s: err %v, want one containing %q", c.name, err, c.want)
		}
	}
}

// TestScanKernelLeavesOtherMethodsAlone: a case in a method outside the
// chains (a helper that merely names the ask) is no hand arm.
func TestScanKernelLeavesOtherMethodsAlone(t *testing.T) {
	m := fixture(t)
	m["dirac/zz_case.go"] = &fstest.MapFile{Data: []byte(kernelSrc + "\nfunc (s *State) describe() int {\n\tswitch s.res.ask {\n\tcase raQuux:\n\t\treturn 1\n\t}\n\treturn 0\n}\n\n//kmanifest:opx OpA\n//kmanifest:askoptsX raB\nfunc (s *State) h() {}\n")}
	if _, err := ScanKernel(m); err != nil {
		t.Fatal(err)
	}
}
