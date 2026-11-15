package manifest

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The adopt fixture: a module whose base (land) has the manifest entries
// CtrTicket (TICKET) and ExtMakeCard and their generated constants, and a
// unit branch that added three hand constants the way changes did before
// the manifest, one per case of Adopt:
//
//   - ExtMakeCard: the same name as an entry;
//   - CtrTix: another name for the entry CtrTicket's vocabulary item
//     (its CtrXNames string is TICKET);
//   - TrigFoo: no entry.
var adoptBase = map[string]string{
	"go.mod": "module github.com/adams-shaun/gorge\n\ngo 1.22\n",
	"dirac/kop/counters.go": `package kop

// CounterKind names a counter type.
type CounterKind uint8

const (
	CtrP1P1 CounterKind = iota
	CtrM1M1
	CtrCharge
	CtrOil
)

// NumCounterKinds3 is frozen: it covers every manifest counter kind.
const NumCounterKinds3 CounterKind = 10

// NumCounterKindsX is the number of unfeaturized kinds.
const NumCounterKindsX = int(NumCounterKinds3 - CtrCharge)

// CtrXNames is each unfeaturized kind's name.
var CtrXNames = [NumCounterKindsX]string{"CHARGE", "OIL"}
`,
	"dirac/kop/ext.go": `package kop

// ExtOp is a sub-op.
type ExtOp int16

const (
	ExtNone   ExtOp = 0
	NumExtOps ExtOp = 321
)

var extOpNames [NumExtOps]string

func (o ExtOp) String() string { return extOpNames[o] }

// Op is an operation.
type Op uint8

// Instr is one instruction.
type Instr struct {
	Op Op
	A  struct{ N int32 }
}
`,
	"dirac/kop/triggers.go": `package kop

// TrigEvent is a trigger event.
type TrigEvent uint8

const (
	TrigNone TrigEvent = iota
	TrigETB
	TrigEventCount
)
`,
	"dirac/kop/testdata/ordinals.tsv": "CtrCharge\tCounterKind\t2\nCtrM1M1\tCounterKind\t1\nCtrOil\tCounterKind\t3\nCtrP1P1\tCounterKind\t0\n" +
		"CtrTicket\tCounterKind\t9\nExtMakeCard\tExtOp\t2\nExtNone\tExtOp\t0\nNumCounterKinds3\tCounterKind\t10\nNumCounterKindsX\tuntyped int\t8\n" +
		"NumExtOps\tExtOp\t321\nTrigETB\tTrigEvent\t1\nTrigEventCount\tTrigEvent\t2\nTrigNone\tTrigEvent\t0\n",
	"dirac/kgen/names.go": `package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

var names []string

func enum[T ~uint8 | ~uint16 | ~int16](pairs ...any) {
	for i := 0; i < len(pairs); i += 2 {
		names = append(names, pairs[i].(string))
	}
}

func init() {
	enum[kop.CounterKind]("kop.CtrP1P1", kop.CtrP1P1, "kop.CtrOil", kop.CtrOil)
}
`,
	"dirac/kernel.go":                      "package dirac\n\nimport \"github.com/adams-shaun/gorge/dirac/kop\"\n\nvar _ = kop.CtrOil\n\ntype (\n\tState struct{ res struct{ ask uint8 } }\n\tprep  struct{}\n\tDecision struct{}\n\tAction struct{}\n)\n",
	"dirac/manifest/CounterKind/CtrTicket": "family: CounterKind\nname: CtrTicket\nvalue: 9\nforge: TICKET\nstatus: reserved\n",
	"dirac/manifest/ExtOp/ExtMakeCard":     "family: ExtOp\nname: ExtMakeCard\nvalue: 2\nforge: MakeCard\nstatus: reserved\n",
}

