package main

import (
	"crypto/sha256"
	"encoding/hex"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/booster"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// defaultRuns are the build-only runs whose bytes TestDraftsimDefaultDigest
// pins: an MBC Commander draft and an FDN limited draft, each two events.
func defaultRuns(dir string) []options {
	return []options{
		{set: "MBC", collation: "draft", format: event.FormatCommander, structure: event.StructureNone,
			seed: 1, events: 2, players: "bot", drafter: "rating-colour", builder: "best23", dir: dir},
		{set: "FDN", structure: event.StructureNone, seed: 1, events: 2, players: "bot",
			drafter: "rating-colour", builder: "best23", dir: dir},
	}
}

// runDigest is the sha256 of a run's stdout and every event log.
func runDigest(t *testing.T, o options) (string, *result, string) {
	t.Helper()
	var out strings.Builder
	res, err := run(o, &out)
	if err != nil {
		t.Fatalf("run %s: %v\n%s", o.set, err, out.String())
	}
	h := sha256.New()
	h.Write([]byte(out.String()))
	for _, er := range res.events {
		h.Write(er.log)
	}
	return hex.EncodeToString(h.Sum(nil)), res, out.String()
}

// TestDraftsimDefaultDigest: without -only the packs, picks, builds and
// output of the default runs are the bytes they were before -only existed.
func TestDraftsimDefaultDigest(t *testing.T) {
	dir := corpusReady(t)
	want := map[string]string{
		"MBC": "13624c877213d1945965de063e4c9057dc7a53c5fc6e854adf76e98439270f37",
		"FDN": "10fb1252ce8a9373e4be769c5157d399cb330dd87ed7560f8d342334ed42132f",
	}
	for _, o := range defaultRuns(dir) {
		got, _, _ := runDigest(t, o)
		if got != want[o.set] {
			t.Errorf("%s default run digest %s, want %s", o.set, got, want[o.set])
		}
	}
}

// sheetNamesOf is every name on spec's sheets, sorted.
func sheetNamesOf(spec setdata.Spec) []string {
	seen := map[string]bool{}
	for _, sh := range spec.Config.Sheets {
		for _, c := range sh.Cards {
			seen[c.Name] = true
		}
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// dealt is every card name an event log's packs dealt.
func dealt(t *testing.T, log []byte) []limited.CardRef {
	t.Helper()
	entries, err := event.ParseLog(log)
	if err != nil {
		t.Fatal(err)
	}
	var out []limited.CardRef
	for _, en := range entries {
		if en.Kind == event.KindPackOpened {
			out = append(out, en.Cards...)
		}
	}
	return out
}

// TestDraftsimOnly: an -only list that names every sheet card reproduces
// the default MBC run byte for byte but for the fill report. A list of
// every other name, with nothing of the newCommander sheet, deals only
// listed names; newCommander borrows the listed rares of the other sheets;
// every sheet gets a fill line; two runs are identical, and -verify
// replays.
func TestDraftsimOnly(t *testing.T) {
	dir := corpusReady(t)
	spec, err := setdata.LoadCollation("MBC", "draft")
	if err != nil {
		t.Fatal(err)
	}
	names := sheetNamesOf(spec)
	tmp := t.TempDir()
	all := filepath.Join(tmp, "all.txt")
	if err := os.WriteFile(all, []byte("# every sheet name\n"+strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := defaultRuns(dir)[0]
	base.events = 1
	_, def, _ := runDigest(t, base)
	o := base
	o.only = all
	_, full, fullOut := runDigest(t, o)
	if string(full.events[0].log) != string(def.events[0].log) {
		t.Fatal("an -only list of every sheet name changed the event log")
	}
	if !strings.Contains(fullOut, "only: "+all+", ") {
		t.Fatalf("no fill report:\n%s", fullOut)
	}

	newCmd := map[string]bool{}
	for _, c := range spec.Config.Sheets["newCommander"].Cards {
		newCmd[c.Name] = true
	}
	var lines []string
	allowed := onlyList{}
	for i, n := range names {
		if i%2 == 0 && !newCmd[n] {
			lines = append(lines, n+"\ttrue")
			allowed[n] = true
		} else {
			lines = append(lines, n+"\tfalse")
		}
	}
	half := filepath.Join(tmp, "half.tsv")
	if err := os.WriteFile(half, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	o = base
	o.only, o.verify, o.events = half, true, 2
	d1, res, out := runDigest(t, o)
	d2, _, _ := runDigest(t, o)
	if d1 != d2 {
		t.Fatalf("two -only runs differ: %s, %s", d1, d2)
	}
	t.Log(out)
	for _, er := range res.events {
		for _, c := range dealt(t, er.log) {
			if !allowed.allows(c.Name) {
				t.Fatalf("dealt %q, which the list does not name", c.Name)
			}
		}
	}
	for name := range spec.Config.Sheets {
		if !strings.Contains(out, "  sheet "+name+": ") {
			t.Errorf("no fill line for sheet %s", name)
		}
	}
	if !strings.Contains(out, "  sheet newCommander: 0/60 entries allowed (0.0%, 0.0% of weight), slot 1, uniform over 0 own + ") {
		t.Errorf("newCommander did not borrow:\n%s", out)
	}
}

// TestRestrictSpecFallback: on a synthetic spec, a sheet with no listed
// entry deals uniformly from the listed entries of its rarities on the
// other sheets; one that cannot fill even so is emptied, and a list that
// empties every variant is refused.
func TestRestrictSpecFallback(t *testing.T) {
	card := func(name string, r limited.Rarity, w int) setdata.SheetCard {
		return setdata.SheetCard{UUID: name, Name: name, Rarity: r, Weight: w}
	}
	spec := setdata.Spec{Code: "TST", Config: setdata.Config{
		TotalWeight: 3,
		Variants: []setdata.Variant{
			{Weight: 2, Contents: map[string]int{"common": 2, "rare": 1}},
			{Weight: 1, Contents: map[string]int{"common": 2, "mythic": 1}},
		},
		Sheets: map[string]setdata.Sheet{
			"common": {TotalWeight: 6, Cards: []setdata.SheetCard{card("c1", 'C', 1), card("c2", 'C', 2), card("c3", 'C', 3)}},
			"rare":   {TotalWeight: 2, Cards: []setdata.SheetCard{card("r1", 'R', 1), card("r2", 'R', 1)}},
			"mythic": {TotalWeight: 1, Cards: []setdata.SheetCard{card("m1", 'M', 1)}},
			"bonus":  {TotalWeight: 9, Cards: []setdata.SheetCard{card("r3", 'R', 9), card("c4", 'C', 9)}},
		},
	}}
	got, fills, err := restrictSpec(spec, onlyList{"c1": true, "c3": true, "r3": true, "c4": true})
	if err != nil {
		t.Fatal(err)
	}
	cards := func(name string) string {
		var b []string
		for _, c := range got.Config.Sheets[name].Cards {
			b = append(b, c.Name+":"+string(rune('0'+c.Weight)))
		}
		return strings.Join(b, ",")
	}
	if c := cards("common"); c != "c1:1,c3:3" {
		t.Errorf("common = %s", c)
	}
	if c := cards("rare"); c != "r3:1" {
		t.Errorf("rare = %s, want the bonus sheet's listed rare at weight 1", c)
	}
	if c := cards("mythic"); c != "" {
		t.Errorf("mythic = %s, want empty (no listed mythic anywhere)", c)
	}
	byName := map[string]sheetFill{}
	for _, f := range fills {
		byName[f.sheet] = f
	}
	if f := byName["rare"]; f.borrowed != 1 || f.dead || f.rarities != "R" {
		t.Errorf("rare fill %+v", f)
	}
	if f := byName["mythic"]; !f.dead {
		t.Errorf("mythic fill %+v, want dead", f)
	}
	if f := byName["common"]; f.allowed != 2 || f.allowedWeight != 4 || f.weight != 6 || f.need != 2 {
		t.Errorf("common fill %+v", f)
	}
	if spec.Config.Sheets["rare"].Cards[0].Name != "r1" {
		t.Error("restrictSpec changed its input")
	}
	// Packs from the cut spec: the mythic variant opens as the rare one.
	swaps := 0
	for seed := uint64(0); seed < 30; seed++ {
		pack, subs, err := booster.OpenLog(got, rand.New(rand.NewPCG(seed, 0)), &booster.IDs{}, nil)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		swaps += len(subs)
		var b []string
		for _, c := range pack {
			b = append(b, c.Name)
		}
		if p := strings.Join(b, ","); len(pack) != 3 || !strings.HasSuffix(p, ",r3") || strings.Contains(p, "c2") {
			t.Fatalf("seed %d pack %s, want two of c1/c3 and r3", seed, p)
		}
	}
	if swaps == 0 {
		t.Error("no pack opened the mythic variant as another")
	}
	if _, _, err := restrictSpec(spec, onlyList{"m1": true}); err == nil {
		t.Error("a list that empties every variant was accepted")
	}
}

// TestReadOnly: plain names and kgen -supported rows, comments and blanks
// skipped, false rows dropped, a first face matching a split card.
func TestReadOnly(t *testing.T) {
	p := filepath.Join(t.TempDir(), "l.tsv")
	if err := os.WriteFile(p, []byte("# c\nLightning Bolt\n\nGrizzly Bears\ttrue\nShock\tfalse\nFire\ttrue\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	l, err := readOnly(p)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]bool{"Lightning Bolt": true, "Grizzly Bears": true, "Shock": false, "Fire // Ice": true, "# c": false} {
		if l.allows(name) != want {
			t.Errorf("allows(%q) = %v", name, !want)
		}
	}
	if err := os.WriteFile(p, []byte("Shock\tmaybe\n"), 0o644); err == nil {
		if _, err := readOnly(p); err == nil {
			t.Error("a bad second field was accepted")
		}
	}
}
