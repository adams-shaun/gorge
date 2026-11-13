package kgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Activation-cost tokens: SubCounter<X/KIND> (The Astonishing Ant-Man), an
// Exert<1/NICKNAME> cost (Basri, Tomorrow's Champion) and the
// ExileFromStack<1/Card.Self> part of an UnlessCost$ (Sphinx's Approach).
// Each is named by the corpus card that carries it, as the neighbouring
// cost-shape tests do.
func TestActivationCostTokens(t *testing.T) {
	heavyParallel(t)
	checkForms(t, []formCase{
		{card: "The Astonishing Ant-Man", form: "SubCounter<X/P1P1> announces X up to the counters", lir: "CreateToken obj=You A=X CS=g_1_1_insect"},
		{card: "Basri, Tomorrow's Champion", form: "Exert<1/NICKNAME> as the activated cost", lir: "Activated  cost={0W} T"},
		{card: "Sphinx's Approach", form: "UnlessCost$ ExileFromGrave + ExileFromStack<1/Card.Self>", lir: "UnlessSwitched obj=You A=4 CS=ExileStackSelf"},
	})
}

// TestActivationCostTokenBuild pins the kernel cost shape each lowering
// produces: the announced-X counter flag with its kind, and the exert byte.
func TestActivationCostTokenBuild(t *testing.T) {
	cardsDir := ratchetCards(t)
	list := filepath.Join(t.TempDir(), "cards.txt")
	names := "The Astonishing Ant-Man\nBasri, Tomorrow's Champion\nSphinx's Approach\n"
	if err := os.WriteFile(list, []byte(names), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	cards, built, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]kop.CardDef{}
	for i, c := range cards {
		byName[c.Name] = built[i].def
	}
	ant, ok := byName["The Astonishing Ant-Man"]
	if !ok || len(ant.Abilities) == 0 {
		t.Fatal("The Astonishing Ant-Man not built")
	}
	if c := ant.Abilities[0].Cost; c.Flags&kop.CostLoyaltyX == 0 || c.Flags&kop.CostLoyalty != 0 || c.Ctr != kop.CtrP1P1 || c.N != 0 {
		t.Fatalf("The Astonishing Ant-Man cost = %+v, want CostLoyaltyX with CtrP1P1 and no CostLoyalty", c)
	}
	basri, ok := byName["Basri, Tomorrow's Champion"]
	if !ok || len(basri.Abilities) == 0 {
		t.Fatal("Basri, Tomorrow's Champion not built")
	}
	if c := basri.Abilities[0].Cost; !c.Exert || c.Flags&kop.CostTap == 0 {
		t.Fatalf("Basri cost = %+v, want Exert beside CostTap", c)
	}
	sphinx, ok := byName["Sphinx's Approach"]
	if !ok || len(sphinx.Abilities) == 0 || len(sphinx.Abilities[0].Prog) < 2 {
		t.Fatal("Sphinx's Approach not built")
	}
	pay := sphinx.Abilities[0].Prog[1]
	if pay.Op != kop.OpUnlessSwitched || pay.C&0xF != kop.PayExileGrave || pay.B.N != kop.PayBSubStackSelf || pay.A != kop.Const(4) {
		t.Fatalf("Sphinx's Approach unless pay = %+v, want UnlessSwitched/ExileGrave/stack-self A=4", pay)
	}
	if pay.F == 0 {
		t.Fatal("Sphinx's Approach unless pay carries no graveyard filter")
	}
}
