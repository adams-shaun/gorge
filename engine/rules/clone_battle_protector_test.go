package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// CR 310.10 / 707.2: a permanent that enters as a copy of a Battle is a
// Battle as it enters, so its controller chooses an opponent to protect it.
// In a two-seat game the only opponent is the original's controller.
func TestCopyBattleGetsProtector(t *testing.T) {
	t.Parallel()
	run := runETBCloneScenario(t, `{"name":"clone-battle-protector","cr":["310.10","707.2"],"why":"a copy of a Battle records a protector","setup":{`+
		`"p0":{"hand":["Invasion of Segovia"]},"p1":{"hand":["Clever Impersonator"],"library":["Forest","Forest"]}},`+
		`"steps":[{"op":"cast","seat":0,"card":"p0:Invasion of Segovia","mana":"CCUU"},{"op":"resolve","seat":0},`+
		`{"op":"pass_to","seat":0,"step":"upkeep","active":"p1"},{"op":"pass_to","seat":1,"step":"main1","active":"p1"},`+
		`{"op":"cast","seat":1,"card":"p1:Clever Impersonator","mana":"CCUU"},`+
		`{"op":"resolve","seat":1,"answers":[{"kind":"choose","pick":["p0:Invasion of Segovia"]}]}],"expect":[]}`)
	e := run.e
	orig := e.G.Obj(findOn(t, e, state.ZBattlefield, 0, "Invasion of Segovia"))
	cp := e.G.Obj(findOn(t, e, state.ZBattlefield, 1, "Invasion of Segovia"))
	if !orig.ProtectorValid || orig.Protector != 1 {
		t.Fatalf("precondition: original protector valid=%v seat=%d, want true/1", orig.ProtectorValid, orig.Protector)
	}
	if !cp.ProtectorValid || cp.Protector != 0 {
		t.Fatalf("copy protector valid=%v seat=%d, want true/0", cp.ProtectorValid, cp.Protector)
	}
	if cp.Counter("DEFENSE") != 4 {
		t.Fatalf("precondition: copy defense %d, want 4", cp.Counter("DEFENSE"))
	}
	replayCheck(t, e, run.cfg)
}
