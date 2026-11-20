package kgen

import (
	"strings"
	"testing"
)

// TestClassMechanism pins the family and kind of every family, using classes
// from the live ALL census (one per family).
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
		{`other:filter "_": "_": two comparisons in one clause`, "M5", "guard"},
		{`other:Condition$ "_"`, "M6", "value"},
		{"kw:Soulshift", "M7", "atom"},
		{"trig:ChaosEnsues", "M8", "atom"},
		{"cost:ExiledMoveToGrave", "M9", "atom"},
		{"other:mana ability with a conditional or unbounded rider program", "M10", "guard"},
		{"other:optional ChooseCard", "M11", "guard"},
		{"api:MakeCard", "M12", "atom"},
		{`other:MoveCounter CounterType "_"`, "M13", "value"},
	} {
		fam, kind := classMechanism(c.class)
		if fam != c.fam || kind != c.kind {
			t.Errorf("classMechanism(%q) = %s/%s, want %s/%s", c.class, fam, kind, c.fam, c.kind)
		}
	}
}

// TestParamMechanism pins the family of one param name per family.
func TestParamMechanism(t *testing.T) {
	for _, c := range []struct{ param, fam string }{
		{"Seek", "M0"},
		{"TokenOwner", "M1"},
		{"RememberObjects", "M2"},
		{"Defined", "M3"},
		{"Duration", "M4"},
		{"ValidTgts", "M5"},
		{"ConditionPresent", "M6"},
		{"KW", "M7"},
		{"Mode", "M8"},
		{"Cost", "M9"},
		{"Mana", "M10"},
		{"Optional", "M11"},
		{"Zone", "M12"},
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
	if !strings.HasSuffix(lines[2], "\tM12,M3,M5\thidden") {
		t.Errorf("partial row = %q, want family M12,M3,M5 and kind hidden", lines[2])
	}
}

func TestPartialMechanismFamilies(t *testing.T) {
	got := partialMechanismFamilies("unread after refusal [Defined ValidTgts Zone]")
	want := "M12,M3,M5" // sorted like the reference's sorted(set(...))
	if joined := strings.Join(got, ","); joined != want {
		t.Errorf("partialMechanismFamilies = %q, want %q", joined, want)
	}
	if got := partialMechanismFamilies("not an unread reason"); len(got) != 0 {
		t.Errorf("partialMechanismFamilies(not unread) = %q, want none", got)
	}
}