// adoptUnit is the unit's change on top of adoptBase.
var adoptUnit = map[string]string{
	"dirac/kop/counters.go": strings.Replace(strings.Replace(adoptBase["dirac/kop/counters.go"],
		"\tCtrOil\n)", "\tCtrOil\n\t// CtrTix is a ticket counter.\n\tCtrTix\n)", 1),
		`{"CHARGE", "OIL"}`, `{"CHARGE", "OIL", "TICKET"}`, 1),
	"dirac/kop/makecard.go": `package kop

// ExtMakeCard conjures a card.
const ExtMakeCard ExtOp = 12

func init() { extOpNames[ExtMakeCard] = "MakeCard" }
`,
	"dirac/kop/triggers_foo.go": `package kop

// TrigFoo fires when a foo happens (Forge Mode$ Foo).
// It is the unit's own.
const TrigFoo TrigEvent = 60
`,
	"dirac/kop/testdata/ordinals.tsv": adoptBase["dirac/kop/testdata/ordinals.tsv"] + "CtrTix\tCounterKind\t4\nTrigFoo\tTrigEvent\t60\n",
	"dirac/kgen/names.go": strings.Replace(adoptBase["dirac/kgen/names.go"],
		`"kop.CtrOil", kop.CtrOil)`, `"kop.CtrOil", kop.CtrOil, "kop.CtrTix", kop.CtrTix, "kop.TrigFoo", kop.TrigFoo)`, 1),
	"dirac/kgen/lower_foo.go": "package kgen\n\nimport \"github.com/adams-shaun/gorge/dirac/kop\"\n\nvar foo = kop.TrigFoo\n",
	"dirac/ctr_tix.go":        "package dirac\n\nimport \"github.com/adams-shaun/gorge/dirac/kop\"\n\nfunc tix() kop.CounterKind { return kop.CtrTix }\n",
	"dirac/act_makecard.go":   "package dirac\n\nimport \"github.com/adams-shaun/gorge/dirac/kop\"\n\nvar makeCard = kop.ExtMakeCard\n",
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, s := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// adoptRepo builds the fixture repository: land (adoptBase plus its
// generated files) and the unit's commit on top, checked out.
func adoptRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFiles(t, root, adoptBase)
	if _, err := WriteGen(root, false); err != nil {
		t.Fatal(err)
	}
	gitT(t, root, "init", "-q", "-b", "land")
	gitT(t, root, "add", ".")
	gitT(t, root, "commit", "-q", "-m", "land")
	gitT(t, root, "checkout", "-q", "-b", "unit")
	writeFiles(t, root, adoptUnit)
	gitT(t, root, "add", ".")
	gitT(t, root, "commit", "-q", "-m", "unit")
	return root
}

