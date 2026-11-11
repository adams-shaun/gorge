package cards

import (
	"reflect"
	"testing"
)

func parseFixture(t *testing.T, src string) *Face {
	t.Helper()
	c, diags := ParseBytes("fixture.txt", []byte(src))
	if c == nil || len(c.Faces) == 0 {
		t.Fatalf("fixture did not parse: %v", diags)
	}
	c.Link()
	return c.Faces[0]
}

// TestNeedsToPlayVarHintIsNotAReference: an SVar read only by the AI hint
// SVar:NeedsToPlayVar reports no value head (Etched Oracle's shape), while
// the same SVar read by an ability parameter still does.
func TestNeedsToPlayVarHintIsNotAReference(t *testing.T) {
	hint := parseFixture(t, "Name:Hint Fixture\nManaCost:4\nTypes:Artifact\n"+
		"A:AB$ Draw | Cost$ 1 | NumCards$ 3 | SpellDescription$ Draw three.\n"+
		"SVar:NeedsToPlayVar:Q GE3\nSVar:Q:Count$UniqueManaColorsProduced\nOracle:x\n")
	if _, ok := hint.SVars["NeedsToPlayVar"]; !ok {
		t.Fatal("precondition: the fixture carries the NeedsToPlayVar hint")
	}
	if got := hint.ValueHeads(); len(got) != 0 {
		t.Fatalf("hint-only ValueHeads() = %v, want none", got)
	}
	read := parseFixture(t, "Name:Read Fixture\nManaCost:4\nTypes:Artifact\n"+
		"A:AB$ Draw | Cost$ 1 | NumCards$ Q | SpellDescription$ Draw.\n"+
		"SVar:NeedsToPlayVar:Q GE3\nSVar:Q:Count$UniqueManaColorsProduced\nOracle:x\n")
	if got, want := read.ValueHeads(), []string{"count:UniqueManaColorsProduced"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ability-read ValueHeads() = %v, want %v", got, want)
	}
}

// TestBeginTurnShapeGate: the implemented ExtraTurn$ True | Skip$ True shape
// is labelled repl:BeginTurn; a normal-turn optional skip with a ReplaceWith$
// body (Time Vault's shape) is repl:BeginTurn.OtherShape.
func TestBeginTurnShapeGate(t *testing.T) {
	skip := parseFixture(t, "Name:Skip Fixture\nManaCost:2\nTypes:Enchantment\n"+
		"R:Event$ BeginTurn | ActiveZones$ Battlefield | ValidPlayer$ Opponent | ExtraTurn$ True | Skip$ True | Description$ x\nOracle:x\n")
	if got := skip.Primitives(); !containsPrimitive(got, "repl:BeginTurn") || containsPrimitive(got, "repl:BeginTurn.OtherShape") {
		t.Fatalf("extra-turn skip primitives = %v", got)
	}
	vault := parseFixture(t, "Name:Vault Fixture\nManaCost:2\nTypes:Artifact\n"+
		"R:Event$ BeginTurn | ActiveZones$ Battlefield | ValidPlayer$ You | IsPresent$ Card.Self+tapped | Optional$ True | ReplaceWith$ VaultUntap | Description$ x\n"+
		"SVar:VaultUntap:DB$ Untap | Defined$ Self\nOracle:x\n")
	if got := vault.Primitives(); !containsPrimitive(got, "repl:BeginTurn.OtherShape") || containsPrimitive(got, "repl:BeginTurn") {
		t.Fatalf("normal-turn skip primitives = %v", got)
	}
}

// TestCopySpellShapeGate: the Twinning Staff body parses to its Plus.N extra
// and is labelled repl:CopySpell; an unparseable count body is
// repl:CopySpell.OtherShape.
func TestCopySpellShapeGate(t *testing.T) {
	const line = "R:Event$ CopySpell | ActiveZones$ Battlefield | ValidPlayer$ You | ValidSpell$ Spell | ReplaceWith$ ExtraCopy | Description$ x\n" +
		"SVar:ExtraCopy:DB$ ReplaceEffect | VarName$ Amount | VarValue$ N\n"
	good := parseFixture(t, "Name:Copy Fixture\nManaCost:3\nTypes:Artifact\n"+line+"SVar:N:ReplaceCount$Amount/Plus.1\nOracle:x\n")
	if n, ok := CopySpellExtraOf(good, good.Repls[0]); !ok || n != 1 {
		t.Fatalf("CopySpellExtraOf = %d, %v; want 1, true", n, ok)
	}
	if got := good.Primitives(); !containsPrimitive(got, "repl:CopySpell") || containsPrimitive(got, "repl:CopySpell.OtherShape") {
		t.Fatalf("parsed shape primitives = %v", got)
	}
	bad := parseFixture(t, "Name:Copy Fixture\nManaCost:3\nTypes:Artifact\n"+line+"SVar:N:ReplaceCount$Amount/Twice\nOracle:x\n")
	if _, ok := CopySpellExtraOf(bad, bad.Repls[0]); ok {
		t.Fatal("precondition: the Twice body must not parse")
	}
	if got := bad.Primitives(); !containsPrimitive(got, "repl:CopySpell.OtherShape") || containsPrimitive(got, "repl:CopySpell") {
		t.Fatalf("unparseable shape primitives = %v", got)
	}
}
