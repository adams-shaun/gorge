package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// urzaFixture returns a 2-seat game with the three Urza lands on the
// battlefield, each authored with the corpus shape -- an A:AB$ Mana ability
// whose Amount$ names an SVar that is a Count$UrzaLands expression. The
// plant's Name is "Urza's Power Plant" (no hyphen) while its Types line
// carries the hyphenated "Urza's Power-Plant" subtype, reproducing the
// corpus's own spelling split so the test pins which string the count
// matches. optionalCtrl, when given, overrides the controller of the Nth
// land (order: mine, tower, plant) -- other lands default to seat 0.
func urzaFixture(t *testing.T, ctrls ...state.PlayerID) (*state.Game, map[string]state.ObjID) {
	t.Helper()
	g := state.NewGame([]string{"you", "them"})
	ctrl := func(i int) state.PlayerID {
		if i < len(ctrls) {
			return ctrls[i]
		}
		return 0
	}
	mineSrc := "Name:Urza's Mine\nTypes:Land Urza's Mine\n" +
		"A:AB$ Mana | Cost$ T | Produced$ C | Amount$ UrzaAmount | SpellDescription$ Add {C}. If you control an Urza's Power-Plant and an Urza's Tower, add {C}{C} instead.\n" +
		"SVar:UrzaAmount:Count$UrzaLands.2.1\nOracle:x\n"
	towerSrc := "Name:Urza's Tower\nTypes:Land Urza's Tower\n" +
		"A:AB$ Mana | Cost$ T | Produced$ C | Amount$ UrzaAmount | SpellDescription$ Add {C}. If you control an Urza's Mine and an Urza's Power-Plant, add {C}{C}{C} instead.\n" +
		"SVar:UrzaAmount:Count$UrzaLands.3.1\nOracle:x\n"
	plantSrc := "Name:Urza's Power Plant\nTypes:Land Urza's Power-Plant\n" +
		"A:AB$ Mana | Cost$ T | Produced$ C | Amount$ UrzaAmount | SpellDescription$ Add {C}. If you control an Urza's Mine and an Urza's Tower, add {C}{C} instead.\n" +
		"SVar:UrzaAmount:Count$UrzaLands.2.1\nOracle:x\n"

	mk := func(order int, src string) state.ObjID {
		c, d := cards.ParseBytes("t.txt", []byte(src))
		if len(d) != 0 {
			t.Fatalf("diags: %v", d)
		}
		c.Link()
		for _, f := range c.Faces {
			f.ApplyIntrinsics()
		}
		o := g.AddObject(c, 0)
		o.Zone = state.ZBattlefield
		o.Controller = ctrl(order)
		g.SetZone(state.ZBattlefield, 0, append(g.Zone(state.ZBattlefield, 0), o.ID))
		return o.ID
	}
	return g, map[string]state.ObjID{
		"mine":  mk(0, mineSrc),
		"tower": mk(1, towerSrc),
		"plant": mk(2, plantSrc),
	}
}

// countUrza evaluates the Count$UrzaLands expression as seat ctrl, with src
// as the source object -- the same shape EvalCount sees when effMana prices
// a land's Amount$ SVar.
func countUrza(g *state.Game, ctrl state.PlayerID, src state.ObjID, expr string) int32 {
	return EvalCount(&fakeHost{g: g}, &Ctx{Controller: ctrl, Source: src}, expr)
}

// TestUrzaLandsAllThreeAssembled: when one seat controls all three lands, the
// count returns the assembled value -- Mine/Plant 2, Tower 3.
func TestUrzaLandsAllThreeAssembled(t *testing.T) {
	g, ids := urzaFixture(t)
	if n := countUrza(g, 0, ids["mine"], "Count$UrzaLands.2.1"); n != 2 {
		t.Fatalf("Mine with all three = %d, want 2", n)
	}
	if n := countUrza(g, 0, ids["tower"], "Count$UrzaLands.3.1"); n != 3 {
		t.Fatalf("Tower with all three = %d, want 3", n)
	}
	if n := countUrza(g, 0, ids["plant"], "Count$UrzaLands.2.1"); n != 2 {
		t.Fatalf("Power Plant with all three = %d, want 2", n)
	}
}

// TestUrzaLandsAmountSVarPath drives the full SVar-indirection path Num uses
// at the offer/activation site: Amount$ names an SVar whose body is the
// Count$ expression, so a literal Amount$ would never reach the head.
func TestUrzaLandsAmountSVarPath(t *testing.T) {
	g, ids := urzaFixture(t)
	src := g.Obj(ids["mine"])
	got := Num(&fakeHost{g: g}, &Ctx{Controller: 0, Source: ids["mine"], SVars: src.Face().SVars},
		src.Face().Abilities[0], "Amount", 1)
	if got != 2 {
		t.Fatalf("Num through UrzaAmount SVar = %d, want 2", got)
	}
}

// TestUrzaLandsOnlyTwo: the two present are Mine+Tower (plant missing) --
// count returns the not-assembled value, 1.
func TestUrzaLandsOnlyTwo(t *testing.T) {
	g, ids := urzaFixture(t)
	moveTo(g, ids["plant"], state.ZGraveyard)
	if n := countUrza(g, 0, ids["mine"], "Count$UrzaLands.2.1"); n != 1 {
		t.Fatalf("Mine+Tower only = %d, want 1", n)
	}
	if n := countUrza(g, 0, ids["tower"], "Count$UrzaLands.3.1"); n != 1 {
		t.Fatalf("Tower+Mine only = %d, want 1", n)
	}
}

// TestUrzaLandsOtherPair: the two present are Mine+Plant (tower missing) --
// the pair the Mine's own condition (Power-Plant+Tower) does not name. A
// count that read "assembled" from any two Urza lands, rather than all
// three, would wrongly claim assembled here.
func TestUrzaLandsOtherPair(t *testing.T) {
	g, ids := urzaFixture(t)
	moveTo(g, ids["tower"], state.ZGraveyard)
	if n := countUrza(g, 0, ids["mine"], "Count$UrzaLands.2.1"); n != 1 {
		t.Fatalf("Mine+Plant only = %d, want 1", n)
	}
	if n := countUrza(g, 0, ids["plant"], "Count$UrzaLands.2.1"); n != 1 {
		t.Fatalf("Plant+Mine only = %d, want 1", n)
	}
}

// TestUrzaLandsSplitAcrossControllers: Mine+Tower under seat 0, Plant under
// seat 1 -- no single seat controls all three, so each taps for one. This is
// the leaf that catches a count reading the wrong battlefield.
func TestUrzaLandsSplitAcrossControllers(t *testing.T) {
	g, ids := urzaFixture(t, 0, 0, 1) // plant goes to seat 1
	if n := countUrza(g, 0, ids["mine"], "Count$UrzaLands.2.1"); n != 1 {
		t.Fatalf("seat 0 Mine = %d, want 1 (plant is seat 1's)", n)
	}
	if n := countUrza(g, 0, ids["tower"], "Count$UrzaLands.3.1"); n != 1 {
		t.Fatalf("seat 0 Tower = %d, want 1", n)
	}
	if n := countUrza(g, 1, ids["plant"], "Count$UrzaLands.2.1"); n != 1 {
		t.Fatalf("seat 1 plant = %d, want 1", n)
	}
}
