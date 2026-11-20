package kgen

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestClassMechanism pins the family and kind of every family. The expected
// values are the reference classifier's (repro/mech.py), checked in full by
// TestClassMechanismMatchesReferenceCensus below.
func TestClassMechanism(t *testing.T) {
	for _, c := range []struct {
		class string
		fam   string
		kind  string
	}{
		{`other:Seek Defined$ "_"`, "M0", "value"},
		{"other:token refused", "M1", "guard"},
		{"param:Cleanup.ClearImprinted", "M2", "atom"},
		{`other:Defined$ "_"`, "M3", "value"},
		{`other:Effect replacement "_"`, "M4", "value"},
		{`other:filter "_": "_": two comparisons in one clause`, "M5", "value"},
		{`other:Condition$ "_"`, "M6", "value"},
		{"kw:Soulshift", "M7", "atom"},
		{"trig:Mutates", "M8", "atom"},
		{"cost:ExiledMoveToGrave", "M9", "atom"},
		{"other:mana ability with a conditional or unbounded rider program", "M10", "guard"},
		{"other:optional ChooseCard", "M11", "guard"},
		{"api:Learn", "M12", "atom"},
		{`other:MoveCounter CounterType "_"`, "M13", "value"},
		// The reference's order decides these: keyword memory forms are M2
		// before the keyword family, and TargetType is a filter word.
		{"kw:Haunt", "M2", "atom"},
		{"kw:Champion", "M2", "atom"},
		{"param:X.TargetType", "M5", "atom"},
	} {
		fam, kind := classMechanism(c.class)
		if fam != c.fam || kind != c.kind {
			t.Errorf("classMechanism(%q) = %s/%s, want %s/%s", c.class, fam, kind, c.fam, c.kind)
		}
	}
}

// TestClassMechanismMatchesReferenceCensus checks classMechanism against the
// reference classifier's output for every paper gap-A class. The file is
// repro/mech.py's mech-classes.tsv (count, family, kind, class), copied here so
// CI has it; its per-family sums are DESIGN.md 2.3's classes and carrier-rows.
func TestClassMechanismMatchesReferenceCensus(t *testing.T) {
	b, err := os.ReadFile("testdata/mech-classes-ref.tsv")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	const wantRows = 3061
	if len(lines) != wantRows {
		t.Fatalf("reference census has %d rows, want %d", len(lines), wantRows)
	}
	type familyCount struct{ classes, carriers int }
	got := map[string]*familyCount{}
	classes := map[string]map[string]bool{}
	checked := 0
	for i, line := range lines {
		f := strings.SplitN(line, "\t", 4)
		if len(f) != 4 || f[3] == "" {
			t.Fatalf("line %d is not count<TAB>family<TAB>kind<TAB>class: %q", i+1, line)
		}
		count, err := strconv.Atoi(f[0])
		if err != nil || count < 1 {
			t.Fatalf("line %d has bad count %q", i+1, f[0])
		}
		ref := strings.Fields(f[1])
		if len(ref) == 0 || !strings.HasPrefix(ref[0], "M") {
			t.Fatalf("line %d has bad reference family %q", i+1, f[1])
		}
		wantFam, wantKind := ref[0], f[2]
		switch wantKind {
		case "guard", "value", "atom":
		default:
			t.Fatalf("line %d has bad reference kind %q", i+1, wantKind)
		}
		fam, kind := classMechanism(f[3])
		if fam != wantFam || kind != wantKind {
			t.Errorf("classMechanism(%q) = %s/%s, reference says %s/%s", f[3], fam, kind, wantFam, wantKind)
			continue
		}
		if got[fam] == nil {
			got[fam] = &familyCount{}
			classes[fam] = map[string]bool{}
		}
		if !classes[fam][f[3]] {
			classes[fam][f[3]] = true
			got[fam].classes++
		}
		got[fam].carriers += count
		checked++
	}
	if checked != wantRows {
		t.Fatalf("checked %d classes, want %d", checked, wantRows)
	}
	// DESIGN.md 2.3, paper gap A: the same classes and carrier rows.
	for _, want := range []struct {
		fam      string
		classes  int
		carriers int
	}{
		{"M2", 371, 1627},
		{"M3", 407, 1629},
		{"M4", 133, 676},
		{"M5", 454, 1275},
		{"M6", 309, 822},
		{"M7", 108, 721},
		{"M8", 394, 1285},
		{"M9", 180, 576},
		{"M10", 55, 168},
		{"M11", 68, 254},
		{"M12", 247, 645},
		{"M13", 329, 873},
	} {
		fc := got[want.fam]
		if fc == nil {
			t.Errorf("%s has no classes", want.fam)
			continue
		}
		if fc.classes != want.classes || fc.carriers != want.carriers {
			t.Errorf("%s: %d classes/%d carriers, DESIGN.md 2.3 says %d/%d",
				want.fam, fc.classes, fc.carriers, want.classes, want.carriers)
		}
		t.Logf("%s classes=%d carriers=%d", want.fam, fc.classes, fc.carriers)
	}
}