// TestAdoptThreeCases adopts the fixture unit: ExtMakeCard keeps its name
// and the generated constant; CtrTix's uses become CtrTicket (gofmt -r) and
// its CtrXNames element and kgen pair go; TrigFoo becomes a value: next
// entry with its doc. Entries the kernel uses turn active. The result
// builds, passes check against land, and assign numbers TrigFoo.
func TestAdoptThreeCases(t *testing.T) {
	root := adoptRepo(t)
	base, err := ReadGit(root, "land")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Adopt(root, base)
	if err != nil {
		t.Fatal(err)
	}
	want := []Adoption{
		{Name: "CtrTix", Entry: "dirac/manifest/CounterKind/CtrTicket", Case: AdoptRenamed, Active: true},
		{Name: "ExtMakeCard", Entry: "dirac/manifest/ExtOp/ExtMakeCard", Case: AdoptSameName, Active: true},
		{Name: "TrigFoo", Entry: "dirac/manifest/TrigEvent/TrigFoo", Case: AdoptNext, Active: false},
	}
	if len(got) != len(want) {
		t.Fatalf("adopted %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("adoption %d: %+v, want %+v", i, got[i], want[i])
		}
	}

	for p, s := range map[string]string{
		"dirac/kop/counters.go":     adoptBase["dirac/kop/counters.go"],
		"dirac/kop/makecard.go":     "package kop\n",
		"dirac/kop/triggers_foo.go": "package kop\n",
		"dirac/kgen/names.go":       adoptBase["dirac/kgen/names.go"],
		"dirac/ctr_tix.go":          strings.Replace(adoptUnit["dirac/ctr_tix.go"], "kop.CtrTix", "kop.CtrTicket", 1),
		"dirac/manifest/TrigEvent/TrigFoo": "family: TrigEvent\nname: TrigFoo\nvalue: next\nstatus: reserved\n" +
			"doc: TrigFoo fires when a foo happens (Forge Mode$ Foo). It is the unit's own.\n",
		"dirac/manifest/CounterKind/CtrTicket": strings.Replace(adoptBase["dirac/manifest/CounterKind/CtrTicket"], "reserved", "active", 1),
		"dirac/manifest/ExtOp/ExtMakeCard":     strings.Replace(adoptBase["dirac/manifest/ExtOp/ExtMakeCard"], "reserved", "active", 1),
	} {
		if g := readFile(t, root, p); g != s {
			t.Errorf("%s:\n%s\nwant:\n%s", p, g, s)
		}
	}
	gen := readFile(t, root, KopGen)
	for _, s := range []string{"const TrigFoo TrigEvent = 3\n", "const ExtMakeCard ExtOp = 2\n", "const CtrTicket CounterKind = 9\n"} {
		if !strings.Contains(gen, s) {
			t.Errorf("%s lacks %q:\n%s", KopGen, s, gen)
		}
	}

	// runtime.GOROOT() is empty in a -trimpath test binary (CI lint), so fall back to the go on PATH.
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		if goBin, err = exec.LookPath("go"); err != nil {
			t.Fatalf("no go toolchain: %v", err)
		}
	}
	build := exec.Command(goBin, "build", "./...")
	build.Dir = root
	build.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=", "GOTOOLCHAIN=local")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("the adopted unit does not build: %v\n%s", err, out)
	}

	tr, err := Read(os.DirFS(root))
	if err != nil {
		t.Fatal(err)
	}
	if errs := Check(base, tr, CheckOptions{}); len(errs) > 0 {
		t.Errorf("check after adopt: %v", errs)
	}
	as, err := Assign(root, base)
	if err != nil || len(as) != 1 || as[0] != (Assignment{"dirac/manifest/TrigEvent/TrigFoo", 3}) {
		t.Errorf("assign after adopt: %v, %v", as, err)
	}

	again, err := Adopt(root, base)
	if err != nil || len(again) != 0 {
		t.Errorf("a second adopt: %v, %v; want nothing to do", again, err)
	}
}

// TestAdoptRefuses: a constant two entries match, and a declaration a later
// constant repeats, are errors naming the constant, and nothing is written.
func TestAdoptRefuses(t *testing.T) {
	for _, c := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"ambiguous", map[string]string{
			"dirac/manifest/CounterKind/CtrTicketB": "family: CounterKind\nname: CtrTicketB\nvalue: 8\nforge: Ticket\nstatus: reserved\n",
		}, "CtrTix matches 2 entries"},
		{"repeated", map[string]string{
			"dirac/kop/triggers_foo.go": "package kop\n\nconst (\n\tTrigFoo TrigEvent = 60 + iota\n\tTrigFooToo\n)\n",
		}, "cannot remove TrigFoo: TrigFooToo repeats its expression"},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := adoptRepo(t)
			writeFiles(t, root, c.files)
			base, err := ReadGit(root, "land")
			if err != nil {
				t.Fatal(err)
			}
			before := readFile(t, root, "dirac/kop/counters.go")
			_, err = Adopt(root, base)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("adopt: %v, want %q", err, c.want)
			}
			if readFile(t, root, "dirac/kop/counters.go") != before {
				t.Error("a refused adopt edited counters.go")
			}
		})
	}
}
