package kgen

import (
	"strings"
	"testing"
)

// TestDefinedSetDistinctQualifiers: two sets that differ only in a named
// qualifier stay two sets, print two SET lines, and bind two SetRefs.
func TestDefinedSetDistinctQualifiers(t *testing.T) {
	c, lir := definedSetLIR(t, definedSetSrc(
		"A:AB$ Pump | Cost$ T | Defined$ Self.namedGrizzly Bears | NumAtt$ +1 | SubAbility$ ProbeDBTap",
		"SVar:ProbeDBTap:DB$ Tap | Defined$ Self.namedSavannah Lions"))
	if len(c.Refusals) != 0 {
		t.Fatalf("refused: %s\n%s", abReasons(c), lir)
	}
	if len(c.Sets) != 2 {
		t.Fatalf("want 2 sets, got %d:\n%s", len(c.Sets), lir)
	}
	if !strings.Contains(lir, "Grizzly") || !strings.Contains(lir, "Savannah") {
		t.Fatalf("a SET line omits the name:\n%s", lir)
	}
	// the pool holds both names, so the two filters' PredN differ
	bears := abLower(t, "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\n", false)
	lions := abLower(t, "Name:Savannah Lions\nManaCost:W\nTypes:Creature Cat\nPT:2/1\n", false)
	bears.Name, lions.Name = "Grizzly Bears", "Savannah Lions" // abLower names every card Probe
	b := buildCard(newPoolTables(&lowered{cards: []*lCard{c, bears, lions}}, nil), c)
	if len(b.refusals) != 0 {
		t.Fatalf("build refused: %+v", b.refusals)
	}
	var refs []uint16
	for _, ab := range b.def.Abilities {
		for _, r := range ab.SetRefs {
			refs = append(refs, uint16(r.Set))
		}
	}
	if len(refs) != 2 || refs[0] == refs[1] {
		t.Fatalf("want two distinct SetRefs, got %v", refs)
	}
}
