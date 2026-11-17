package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPumpForgetObjects pins Forge PumpEffect's ForgetObjects$ (Celestial
// Vault's, Spell Queller's DBForget): the named objects leave both halves of
// the source's remembered state -- the chain's Ctx.Remembered and the source
// object's persistent list -- so a later Defined$ Remembered no longer reads
// them. An object not named stays remembered.
func TestPumpForgetObjects(t *testing.T) {
	h := newHost(t, 2)
	onBattlefield := func(name string) state.ObjID {
		t.Helper()
		o := h.g.AddObject(mkCard(t, "Name:"+name+"\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0)
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
		return o.ID
	}
	src, a, b := onBattlefield("Vault"), onBattlefield("A"), onBattlefield("B")
	c := &Ctx{Source: src, Controller: 0}
	for _, id := range []state.ObjID{a, b} {
		c.Remembered = append(c.Remembered, state.Target{Obj: id})
		eventRemember(h, c, id)
	}
	c.Targets = []state.Target{{Obj: a}}
	sa := chainSA(t, "SP$ Pump | ForgetObjects$ Targeted")
	Resolve(h, c, sa)
	for _, got := range [][]state.Target{c.Remembered, h.g.Obj(src).Remembered} {
		if len(got) != 1 || got[0].Obj != b {
			t.Fatalf("remembered %v after ForgetObjects$ Targeted, want only %d", got, b)
		}
	}
}
