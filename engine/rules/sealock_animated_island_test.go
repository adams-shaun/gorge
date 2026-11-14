package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// Sealock Monster's monstrosity trigger makes a land an Island in addition to
// its other types for good (CR 305.7, 611.2a). A Plains so animated is an Island
// the defending player controls, so UnlessDefender$ controlsIsland releases the
// Monster.
func TestSealockMonsterMayAttackPlayerWhoseLandBecameAnIsland(t *testing.T) {
	t.Parallel()
	e := attackBlockedRegressionEngine(t)
	monster := onBoardCard(t, e, 0, corpusCard(t, "Sealock Monster"))
	const src = "Name:Test Plains\nTypes:Basic Land Plains\nPT:0/0\n" +
		"SVar:Turn:DB$ Animate | Types$ Island | Duration$ Permanent\nOracle:x\n"
	plains := onBoard(t, e, 1, src)
	if !e.attackBlocked(monster, 1, 0) {
		t.Fatal("precondition: Sealock Monster attacked a defender with no Island")
	}
	body := cards.ResolveSVar(e.G.Obj(plains).Card.Faces[0].SVars, "Turn")
	if body == nil || body.API != "Animate" {
		t.Fatalf("precondition: fixture Animate body missing: %+v", body)
	}
	effects.Resolve(e, &effects.Ctx{Source: plains, Controller: 1}, body)
	if got := e.Derived(plains).Types; !slices.Contains(got, "Island") || !slices.Contains(got, "Plains") {
		t.Fatalf("animated land types = %v; want Island in addition to Plains", got)
	}
	if o := e.G.Obj(plains); o.Zone != state.ZBattlefield || o.Controller != 1 {
		t.Fatalf("precondition: the animated land is not seat 1's battlefield land: %+v", o)
	}
	if e.attackBlocked(monster, 1, 0) {
		t.Fatal("Sealock Monster stayed blocked although the defender controls a land that became an Island")
	}
}
