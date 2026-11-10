package rules

// Tam, Mindful First-Year pinned on the real corpus card: "each other creature
// you control has hexproof from each of its colors" (CR 702.11d) and its {T}
// "becomes all colors until end of turn" ability. The generic expansion is
// pinned in hexproof_owncolours_test.go on a fixture; these rows cover the
// corpus text, Tam's own activation, the cast-offer census and a colourless
// source.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestHexproofFromEachOfItsColoursTam(t *testing.T) {
	t.Parallel()
	const (
		redBear   = "Name:Red Bear\nManaCost:1 R\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
		golem     = "Name:Golem\nManaCost:3\nTypes:Artifact Creature Golem\nPT:2/2\nOracle:x\n"
		colorless = "Name:Colorless Source\nManaCost:3\nTypes:Artifact Creature Golem\nPT:1/1\nOracle:x\n"
		blueSrc   = "Name:Blue Source\nManaCost:U\nTypes:Creature Elf\nPT:1/1\nOracle:x\n"
		redSrc    = "Name:Red Source\nManaCost:R\nTypes:Creature Elf\nPT:1/1\nOracle:x\n"
	)
	// setup puts the corpus Tam and the given bearer on seat 0's battlefield.
	setup := func(t *testing.T, bearer string) (*Engine, state.ObjID, state.ObjID) {
		e := handEngine(t)
		tam := onBoardCard(t, e, 0, corpusCard(t, "Tam, Mindful First-Year"))
		e.G.Obj(tam).SummonSick = false
		return e, tam, onBoard(t, e, 0, bearer)
	}
	// becomeAllColours runs Tam's real {T} ability on the bearer to resolution.
	becomeAllColours := func(t *testing.T, e *Engine, tam, bear state.ObjID) {
		d := activateObjAbility(t, e, 0, tam)
		pick := -1
		for _, o := range d.Options {
			if o.Obj == bear {
				pick = o.Index
			}
		}
		if pick < 0 {
			t.Fatalf("bear not offered as Tam's ability target: %+v", d.Options)
		}
		submitChoices(t, e, pick)
		for i := 0; i < 20 && len(e.G.Stack) > 0; i++ {
			passFirst(t, e)
		}
		if len(e.G.Stack) != 0 {
			t.Fatalf("Tam's ability never resolved: %v", e.G.Stack)
		}
	}

	t.Run("own colour source is blocked", func(t *testing.T) {
		e, _, bear := setup(t, redBear)
		src := onBoard(t, e, 1, redSrc)
		if !e.HasKeyword(bear, "Hexproof") {
			t.Fatalf("precondition: bear lacks hexproof: %v", e.Keywords(bear))
		}
		if !e.hexproofBlocksTarget(bear, 1, src) {
			t.Fatalf("red source not blocked from red bear: %v", e.Keywords(bear))
		}
	})
	t.Run("Shock is not offered the red bear", func(t *testing.T) {
		e, tam, bear := setup(t, redBear)
		sh := addShockToHand(t, e, 1)
		addMana(t, e, 1, "R")
		e.askPriority(1)
		if !castOffered(e, sh) {
			t.Fatal("precondition: Shock not offered although Tam is a legal target")
		}
		castFirst(t, e, "cast")
		d := e.Pending()
		if d == nil || d.Kind != decision.KTarget {
			t.Fatalf("expected a target decision, got %+v", d)
		}
		tamOffered := false
		for _, o := range d.Options {
			if o.Obj == bear {
				t.Fatalf("hexproof-from-red bear offered to Shock: %+v", d.Options)
			}
			tamOffered = tamOffered || o.Obj == tam
		}
		if !tamOffered {
			t.Fatalf("precondition: Tam (no hexproof) not offered: %+v", d.Options)
		}
	})
	t.Run("other colour source is offered", func(t *testing.T) {
		e, _, bear := setup(t, redBear)
		src := onBoard(t, e, 1, blueSrc)
		if e.objColors(e.G.Obj(bear)) == e.objColors(e.G.Obj(src)) {
			t.Fatal("precondition: bear and source share colours")
		}
		if e.hexproofBlocksTarget(bear, 1, src) {
			t.Fatalf("blue source blocked from red bear: %v", e.Keywords(bear))
		}
	})
	t.Run("Tam's tap ability gives hexproof from every colour", func(t *testing.T) {
		e, tam, bear := setup(t, redBear)
		src := onBoard(t, e, 1, blueSrc)
		if e.hexproofBlocksTarget(bear, 1, src) {
			t.Fatal("precondition: blue source already blocked")
		}
		becomeAllColours(t, e, tam, bear)
		if got := e.objColors(e.G.Obj(bear)); len(got) != 5 {
			t.Fatalf("precondition: bear colours = %q, want all five", got)
		}
		if !e.hexproofBlocksTarget(bear, 1, src) {
			t.Fatalf("all-colour bear not hexproof from blue: %v", e.Keywords(bear))
		}
		if e.hexproofBlocksTarget(bear, 0, src) {
			t.Fatal("hexproof withheld the bear from its controller")
		}
	})
	t.Run("Tam itself gains nothing", func(t *testing.T) {
		e, tam, _ := setup(t, redBear)
		src := onBoard(t, e, 1, redSrc)
		if e.hexproofBlocksTarget(tam, 1, src) {
			t.Fatalf("Tam is hexproof: %v", e.Keywords(tam))
		}
	})
	t.Run("controller's own Shock is offered the bear", func(t *testing.T) {
		e, _, bear := setup(t, redBear)
		sh := addShockToHand(t, e, 0)
		addMana(t, e, 0, "R")
		e.askPriority(0)
		if !castOffered(e, sh) {
			t.Fatal("own Shock not offered")
		}
		castFirst(t, e, "cast")
		d := e.Pending()
		if d == nil || d.Kind != decision.KTarget {
			t.Fatalf("expected a target decision, got %+v", d)
		}
		found := false
		for _, o := range d.Options {
			found = found || o.Obj == bear
		}
		if !found {
			t.Fatalf("controller not offered its own bear: %+v", d.Options)
		}
	})
	t.Run("colourless source never blocks", func(t *testing.T) {
		e, tam, bear := setup(t, redBear)
		src := onBoard(t, e, 1, colorless)
		if c := e.objColors(e.G.Obj(src)); c != "" {
			t.Fatalf("precondition: source colours = %q", c)
		}
		if e.hexproofBlocksTarget(bear, 1, src) || e.hexproofBlocksTarget(bear, 1, 0) {
			t.Fatal("colourless source blocked by hexproof from colours")
		}
		becomeAllColours(t, e, tam, bear)
		if e.hexproofBlocksTarget(bear, 1, src) {
			t.Fatal("colourless source blocked by all-colour bear")
		}
	})
	t.Run("colourless bearer gains nothing", func(t *testing.T) {
		e, _, g := setup(t, golem)
		src := onBoard(t, e, 1, redSrc)
		if strings.Contains(strings.Join(e.Keywords(g), ","), "Hexproof") {
			t.Fatalf("colourless bearer has hexproof: %v", e.Keywords(g))
		}
		if e.hexproofBlocksTarget(g, 1, src) {
			t.Fatal("colourless bearer blocked")
		}
	})
}
