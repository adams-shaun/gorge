package manifest

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// seedInputs reads a census, classes and alias file.
func seedInputs(t *testing.T, census, classes, aliases string) ([]CensusRow, []ClassRow, []Alias) {
	t.Helper()
	open := func(p string) *os.File {
		f, err := os.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { f.Close() })
		return f
	}
	rows, err := ReadCensus(census, open(census))
	if err != nil {
		t.Fatal(err)
	}
	cls, err := ReadClasses(classes, open(classes))
	if err != nil {
		t.Fatal(err)
	}
	as, err := ReadAliases(aliases, open(aliases))
	if err != nil {
		t.Fatal(err)
	}
	return rows, cls, as
}

func fixtureSeedInputs(t *testing.T) ([]CensusRow, []ClassRow, []Alias) {
	return seedInputs(t, "testdata/seed/census.tsv", "testdata/seed/classes.tsv", "testdata/seed/aliases.tsv")
}

// TestSeedFixture seeds the fixture census into the fixture tree: the
// not-lowered items get entries in (cards_all desc, token) order at the
// lowest free values, aliases and exclusions and items with an entry are
// skipped, objspace and unbounded items are blocked, the bounded State keys
// come from the classes table, and a second seed adds nothing.
func TestSeedFixture(t *testing.T) {
	root := copyTree(t)
	tr := load(t, os.DirFS(root))
	census, classes, aliases := fixtureSeedInputs(t)
	s, err := PlanSeed(tr, census, classes, aliases, nil)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range s.Entries {
		got = append(got, strings.TrimPrefix(e.Path, Dir+"/")+" "+strconv.FormatInt(e.Value, 10)+" "+e.Status.String()+" | "+e.State)
	}
	want := []string{
		"CounterKind/CtrIce 5 reserved | code 0",
		"CounterKind/CtrBounty 6 reserved | code 0",
		"CounterKind/CtrFlood 7 reserved | code 0",
		"CounterKind/CtrAcquiredTaste 8 reserved | code 0",
		"ExtOp/ExtMakeCard 2 blocked objspace:tcode | objspace (outside-game cards)",
		"ExtOp/ExtChooseSource 4 blocked unbounded:prevention/effect-records | unbounded (prevention/effect records)",
		"ExtOp/ExtClash 6 reserved | code 0",
		"KwID/KwBanding 32 reserved | slotbyte 112/112 (banding)",
		"KwID/KwAttacksAlone 33 reserved | code 0",
		"TrigEvent/TrigCrewed 4 reserved | slotbit 16/16 (crewed)",
		"key/keyBanding 1 reserved | slotbyte 112/112",
		"key/keyCrewed 2 reserved | slotbit 16/16",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("seed:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	var skipped []string
	for _, k := range s.Skipped {
		skipped = append(skipped, k.Kind+" "+k.Item+": "+k.Why)
	}
	sort.Strings(skipped)
	wantSkipped := []string{
		"api Bar: has dirac/manifest/ExtOp/ExtBar",
		"ctr EachNotOn: exclude: not a counter type",
		"ctr M1M1: alias CtrM1M1: an existing kind",
		"ctr TICKET: has dirac/manifest/CounterKind/CtrTicket",
		"key turn tallies: exclude: a category",
		"kw Trample: exclude: partial",
	}
	if strings.Join(skipped, "\n") != strings.Join(wantSkipped, "\n") {
		t.Errorf("skipped:\n%s\nwant:\n%s", strings.Join(skipped, "\n"), strings.Join(wantSkipped, "\n"))
	}
	if n := s.Counts(); n["CounterKind"] != 4 || n["key"] != 2 || len(n) != 5 {
		t.Errorf("counts %v", n)
	}

	e := s.Entries[0]
	wantFile := "family: CounterKind\nname: CtrIce\nvalue: 5\nforge: ICE\ncensus: ctr ICE all=7 std=0 mbc=0 sole=0\nstate: code 0\nstatus: reserved\n"
	if string(e.Format()) != wantFile {
		t.Errorf("CtrIce's file:\n%s\nwant:\n%s", e.Format(), wantFile)
	}
	if err := WriteSeed(root, s); err != nil {
		t.Fatal(err)
	}
	after := load(t, os.DirFS(root))
	if len(after.Entries) != len(tr.Entries)+len(s.Entries) {
		t.Errorf("after the seed the tree has %d entries, want %d", len(after.Entries), len(tr.Entries)+len(s.Entries))
	}
	again, err := PlanSeed(after, census, classes, aliases, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Entries) != 0 {
		t.Errorf("a second seed adds %d entries, want none", len(again.Entries))
	}
	if err := WriteSeed(root, s); err == nil || !strings.Contains(err.Error(), "never edits") {
		t.Errorf("rewriting a seeded entry: %v, want a refusal", err)
	}
}

// TestSeedRefusesExistingNames: a derived name that is already an ordinals
// row, a declared identifier, another entry or another new item's name is an
// error naming the item, so the alias list gains a row for it.
func TestSeedRefusesExistingNames(t *testing.T) {
	tr := load(t, fixture(t))
	census, classes, _ := fixtureSeedInputs(t)
	row := func(kind, item string) CensusRow {
		return CensusRow{Kind: kind, Item: item, All: 1, Status: "refused", StateCat: "code"}
	}
	for _, c := range []struct {
		name     string
		extra    []CensusRow
		declared map[string]map[string]bool
		want     string
	}{
		{"ordinals row", []CensusRow{row("ctr", "CHARGE")}, nil, `ctr "CHARGE": CtrCharge is already a CounterKind constant`},
		{"declared", []CensusRow{row("api", "Helper")}, map[string]map[string]bool{"kop": {"ExtHelper": true}}, `api "Helper": ExtHelper is already declared in package kop`},
		{"declared dirac", nil, map[string]map[string]bool{"dirac": {"keyCrewed": true}}, `key "crewed": keyCrewed is already declared in package dirac`},
		{"entry", []CensusRow{row("ctr", "Voyage Two"), row("ctr", "VoyageTwo")}, nil, `CtrVoyageTwo is also the name of ctr "Voyage Two"`},
		{"other entry's name", []CensusRow{row("trig", "foo!")}, nil, `TrigFoo is already dirac/manifest/TrigEvent/TrigFoo`},
		{"pipe", []CensusRow{row("kw", "A | B")}, nil, `a forge token cannot hold |`},
	} {
		_, err := PlanSeed(tr, append(append([]CensusRow(nil), census...), c.extra...), classes, nil, c.declared)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", c.name, err, c.want)
		}
	}
}

func TestSeedAliasErrors(t *testing.T) {
	tr := load(t, fixture(t))
	census, classes, _ := fixtureSeedInputs(t)
	for _, c := range []struct {
		alias Alias
		want  string
	}{
		{Alias{Kind: "ctr", Item: "NOPE", Action: AliasExclude, Target: "-", Why: "x", Line: 3}, `aliases.tsv:3: ctr "NOPE" is not in the census`},
		{Alias{Kind: "ctr", Item: "ICE", Action: AliasExisting, Target: "CtrIce", Why: "x", Line: 4}, `aliases CtrIce, which is no ordinals row or entry`},
	} {
		_, err := PlanSeed(tr, census, classes, []Alias{c.alias}, nil)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v: %v, want %q", c.alias, err, c.want)
		}
	}
}

