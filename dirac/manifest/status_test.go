package manifest

import (
	"strings"
	"testing"
	"testing/fstest"
)

// TestStatusMismatches: an active entry the kernel does not reference, and a
// reserved one it does, each fail naming the entry; a reference through a
// //kmanifest:ext handler, a bare dirac name, a test file and the generated
// file are handled as the invariant says.
func TestStatusMismatches(t *testing.T) {
	entry := func(fam, name, status string) *fstest.MapFile {
		return &fstest.MapFile{Data: []byte("family: " + fam + "\nname: " + name + "\nvalue: next\nforge: X" + name + "\nstate: slotbit 16\nstatus: " + status + "\n")}
	}
	m := fstest.MapFS{
		OrdinalsPath:                          {Data: []byte("CtrP1P1\tCounterKind\t0\n")},
		"dirac/manifest/CounterKind/CtrUsed":  entry("CounterKind", "CtrUsed", "active"),
		"dirac/manifest/CounterKind/CtrIdle":  entry("CounterKind", "CtrIdle", "active"),
		"dirac/manifest/CounterKind/CtrEarly": entry("CounterKind", "CtrEarly", "reserved"),
		"dirac/manifest/CounterKind/CtrTest":  entry("CounterKind", "CtrTest", "reserved"),
		"dirac/manifest/ExtOp/ExtHandled":     entry("ExtOp", "ExtHandled", "active"),
		"dirac/manifest/mech/mechMarked":      entry("mech", "mechMarked", "active"),
		"dirac/manifest/key/keyGen":           entry("key", "keyGen", "reserved"),
		"dirac/extop.go":                      {Data: []byte(handlerSrc + "func (s *State) execExt(k *prep, in *kop.Instr) (ok, done bool) {\n\tswitch kop.ExtOp(in.A.N) {\n\t}\n\treturn\n}\n")},
		"dirac/ctr_used.go":                   {Data: []byte(handlerSrc + "var _ = kop.CtrUsed\nvar _ = kop.CtrEarly\nvar _ = mechMarked\n")},
		"dirac/act_handled.go":                {Data: []byte(handlerSrc + "//kmanifest:ext ExtHandled\nfunc (s *State) extHandled(k *prep, in *kop.Instr) (ok, done bool) { return }\n")},
		"dirac/ctr_used_test.go":              {Data: []byte(handlerSrc + "var _ = kop.CtrTest\n")},
		DiracGen:                              {Data: []byte("package dirac\n\nconst keyGen key = 0\n\nvar _ = keyGen\n")},
	}
	tr, err := Load(m)
	if err != nil {
		t.Fatal(err)
	}
	errs, err := StatusMismatches(m, tr)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range errs {
		got = append(got, e.Error())
	}
	want := []string{
		"dirac/manifest/CounterKind/CtrEarly: package dirac references kop.CtrEarly, but its status is reserved: set status: active",
		"dirac/manifest/CounterKind/CtrIdle: status active, but package dirac references no kop.CtrIdle: implement it, or keep the entry reserved",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("StatusMismatches =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
