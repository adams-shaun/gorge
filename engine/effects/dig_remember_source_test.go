package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// digRememberFixture resolves a two-land Dig (ChangeNum$ 2, DestinationZone$
// Battlefield) sourced at a spell object, with extra appended to the line.
func digRememberFixture(t *testing.T, extra string) (*fakeHost, *Ctx, []state.ObjID) {
	t.Helper()
	h := newHost(t, 2)
	spell := mkCard(t, "Name:Sphere\nTypes:Sorcery\nOracle:x\n")
	land := mkCard(t, "Name:Isle\nTypes:Basic Land Island\nOracle:x\n")
	src := h.g.AddObject(spell, 0)
	ids := []state.ObjID{h.g.AddObject(land, 0).ID, h.g.AddObject(land, 0).ID}
	h.g.SetZone(state.ZLibrary, 0, ids)
	c := &Ctx{Controller: 0, Source: src.ID}
	Resolve(h, c, sa(t, "SP$ Dig | Defined$ You | DigNum$ 2 | ChangeNum$ 2 | ChangeValid$ Land | DestinationZone$ Battlefield"+extra))
	if bf := h.g.Zone(state.ZBattlefield, 0); len(bf) != 2 {
		t.Fatalf("battlefield = %v, want both lands moved", bf)
	}
	return h, c, ids
}

// TestDigRememberChangedWritesBothHalves: RememberChanged$ True records each
// moved card on the Ctx AND on the source object, so Count$RememberedSize
// (Expand the Sphere's Number$2/Minus.Y) reads the number the Dig moved.
func TestDigRememberChangedWritesBothHalves(t *testing.T) {
	h, c, _ := digRememberFixture(t, " | RememberChanged$ True")
	if len(c.Remembered) != 2 {
		t.Fatalf("ctx Remembered = %+v, want 2", c.Remembered)
	}
	if n := len(h.g.Obj(c.Source).Remembered); n != 2 {
		t.Fatalf("source Remembered has %d entries, want 2", n)
	}
	if n := EvalCount(h, c, "Count$RememberedSize"); n != 2 {
		t.Fatalf("Count$RememberedSize = %d, want 2", n)
	}
}

// TestDigRememberChangedAbsentLeavesSourceEmpty: the parameter gate holds.
func TestDigRememberChangedAbsentLeavesSourceEmpty(t *testing.T) {
	h, c, _ := digRememberFixture(t, "")
	if len(c.Remembered) != 0 || len(h.g.Obj(c.Source).Remembered) != 0 {
		t.Fatalf("ctx %+v source %v, want both empty", c.Remembered, h.g.Obj(c.Source).Remembered)
	}
}