func TestReadAliasesErrors(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"ctr\tX\talias\tCtrX\n", "4 fields, want"},
		{"zap\tX\texclude\t-\twhy\n", `kind "zap"`},
		{"ctr\tX\tdrop\t-\twhy\n", `action "drop"`},
		{"ctr\tX\texclude\tCtrX\twhy\n", "an exclude's target is -"},
		{"ctr\tX\tname\tnot an ident\twhy\n", "not a Go identifier"},
		{"ctr\tX\texclude\t-\t\n", "needs an item and a why"},
		{"ctr\tX\texclude\t-\twhy\n# c\nctr\tX\texclude\t-\twhy\n", "is also line 1"},
	} {
		_, err := ReadAliases("a.tsv", strings.NewReader(c.in))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: %v, want %q", c.in, err, c.want)
		}
	}
}

func TestReadCensusHeader(t *testing.T) {
	if _, err := ReadCensus("c.tsv", strings.NewReader("kind\titem\n")); err == nil || !strings.Contains(err.Error(), "header") {
		t.Errorf("a wrong header: %v", err)
	}
	head := strings.Join(censusHeader, "\t") + "\n"
	if _, err := ReadCensus("c.tsv", strings.NewReader(head+"ctr\tX\t1\t0\t0\t0\t0\t0\trefused\tcode\t-\t0\n")); err == nil || !strings.Contains(err.Error(), "c.tsv:2: 12 fields") {
		t.Errorf("a short row: %v", err)
	}
	if _, err := ReadCensus("c.tsv", strings.NewReader(head+"ctr\tX\tmany\t0\t0\t0\t0\t0\trefused\tcode\t-\t0\t0\n")); err == nil || !strings.Contains(err.Error(), `"many" is not an integer`) {
		t.Errorf("a non-number: %v", err)
	}
}

