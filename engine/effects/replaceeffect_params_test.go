package effects

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestReplaceEffectKnownKeysSorted: the unread lookup binary-searches the
// table.
func TestReplaceEffectKnownKeysSorted(t *testing.T) {
	if !slices.IsSorted(replaceEffectKnownKeys[:]) || len(slices.Compact(slices.Clone(replaceEffectKnownKeys[:]))) != len(replaceEffectKnownKeys) {
		t.Fatal("replaceEffectKnownKeys must be sorted and duplicate-free; regenerate with: go test ./engine/rules -run 'KnownKeysMatchTheCensus' -update")
	}
}

// TestCompileReplaceEffect pins the compiled shapes effReplaceEffect and
// rules' ReplaceWith$ readers use, the unread report and the zero-alloc
// front-cache hit.
func TestCompileReplaceEffect(t *testing.T) {
	sa := &cards.SA{API: "ReplaceEffect", Params: map[string]string{
		"VarName": "DamageAmount", "VarValue": " ReplaceCount$DamageAmount/Twice ", "VarType": "Card",
		"Unmodelled": "x",
	}}
	p := ReplaceEffectOf(sa)
	if p.VarName != "DamageAmount" || p.VarValue != (ParamText{Text: " ReplaceCount$DamageAmount/Twice ", Present: true}) {
		t.Fatalf("compiled = %+v", p)
	}
	// VarType is a known, deliberately ignored key: the referent kind comes
	// from the replacement code (replaceEventRefCodes), not VarType$. A key
	// with no reader anywhere still reaches the unread report.
	if !slices.Equal(p.Unread, []string{"Unmodelled"}) {
		t.Fatalf("unread = %v, want [Unmodelled] (VarType is a known ignored key)", p.Unread)
	}
	if allocs := allocsPerRun(100, func() { _ = ReplaceEffectOf(sa) }); allocs != 0 {
		t.Fatalf("ReplaceEffectOf front-cache hit allocates %v", allocs)
	}
	if d := ReplaceEffectOf(&cards.SA{API: "ReplaceEffect", Params: map[string]string{}}); d.VarName != "" || d.VarValue.Present || d.Unread != nil {
		t.Fatalf("bare body = %+v", d)
	}
}
