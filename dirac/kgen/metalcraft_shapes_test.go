package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDemoMetalcraftShapes: the three demo-deck cards lower with no refusal,
// and each carries the shape the lowering adds (CR 207.2c):
//
//   - Brotherhood Scribe: a Metalcraft-gated non-mana activated ability and
//     the player-counter CounterAddedOnce line (inert in both engines: gorge's
//     counterAddedMatches reads CounterChange only, so kgen keeps the ability
//     with a constant-false condition);
//   - Urza's Workshop: a plain {T}: Add {C} beside a Metalcraft-gated
//     evaluated {T}: Add {C};
//   - Vedalken Humiliator: an Attacks trigger whose intervening-if is the
//     Metalcraft count.
func TestDemoMetalcraftShapes(t *testing.T) {
	names := []string{"Brotherhood Scribe", "Urza's Workshop", "Vedalken Humiliator"}
	lw := lowerNamedPool(t, names)
	if len(lw.cards) != len(names) {
		t.Fatalf("lowered %d cards, want %d", len(lw.cards), len(names))
	}
	for _, c := range lw.cards {
		for _, r := range c.Refusals {
			t.Errorf("%s refused: %s", c.Name, r.Reason)
		}
	}
	all, bs, pt, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]kop.CardDef{}
	for i, c := range all {
		byName[c.Name] = bs[i].def
		for _, r := range bs[i].refusals {
			t.Errorf("%s build refusal: %v", c.Name, r)
		}
	}
	metalcraft := func(id kop.CondID) bool { return metalcraftCond(pt, id) }

	scribe, ok := byName["Brotherhood Scribe"]
	if !ok {
		t.Fatal("Brotherhood Scribe: not built")
	}
	act, ctr := false, false
	for i := range scribe.Abilities {
		a := &scribe.Abilities[i]
		if a.Kind == kop.AbActivated && metalcraft(a.Trig.Cond) {
			act = true
		}
		if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigCounterAddedOnce && neverCond(pt, a.Trig.Cond) {
			ctr = true
		}
	}
	if !act {
		t.Error("Brotherhood Scribe: no Metalcraft-gated activated ability")
	}
	if !ctr {
		t.Error("Brotherhood Scribe: no player-counter CounterAddedOnce line")
	}

	workshop, ok := byName["Urza's Workshop"]
	if !ok {
		t.Fatal("Urza's Workshop: not built")
	}
	plain, gated := false, false
	for i := range workshop.Abilities {
		a := &workshop.Abilities[i]
		if a.Kind != kop.AbMana {
			continue
		}
		if a.Trig.Cond == 0 && a.AFlags&kop.AfManaDynamic == 0 {
			plain = true
		}
		if metalcraft(a.Trig.Cond) && a.AFlags&kop.AfManaDynamic != 0 {
			gated = true
		}
	}
	if !plain || !gated {
		t.Errorf("Urza's Workshop: plain=%v gated evaluated=%v, want both", plain, gated)
	}

	hum, ok := byName["Vedalken Humiliator"]
	if !ok {
		t.Fatal("Vedalken Humiliator: not built")
	}
	atk := false
	for i := range hum.Abilities {
		if a := &hum.Abilities[i]; a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigAttacks && metalcraft(a.Trig.Cond) {
			atk = true
		}
	}
	if !atk {
		t.Error("Vedalken Humiliator: no Attacks trigger with a Metalcraft condition")
	}
}

// metalcraftCond reports whether id is kgen's metalcraft condition:
// Count$Valid <artifacts you control> >= 3.
func metalcraftCond(pt *poolTables, id kop.CondID) bool {
	if int(id) >= len(pt.conds) {
		return false
	}
	c := &pt.conds[id]
	return c.Kind == kop.CondValue && !c.Neg && c.Op == kop.CmpGE && c.B.K == kop.VConst && c.B.N == 3 &&
		c.A.K == kop.VCountFilter && c.A.N != 0
}

// neverCond reports whether id is the constant-false condition the lowering
// gives a player-counter line: 0 >= 1.
func neverCond(pt *poolTables, id kop.CondID) bool {
	if int(id) >= len(pt.conds) {
		return false
	}
	c := &pt.conds[id]
	return c.Kind == kop.CondValue && !c.Neg && c.Op == kop.CmpGE && c.A.K == kop.VConst && c.A.N == 0 &&
		c.B.K == kop.VConst && c.B.N == 1
}
