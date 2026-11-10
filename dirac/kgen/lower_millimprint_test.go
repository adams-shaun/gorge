package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// millImprintShape reports what a lowered card's abilities hold of the Mill
// Imprint$ family: the Mill with the imprint mark, a move whose filter reads
// the imprinted word, and the clearing Cleanup.
func millImprintShape(c *lCard) (mill, read, clear bool) {
	for _, a := range c.Abilities {
		for _, in := range a.Prog {
			mill = mill || in.Op == "Mill" && strings.Contains(in.CS, "ImprintMove")
			read = read || filterReadsImprinted(in.F)
			clear = clear || in.Op == "ClearImprinted"
		}
	}
	return
}

// TestMillImprintWriterAndFilter: Mill Imprint$ True opens the chain's
// imprinted set, IsImprinted in the Graveyard,Exile pick reads it, and the
// Cleanup clears it (Patient Naturalist and its siblings); outside such a
// chain, the word and the writer's other spellings refuse by name.
func TestMillImprintWriterAndFilter(t *testing.T) {
	for _, name := range []string{"Patient Naturalist", "Blanchwood Prowler", "Ravenous Gigamole", "Fallaji Archaeologist"} {
		c := lowerCorpusCard(t, name)
		for _, rf := range c.Refusals {
			t.Errorf("%s refused: %s", name, rf.Reason)
		}
		if mill, read, clear := millImprintShape(c); !mill || !read || !clear {
			t.Errorf("%s: imprinting Mill %v, IsImprinted read %v, ClearImprinted %v; want all three", name, mill, read, clear)
		}
	}
	c := lowerCorpusCard(t, "Ballad of the Black Flag")
	for _, rf := range c.Refusals {
		t.Logf("Ballad of the Black Flag refuses: %s", rf.Reason)
	}

	const src = `Name:Mill Imprint Probe
ManaCost:1 G
Types:Creature Human
PT:1/1
T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | Execute$ PrMill | TriggerDescription$ probe
SVar:PrMill:DB$ Mill | NumCards$ 3 | Defined$ You | Imprint$ True | SubAbility$ PrPick
SVar:PrPick:DB$ ChangeZone | Origin$ Graveyard,Exile | Destination$ Hand | ChangeType$ Land.YouOwn+IsImprinted | Hidden$ True | Mandatory$ True | SubAbility$ PrClean
SVar:PrClean:DB$ Cleanup | ClearImprinted$ True
Oracle:probe
`
	if rf := lowerScript(t, "Mill Imprint Probe", src); len(rf) != 0 {
		t.Fatalf("the probe refused: %v", rf)
	}
	has := func(rf []string, want string) bool { return strings.Contains(strings.Join(rf, "\n"), want) }

	noClean := strings.Replace(src, " | SubAbility$ PrClean", "", 1)
	if noClean == src {
		t.Fatal("the no-Cleanup probe was not built")
	}
	if rf := lowerScript(t, "Mill Imprint Probe", noClean); !has(rf, "imprints without an unconditional Cleanup ClearImprinted$") {
		t.Errorf("a repeatable Mill imprint without the Cleanup: refusals %v", rf)
	}

	noImp := strings.Replace(src, "Imprint$ True", "Imprint$ False", 1)
	if rf := lowerScript(t, "Mill Imprint Probe", noImp); !has(rf, `Mill Imprint$ "False"`) {
		t.Errorf("Imprint$ False: refusals %v", rf)
	}

	// IsImprinted with no writer in the chain
	noWriter := strings.Replace(src, " | Imprint$ True", "", 1)
	if noWriter == src {
		t.Fatal("the no-writer probe was not built")
	}
	if rf := lowerScript(t, "Mill Imprint Probe", noWriter); !has(rf, "IsImprinted outside a chain that writes the imprinted set") {
		t.Errorf("IsImprinted with no writer: refusals %v", rf)
	}

	// a read in another ability than the writer's
	cross := src + "A:AB$ ChangeZone | Cost$ T | Origin$ Graveyard | Destination$ Hand | ValidTgts$ Card.IsImprinted | TgtPrompt$ x | SpellDescription$ probe\n"
	cross = strings.Replace(cross, "Oracle:probe\n", "", 1) + "Oracle:probe\n"
	rf := lowerScript(t, "Mill Imprint Probe", cross)
	if len(rf) == 0 || !has(rf, "IsImprinted") {
		t.Errorf("the cross-ability read: refusals %v, want an IsImprinted refusal", rf)
	}

	// the cross-ability check on lowered instructions
	lc := &lowerCtx{out: &lCard{Name: "x"}, name: "x"}
	rd := &lFilter{Clauses: []lClause{{CmpStat: cmpWord7, CmpN: int(kop.W7Imprinted)}}}
	lc.out.Abilities = []*lAbility{
		{Index: 0, Prog: []lInstr{{Op: "Mill", CS: "ImprintMove"}, {Op: "ChooseMove", F: rd}, {Op: "ClearImprinted"}}},
		{Index: 1, Prog: []lInstr{{Op: "ChooseMove", F: rd}}},
	}
	lc.refuseCrossAbilityImprinted()
	if len(lc.out.Refusals) != 1 || lc.out.Refusals[0].Node != "ab[1]" {
		t.Errorf("card-level check: refusals %v, want only ab[1]", lc.out.Refusals)
	}
}
