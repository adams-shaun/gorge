package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited"
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
// output of the default FDN run are the bytes they were before -only existed.
// The MBC default run is not pinned to a digest: its output moves with every
// card support change, and TestDraftsimOnly holds it to the full sheet-name
// list instead (the set's source list, not a committed value).
func TestDraftsimDefaultDigest(t *testing.T) {
	dir := corpusReady(t)
	want := map[string]string{
		"FDN": "10fb1252ce8a9373e4be769c5157d399cb330dd87ed7560f8d342334ed42132f",
	}
	for _, o := range defaultRuns(dir) {
		if _, ok := want[o.set]; !ok {
			continue
		}
		got, _, _ := runDigest(t, o)
		if got != want[o.set] {
			t.Errorf("%s default run digest %s, want %s", o.set, got, want[o.set])
		}
	}
}

// withoutFillReport drops the -only fill report lines, which the default run does not print.
func withoutFillReport(out string) string {
	var b strings.Builder
	for _, line := range strings.SplitAfter(out, "\n") {
		if strings.HasPrefix(line, "only: ") || strings.HasPrefix(line, "  sheet ") {
			continue
		}
		b.WriteString(line)
	}
	return b.String()
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
	_, def, defOut := runDigest(t, base)
	o := base
	o.only = all
	_, full, fullOut := runDigest(t, o)
	if string(full.events[0].log) != string(def.events[0].log) {
		t.Fatal("an -only list of every sheet name changed the event log")
	}
	if !strings.Contains(fullOut, "only: "+all+", ") {
		t.Fatalf("no fill report:\n%s", fullOut)
	}
	if got := withoutFillReport(fullOut); got != defOut {
		t.Fatal("an -only list of every sheet name changed the default MBC output outside the fill report")
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
