// copy_exceptions_test.go pins CR 707.9a/b for api:CopyPermanent: a copy
// effect's exceptions -- Eternalize's "black", "4/4" and "Zombie" -- are part
// of the token's copiable values, so a copy of the token (CR 707.3) has them
// too.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// eternalizeTimelessDragon puts the Timeless Dragon eternalize token onto
// seat 0's battlefield and returns its id.
func eternalizeTimelessDragon(t *testing.T, e *Engine) state.ObjID {
	t.Helper()
	dragon := findCardObj(t, e, 0, "Timeless Dragon", state.ZGraveyard)
	addMana(t, e, 0, "CCWW") // Eternalize {2}{W}{W}
	submitChoices(t, e, embalmAbilityOption(t, e, dragon, "Eternalize"))
	passUntilStackEmpty(t, e, 40)
	return embalmToken(t, e, "Timeless Dragon")
}

// assertEternalizedDragon fails unless id is a 4/4 Zombie Dragon with the
// copied Flying.
func assertEternalizedDragon(t *testing.T, e *Engine, id state.ObjID, what string) {
	t.Helper()
	d := e.Derived(id)
	if !hasWord(d.Types, "Zombie") || !hasWord(d.Types, "Dragon") {
		t.Fatalf("%s types %v, want a Zombie Dragon (CR 707.9b)", what, d.Types)
	}
	if d.Power != 4 || d.Toughness != 4 {
		t.Fatalf("%s P/T %d/%d, want 4/4 (CR 707.9b)", what, d.Power, d.Toughness)
	}
	if !e.HasKeyword(id, "Flying") {
		t.Fatalf("%s lost the copied Flying", what)
	}
}

// TestCloneOfAnEternalizedTokenCopiesTheExceptions: Clone entering as a copy of
// Timeless Dragon's eternalize token copies the token's copiable values --
// the printed Dragon as modified by the Eternalize exceptions -- so it is a
// 4/4 Zombie Dragon, never the printed 5/5 Dragon.
func TestCloneOfAnEternalizedTokenCopiesTheExceptions(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 1405, []string{"Timeless Dragon", "Clone"}, nil, nil)
	tok := eternalizeTimelessDragon(t, e)
	clone := findCardObj(t, e, 0, "Clone", state.ZHand)
	addMana(t, e, 0, "CCCU")
	castCopyCard(t, e, clone, tok, 0)
	if o := e.G.Obj(clone); o.Zone != state.ZBattlefield || o.Face().Name != "Timeless Dragon" {
		t.Fatalf("Clone is not a Timeless Dragon copy: zone %s name %q", o.Zone, o.Face().Name)
	}
	assertEternalizedDragon(t, e, clone, "Clone of the eternalized token")
	assertEternalizedDragon(t, e, tok, "the eternalized token")
	replayCheck(t, e, cfg)
}

// TestTokenCopyOfAnEternalizedTokenKeepsTheExceptions is the token route:
// Cackling Counterpart's copy of the eternalized token is a 4/4 Zombie
// Dragon too.
func TestTokenCopyOfAnEternalizedTokenKeepsTheExceptions(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 1409, []string{"Timeless Dragon", "Cackling Counterpart"}, nil, nil)
	first := eternalizeTimelessDragon(t, e)
	counterpart := findCardObj(t, e, 0, "Cackling Counterpart", state.ZHand)
	addMana(t, e, 0, "CUU")
	castCopyCard(t, e, counterpart, 0, first)
	var second state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); id != first && o.IsToken && o.Face() != nil && o.Face().Name == "Timeless Dragon" {
			second = id
		}
	}
	if second == 0 {
		t.Fatal("no token copy of the eternalized Timeless Dragon")
	}
	assertEternalizedDragon(t, e, second, "token copy of the eternalized token")
	replayCheck(t, e, cfg)
}
