package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// A Play ValidSA$ mana-value bound names its SVar by one capital letter
// (cmcLEY, Captain Benjamin Sisko's Officer count), not only X. The ask's
// options are read once, when the Play poses its choice; Officers arriving
// afterwards do not move the offered set.
func TestPlayCmcLEYReadsTheYSVarOnce(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	var exiled []state.ObjID
	for _, src := range []string{
		"Name:Mv1\nManaCost:R\nTypes:Instant\nOracle:x\n",
		"Name:Mv2\nManaCost:1 R\nTypes:Instant\nOracle:x\n",
		"Name:Mv3\nManaCost:2 R\nTypes:Instant\nOracle:x\n",
		"Name:Land\nTypes:Land\nOracle:x\n",
	} {
		o := h.g.AddObject(mkCard(t, src), 0)
		o.Zone = state.ZExile
		exiled = append(exiled, o.ID)
	}
	h.g.SetZone(state.ZExile, 0, exiled)
	var field []state.ObjID
	for range 2 {
		o := h.g.AddObject(mkCard(t, "Name:Off\nTypes:Creature Human Officer\nPT:1/1\nOracle:x\n"), 0)
		o.Zone = state.ZBattlefield
		o.Controller = 0
		field = append(field, o.ID)
	}
	h.g.SetZone(state.ZBattlefield, 0, field)
	src := h.g.AddObject(mkCard(t, "Name:Play Source\nTypes:Sorcery\nOracle:x\n"), 0)
	src.Zone = state.ZBattlefield

	line := "SP$ Play | Valid$ Card.YouOwn | ValidZone$ Exile | ValidSA$ Spell.cmcLEY | WithoutManaCost$ True | Amount$ All | Controller$ You | Optional$ True"
	// X deliberately disagrees with Y (X reads 0): the bound must follow Y.
	svars := map[string]string{"Y": "Count$Valid Officer.YouCtrl", "X": "Count$Valid Land.YouCtrl"}
	Resolve(h, &Ctx{Source: src.ID, Controller: 0, SVars: svars}, sa(t, line))
	if h.asked == nil || h.asked.ResumeKind != "play" {
		t.Fatalf("Play did not pose its choice: %+v", h.asked)
	}
	got := map[state.ObjID]bool{}
	for _, o := range h.asked.Options {
		got[o.Obj] = true
	}
	// two Officers: Mv1 and Mv2 offered, Mv3 above the bound, the land never
	if len(got) != 2 || !got[exiled[0]] || !got[exiled[1]] {
		t.Fatalf("with 2 Officers the offer = %v, want only Mv1 %d and Mv2 %d", got, exiled[0], exiled[1])
	}
	// a third Officer after the ask moves nothing already posed ...
	o := h.g.AddObject(mkCard(t, "Name:Off\nTypes:Creature Human Officer\nPT:1/1\nOracle:x\n"), 0)
	o.Zone = state.ZBattlefield
	o.Controller = 0
	h.g.SetZone(state.ZBattlefield, 0, append(field, o.ID))
	if len(h.asked.Options) != 2 {
		t.Fatalf("the posed offer changed to %d options after an Officer arrived", len(h.asked.Options))
	}
	// ... and a fresh Play reads the new count: Mv3 now joins
	h.asked = nil
	Resolve(h, &Ctx{Source: src.ID, Controller: 0, SVars: svars}, sa(t, line))
	if h.asked == nil || len(h.asked.Options) != 3 {
		t.Fatalf("with 3 Officers the offer = %+v, want 3 options", h.asked)
	}
}