func TestGoWords(t *testing.T) {
	for in, want := range map[string]string{
		"TICKET":            "Ticket",
		"P1P0":              "P1P0",
		"M0M2":              "M0M2",
		"Cumulative upkeep": "CumulativeUpkeep",
		"xPaid":             "XPaid",
		"day/night":         "DayNight",
		"must-block target": "MustBlockTarget",
		"Exalted":           "Exalted",
		"CARDNAME can't attack unless at least two other creatures attack.": "CardnameCanTAttackUnlessAtLeastTwoOtherCreaturesAttack",
		"A": "A",
	} {
		if got := goWords(in); got != want {
			t.Errorf("goWords(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestAliasesFile holds the committed alias list to the module: it parses,
// and every alias names an ordinals row of dirac/kop.
func TestAliasesFile(t *testing.T) {
	root := filepath.Join("..", "..")
	tr, err := Read(os.DirFS(root))
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(root, filepath.FromSlash(AliasesPath)))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	as, err := ReadAliases(AliasesPath, f)
	if err != nil {
		t.Fatal(err)
	}
	if len(as) < 100 {
		t.Fatalf("%s has %d rows; the reviewed list has over a hundred", AliasesPath, len(as))
	}
	for _, a := range as {
		if a.Action == AliasExisting && tr.byName[a.Target] == nil && tr.entryNamed(a.Target) == nil {
			t.Errorf("%s:%d: %s %q aliases %s, which is no constant", AliasesPath, a.Line, a.Kind, a.Item, a.Target)
		}
	}
}

// TestSeedResearchCensus seeds the research census (the census and classes
// TSVs, plain or gzip, named by KMANIFEST_CENSUS and KMANIFEST_CLASSES; it
// skips without them) into the module with the committed alias list and
// pins the entries per family. Every name is new: PlanSeed refuses one that
// is an ordinals row or a declared identifier of its package.
//
// The design's estimate per family (CounterKind 180, KwID 118, ExtOp 76,
// ExprKind 136, TrigEvent 78, StaticKind 49, ReplKind 29, Zone 3, key 38;
// 707) counted every item kgen does not lower. The alias review takes out
// the partially lowered items (an existing representation, not a new kind),
// the names that are already constants, the counter rows that are no
// permanent counter type (player counters, parser artifacts), deck
// construction text, ReplaceCount$ variables and the State key categories
// that are several keys.
func TestSeedResearchCensus(t *testing.T) {
	census, classes := os.Getenv("KMANIFEST_CENSUS"), os.Getenv("KMANIFEST_CLASSES")
	if census == "" || classes == "" {
		t.Skip("KMANIFEST_CENSUS and KMANIFEST_CLASSES name the research census; unset")
	}
	root := filepath.Join("..", "..")
	tr := load(t, os.DirFS(root))
	declared, err := Declared(root)
	if err != nil {
		t.Fatal(err)
	}
	rows, cls, as := seedInputs(t, census, classes, filepath.Join(root, filepath.FromSlash(AliasesPath)))
	s, err := PlanSeed(tr, rows, cls, as, declared)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{
		"CounterKind": 171, "KwID": 93, "ExtOp": 70, "ExprKind": 96, "TrigEvent": 76,
		"StaticKind": 47, "ReplKind": 21, "Zone": 2, "key": 32,
	}
	got := s.Counts()
	for f, n := range want {
		if got[f] != n {
			t.Errorf("%s: %d entries, want %d", f, got[f], n)
		}
	}
	if len(got) != len(want) {
		t.Errorf("families %v, want %v", got, want)
	}
	for _, e := range s.Entries {
		if e.Family == "CounterKind" && (e.Value < 82 || e.Value > 252) {
			t.Errorf("%s: value %d outside the free run 82..252", e.Path, e.Value)
		}
	}
}