// TestParamMechanism pins the family of unread params against the reference's
// PFAM list (repro/partial.py). There is no keyword, mana or API family there,
// so every unmatched name falls through to M13.
func TestParamMechanism(t *testing.T) {
	for _, c := range []struct{ param, fam string }{
		{"RememberObjects", "M2"},
		{"Defined", "M3"},
		{"Duration", "M4"},
		{"ValidTgts", "M5"},
		{"ConditionPresent", "M6"},
		{"Mode", "M8"},
		{"Cost", "M9"},
		{"Optional", "M9"},
		{"Choice", "M11"},
		{"Conjure", "M13"},
		{"KW", "M13"},
		{"Zone", "M13"},
	} {
		if got := paramMechanism(c.param); got != c.fam {
			t.Errorf("paramMechanism(%q) = %s, want %s", c.param, got, c.fam)
		}
	}
}

// TestStdTSVMechanismColumns pins the two trailing columns: every row keeps
// ten fields, an `other:` class carries its family and kind, and a partial row
// carries its param families and `hidden`.
func TestStdTSVMechanismColumns(t *testing.T) {
	res := &stdCensusResult{tsv: []stdTSVRow{
		{set: "ALL", card: "Ok"},
		{set: "ALL", card: "Defined", node: "A:1", class: `other:Defined$ "_"`, reason: `Defined$ "X"`, label: `other:Defined$ "_"`},
		{set: "ALL", card: "Unread", node: "A:1", class: partialClass, reason: "unread after refusal [Defined ValidTgts Zone]"},
	}}
	var sb strings.Builder
	if err := writeStdTSV(&sb, res, nil); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(sb.String(), "\n"), "\n")
	if len(lines) != len(res.tsv) {
		t.Fatalf("wrote %d lines, want %d:\n%s", len(lines), len(res.tsv), sb.String())
	}
	for i, line := range lines {
		if fields := len(strings.Split(line, "\t")); fields != 10 {
			t.Errorf("line %d has %d fields, want 10: %q", i+1, fields, line)
		}
	}
	if !strings.HasSuffix(lines[1], "\tM3\tvalue") {
		t.Errorf("defined row = %q, want family/kind M3/value", lines[1])
	}
	if !strings.HasSuffix(lines[2], "\tM13,M3,M5\thidden") {
		t.Errorf("partial row = %q, want family M13,M3,M5 and kind hidden", lines[2])
	}
}

func TestPartialMechanismFamilies(t *testing.T) {
	got := partialMechanismFamilies("unread after refusal [Defined ValidTgts Zone]")
	want := "M13,M3,M5" // sorted like the reference's sorted(set(...))
	if joined := strings.Join(got, ","); joined != want {
		t.Errorf("partialMechanismFamilies = %q, want %q", joined, want)
	}
	if got := partialMechanismFamilies("not an unread reason"); len(got) != 0 {
		t.Errorf("partialMechanismFamilies(not unread) = %q, want none", got)
	}
}
