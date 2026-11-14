package manifest

import (
	"io/fs"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
)

// fixture is testdata/tree as a map, so a case can edit one file.
func fixture(t *testing.T) fstest.MapFS {
	t.Helper()
	m := fstest.MapFS{}
	root := os.DirFS("testdata/tree")
	err := fs.WalkDir(root, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(root, p)
		m[p] = &fstest.MapFile{Data: b}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func load(t *testing.T, fsys fs.FS) *Tree {
	t.Helper()
	tr, err := Load(fsys)
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

func TestFamiliesSortedAndUnique(t *testing.T) {
	for i, f := range Families {
		if i > 0 && Families[i-1].Name >= f.Name {
			t.Errorf("Families: %s after %s", f.Name, Families[i-1].Name)
		}
		if f.Pkg != "kop" && f.Pkg != "dirac" {
			t.Errorf("%s: package %q", f.Name, f.Pkg)
		}
		if f.Alloc.Lo < f.Min || f.Alloc.Hi > f.Max || f.Alloc.Lo > f.Alloc.Hi {
			t.Errorf("%s: Alloc %v outside [%d,%d]", f.Name, f.Alloc, f.Min, f.Max)
		}
	}
	op := FamilyNamed("Op")
	if len(op.Reserved) != 1 || op.Reserved[0].Lo != 158 || op.Reserved[0].Hi != 249 {
		t.Errorf("Op's reserved band is %v, want [158,249]", op.Reserved)
	}
	if ck := FamilyNamed("CounterKind"); ck.Max != 65535 {
		t.Errorf("CounterKind max %d, want 65535 (perm.ck is uint16)", ck.Max)
	}
}

// TestFamiliesMatchOrdinals keeps the family table honest against kop: every
// kop family but the new KwID names a type ordinals.tsv records, and the
// source tree's manifest validates.
func TestFamiliesMatchOrdinals(t *testing.T) {
	tr, err := Read(os.DirFS("../.."))
	if err != nil {
		t.Fatal(err)
	}
	types := map[string]int{}
	for _, r := range tr.Ordinals {
		types[r.Type]++
	}
	for _, f := range Families {
		if f.Pkg != "kop" || f.Name == "KwID" { // KwID is new at the cutover
			continue
		}
		if types[f.Name] == 0 {
			t.Errorf("family %s: no ordinals.tsv row has that type", f.Name)
		}
	}
	if errs := tr.Validate(); len(errs) > 0 {
		t.Errorf("the source tree's manifest: %v", errs)
	}
}

func TestReadFixture(t *testing.T) {
	tr := load(t, fixture(t))
	var got []string
	for _, e := range tr.Entries {
		v := "="
		if e.Next {
			v = "~"
		}
		got = append(got, e.Family+"/"+e.Name+v+itoa(e.Value)+" "+e.Status.String())
	}
	want := []string{
		"CounterKind/CtrTicket=4 reserved",
		"CounterKind/CtrVoyage~5 active",
		"ExtOp/ExtBar=5 active",
		"ExtOp/ExtFoo~2 reserved",
		"Op/OpHot~158 active",
		"TrigEvent/TrigFoo=3 blocked objspace:tcode",
		"TrigFlags2/Tf2New~2 reserved",
		"key/keyMutate=0 deferred waits on the keyed arena",
		"mech/mechFooBar~55 reserved",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("entries:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if v := tr.Entry(Dir + "/CounterKind/CtrVoyage"); v == nil || strings.Join(v.Forge, ",") != "VOYAGE,Voyage" {
		t.Errorf("CtrVoyage forge tokens: %+v", v)
	}
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

// TestInactiveCodeIsNamed is invariant 3's lookup: a table code whose entry is
// not active has a refusal class naming the entry, an active one has none,
// and a code no entry holds is not the manifest's.
func TestInactiveCodeIsNamed(t *testing.T) {
	tr := load(t, fixture(t))
	for _, c := range []struct {
		family string
		value  int64
		want   string
	}{
		{"CounterKind", 4, "manifest:reserved:CounterKind/CtrTicket"},
		{"CounterKind", 5, ""}, // CtrVoyage, active at its provisional value
		{"TrigEvent", 3, "manifest:blocked:TrigEvent/TrigFoo"},
		{"key", 0, "manifest:deferred:key/keyMutate"},
		{"ExtOp", 5, ""},
	} {
		e := tr.Lookup(c.family, c.value)
		if e == nil {
			t.Errorf("Lookup(%s, %d) = nil", c.family, c.value)
			continue
		}
		if got := e.RefusalClass(); got != c.want {
			t.Errorf("Lookup(%s, %d).RefusalClass() = %q, want %q", c.family, c.value, got, c.want)
		}
	}
	if e := tr.Lookup("CounterKind", 0); e != nil {
		t.Errorf("Lookup(CounterKind, 0) = %s, want nil: CtrP1P1 is a hand constant", e.Path)
	}
}

// TestValidateRejects is invariant 1 on one tree, and the format's own rules:
// each case edits the fixture and names the error it must raise.
func TestValidateRejects(t *testing.T) {
	ctr := Dir + "/CounterKind/CtrTicket"
	cases := []struct {
		name string
		edit func(m fstest.MapFS)
		want string
	}{
		{"unknown key", set(ctr, "family: CounterKind\nname: CtrTicket\nvalue: 4\nforge: TICKET\nstatus: reserved\ncolour: red\n"), `unknown key "colour"`},
		{"duplicate key", set(ctr, "family: CounterKind\nname: CtrTicket\nvalue: 4\nvalue: 5\nforge: TICKET\nstatus: reserved\n"), "value given twice"},
		{"missing status", set(ctr, "family: CounterKind\nname: CtrTicket\nvalue: 4\nforge: TICKET\n"), "no status line"},
		{"bad value", set(ctr, "family: CounterKind\nname: CtrTicket\nvalue: 0x4\nforge: TICKET\nstatus: reserved\n"), "not a decimal integer or next"},
		{"bad status", set(ctr, "family: CounterKind\nname: CtrTicket\nvalue: 4\nforge: TICKET\nstatus: done\n"), "is not reserved, active"},
		{"blocked without why", set(ctr, "family: CounterKind\nname: CtrTicket\nvalue: 4\nforge: TICKET\nstatus: blocked\n"), "needs a reason"},
		{"name not file", set(ctr, "family: CounterKind\nname: CtrTickets\nvalue: 4\nforge: TICKET\nstatus: reserved\n"), "belong in dirac/manifest/CounterKind/CtrTickets"},
		{"family not dir", set(ctr, "family: ExtOp\nname: CtrTicket\nvalue: 4\nstatus: reserved\n"), "belong in dirac/manifest/ExtOp/CtrTicket"},
		{"unknown family dir", set(Dir+"/Widget/WFoo", "family: Widget\nname: WFoo\nvalue: 1\nstatus: reserved\n"), `"Widget" is not a manifest family`},
		{"prefix", set(Dir+"/CounterKind/Ticket", "family: CounterKind\nname: Ticket\nvalue: 9\nforge: T\nstatus: reserved\n"), `starts with "Ctr"`},
		{"unexported kop", set(Dir+"/ExtOp/extFoo", "family: ExtOp\nname: extFoo\nvalue: 9\nstatus: reserved\n"), "must be exported"},
		{"exported dirac", set(Dir+"/key/KeyFoo", "family: key\nname: KeyFoo\nvalue: 9\nstatus: reserved\n"), "must be unexported"},
		{"no forge counter", set(ctr, "family: CounterKind\nname: CtrTicket\nvalue: 4\nstatus: reserved\n"), "needs a forge line"},
		{"mech width", set(Dir+"/mech/mechFooBar", "family: mech\nname: mechFooBar\nvalue: next\nstate: code 0\nstatus: reserved\n"), "slotbit or objbit"},
		{"out of range", set(Dir+"/ExtOp/ExtBar", "family: ExtOp\nname: ExtBar\nvalue: 321\nstatus: active\n"), "outside ExtOp's range [0,320]"},
		{"not one bit", set(Dir+"/TrigFlags2/Tf2New", "family: TrigFlags2\nname: Tf2New\nvalue: 6\nstatus: reserved\n"), "single bit"},
		{"reserved band", set(Dir+"/Op/OpHot", "family: Op\nname: OpHot\nvalue: 200\nstatus: active\n"), "reserved band [158,249]"},
		{"reserved next", set(Dir+"/Op/OpHot", "family: Op\nname: OpHot\nvalue: next\nstatus: active\n"), "add an approval line"},
		{"takes a hand value", set(Dir+"/ExtOp/ExtBar", "family: ExtOp\nname: ExtBar\nvalue: 3\nstatus: active\n"), "value 3 is already dirac/kop/testdata/ordinals.tsv ExtB"},
		{"takes a hand bit", set(Dir+"/TrigFlags2/Tf2New", "family: TrigFlags2\nname: Tf2New\nvalue: 4\nstatus: reserved\n"), "already dirac/kop/testdata/ordinals.tsv Tf2B"},
		{"two entries one value", set(Dir+"/CounterKind/CtrOther", "family: CounterKind\nname: CtrOther\nvalue: 4\nforge: OTHER\nstatus: reserved\n"), "value 4 is already dirac/manifest/CounterKind/CtrOther"},
		{"name of another type", set(Dir+"/ExtOp/ExtA2", "family: ExtOp\nname: ExtA2\nvalue: 9\nstatus: reserved\n"), ""},
		{"hand name, other type", set(Dir+"/TrigEvent/TrigETBX", "family: TrigEvent\nname: TrigETBX\nvalue: 9\nstatus: reserved\n"), ""},
		{"name clash with ordinals", func(m fstest.MapFS) {
			m[OrdinalsPath].Data = append(append([]byte{}, m[OrdinalsPath].Data...), "CtrTicket\tExtOp\t77\n"...)
		}, "already a ExtOp constant"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := fixture(t)
			c.edit(m)
			_, err := Load(m)
			if c.want == "" {
				if err != nil {
					t.Fatalf("want no error, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("got %v, want an error containing %q", err, c.want)
			}
		})
	}
}

func set(p, body string) func(fstest.MapFS) {
	return func(m fstest.MapFS) { m[p] = &fstest.MapFile{Data: []byte(body)} }
}

// TestResolveIsLowestFreeInOrder: Next entries take the lowest free values in
// (family, name) order, each counting the ones before it.
func TestResolveIsLowestFreeInOrder(t *testing.T) {
	m := fixture(t)
	set(Dir+"/ExtOp/ExtAaa", "family: ExtOp\nname: ExtAaa\nvalue: next\nstatus: reserved\n")(m)
	set(Dir+"/ExtOp/ExtZzz", "family: ExtOp\nname: ExtZzz\nvalue: next\nstatus: reserved\n")(m)
	tr := load(t, m)
	got := map[string]int64{}
	for _, e := range tr.Entries {
		if e.Family == "ExtOp" {
			got[e.Name] = e.Value
		}
	}
	// held: 0 1 3 321 (ordinals) and 5 (ExtBar); in name order ExtAaa,
	// ExtFoo, ExtZzz take 2, 4, 6.
	want := map[string]int64{"ExtAaa": 2, "ExtBar": 5, "ExtFoo": 4, "ExtZzz": 6}
	var names []string
	for n := range want {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if got[n] != want[n] {
			t.Errorf("%s = %d, want %d", n, got[n], want[n])
		}
	}
}

// TestOwnRowTakesNoValue: once ordinals.tsv records a Next entry's generated
// constant at its provisional value, the entry keeps that value rather than
// stepping past its own row.
func TestOwnRowTakesNoValue(t *testing.T) {
	m := fixture(t)
	m[OrdinalsPath].Data = append(append([]byte{}, m[OrdinalsPath].Data...), "ExtFoo\tExtOp\t2\n"...)
	if v := load(t, m).Entry(Dir + "/ExtOp/ExtFoo").Value; v != 2 {
		t.Errorf("ExtFoo = %d, want 2", v)
	}
}

func TestNoManifestDirIsEmpty(t *testing.T) {
	m := fstest.MapFS{OrdinalsPath: {Data: []byte("OpNop\tOp\t0\n")}}
	if tr := load(t, m); len(tr.Entries) != 0 {
		t.Errorf("entries %d, want 0", len(tr.Entries))
	}
}

func TestSkipsPackageFilesAndTestdata(t *testing.T) {
	m := fixture(t)
	m[Dir+"/doc.go"] = &fstest.MapFile{Data: []byte("package manifest\n")}
	m[Dir+"/testdata/x/y"] = &fstest.MapFile{Data: []byte("junk\n")}
	m[Dir+"/CounterKind/.CtrTicket.swp"] = &fstest.MapFile{Data: []byte("junk\n")}
	if n := len(load(t, m).Entries); n != 9 {
		t.Errorf("entries %d, want 9", n)
	}
}
