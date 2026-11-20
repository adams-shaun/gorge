package kgen

import (
	"math/bits"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// sharedPumpAllCards are the cards the SharedKeywordsZone$ PumpAll lowering
// covers: Odric, Lunarch Marshal and Concerted Effort, beside one creature per
// keyword the two list so the pool-scanned Landwalk and Protection family
// words have spellings to expand.
var sharedPumpAllCards = []string{
	"Odric, Lunarch Marshal", "Concerted Effort",
	"Bog Wraith", "River Boa", "White Knight", "Black Knight", "Troll Ascetic",
	"Serra Angel", "Giant Spider", "Kavu Predator", "Fencing Ace", "Vampire Nighthawk",
	"Raging Goblin", "Dross Prowler", "Darksteel Myr", "Kederekt Creeper",
	"Skeleton Key", "Grizzly Bears", "Plains", "Island", "Swamp", "Mountain", "Forest",
}

// sharedPumpAllProg gathers a card's lowered ability programs.
func sharedPumpAllProg(c *lCard) []lInstr {
	var out []lInstr
	for _, ab := range c.Abilities {
		out = append(out, ab.Prog...)
	}
	return out
}

// sharedPumpAllHas reports whether the program holds an instruction op.
func sharedPumpAllHas(p []lInstr, ok func(lInstr) bool) bool {
	for _, in := range p {
		if ok(in) {
			return true
		}
	}
	return false
}

// sharedPumpAllGuarded reports whether op/cs sits directly behind a SkipIf
// whose first clause matches want (one keyword's presence read).
func sharedPumpAllGuarded(p []lInstr, op, cs string, want func(lClause) bool) bool {
	for i, in := range p {
		if in.Op != op || in.CS != cs || i == 0 {
			continue
		}
		g := p[i-1]
		if g.Op != "SkipIf" || g.Cond.Kind != cPresent || g.Cond.F == nil || len(g.Cond.F.Clauses) == 0 {
			continue
		}
		if want(g.Cond.F.Clauses[0]) {
			return true
		}
	}
	return false
}

// TestSharedPumpAllForms: a PumpAll with SharedKeywordsZone$ Battlefield and
// SharedRestrictions$ lowers, with no refusal, and the lowered cards build.
// The LIR must carry the conditional grants, not merely accept the parameters:
// Odric's granted flying sits behind a presence read and its Skulk is a mark;
// Concerted Effort's bare Landwalk and Protection expand to conditional
// override records.
func TestSharedPumpAllForms(t *testing.T) {
	lw := lowerNamedPool(t, sharedPumpAllCards)
	if len(lw.cards) != len(sharedPumpAllCards) {
		t.Fatalf("lowered %d cards, want %d", len(lw.cards), len(sharedPumpAllCards))
	}
	byName := map[string]*lCard{}
	for _, c := range lw.cards {
		byName[c.Name] = c
		for _, r := range c.Refusals {
			t.Errorf("%s refused: %s", c.Name, r.Reason)
		}
	}
	odric := sharedPumpAllProg(byName["Odric, Lunarch Marshal"])
	if !sharedPumpAllGuarded(odric, "PumpAll", "Flying", func(c lClause) bool {
		return c.CmpStat == cmpHasKw && c.CmpN == bits.TrailingZeros32(uint32(kop.KwFlying))
	}) {
		t.Errorf("Odric: no flying grant behind a has-keyword presence read: %v", odric)
	}
	if !sharedPumpAllGuarded(odric, "PumpAll", "Hexproof", func(c lClause) bool {
		return c.CmpStat == cmpHasKw && c.CmpN == bits.TrailingZeros32(uint32(kop.KwHexproof))
	}) {
		t.Errorf("Odric: no hexproof grant behind a has-keyword presence read: %v", odric)
	}
	if !sharedPumpAllHas(odric, func(in lInstr) bool { return in.Op == "KwMark" && in.C == kop.KwMarkSkulk }) {
		t.Error("Odric: no Skulk mark grant")
	}
	concerted := sharedPumpAllProg(byName["Concerted Effort"])
	if !sharedPumpAllGuarded(concerted, "GrantKwAll", "Fear", func(c lClause) bool {
		return c.CmpStat == cmpHasKw && c.CmpN == bits.TrailingZeros32(uint32(kop.KwFear))
	}) {
		t.Errorf("Concerted Effort: no fear grant behind a has-keyword presence read: %v", concerted)
	}
	if !sharedPumpAllGuarded(concerted, "OverrideAll", "EOT", func(c lClause) bool {
		return c.CmpStat == cmpHasLandwalk && c.CmpFilter != nil
	}) {
		t.Errorf("Concerted Effort: no shared landwalk behind a landwalk presence read: %v", concerted)
	}
	if !sharedPumpAllGuarded(concerted, "OverrideAll", "EOT", func(c lClause) bool {
		return c.CmpStat == cmpHasProtColor || c.CmpStat == cmpHasProtQuality && c.CmpFilter != nil
	}) {
		t.Errorf("Concerted Effort: no shared protection behind a protection presence read: %v", concerted)
	}
	_, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range bs {
		for _, r := range b.refusals {
			t.Errorf("%s build refusal: %v", b.def.Name, r)
		}
	}
}
