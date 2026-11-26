package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const sieglessSiege = "Name:Mini Siege\nManaCost:3 R\nTypes:Enchantment\n" +
	"K:ETBReplacement:Other:SiegeChoice\n" +
	"SVar:SiegeChoice:DB$ GenericChoice | Choices$ Khans,Dragons | Defined$ You | SetChosenMode$ True | AILogic$ Khans | ShowChoice$ ExceptSelf | SpellDescription$ As CARDNAME enters, choose Khans or Dragons.\n" +
	"SVar:Khans:DB$ Nothing | SpellDescription$ Khans\n" +
	"SVar:Dragons:DB$ Nothing | SpellDescription$ Dragons\n" +
	"Oracle:x\n"

// TestUntilLeavesReturnAsksAsEntersMode pins CR 614.12 and 610.3: a Siege an
// until-leaves exile returns from a state-based departure (no resolution run
// serves the entry) is entered only after its owner chooses Khans or Dragons;
// gorge once logged "ask answered with its default" and took the first mode.
// Forge corroboration: ChooseGenericEffect asks the Defined$ player through
// chooseSingleSpellAbility, never defaulting it.
func TestUntilLeavesReturnAsksAsEntersMode(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	host := onBoard(t, e, 0, "Name:Prison\nManaCost:2 W\nTypes:Enchantment\nOracle:x\n")
	siege := inExile(t, e, 1, sieglessSiege)
	e.G.Obj(host).ExileReturn = []state.ExileReturnEntry{{Obj: siege, From: state.ZBattlefield}}
	mark := len(e.L.Events)
	e.emit(events.Event{Kind: events.MoveZone, Obj: host, From: state.ZBattlefield, To: state.ZGraveyard})
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes || d.Player != 1 || len(d.Options) != 2 {
		t.Fatalf("pending after the host left = %+v, want the owner's two-mode ask", d)
	}
	if z := e.G.Obj(siege).Zone; z != state.ZExile {
		t.Fatalf("Siege in zone %v while its mode is asked, want exile (CR 614.12)", z)
	}
	submitChoices(t, e, 1) // Dragons
	if z := e.G.Obj(siege).Zone; z != state.ZBattlefield {
		t.Fatalf("Siege in zone %v after the answer, want the battlefield", z)
	}
	if got := e.G.Obj(siege).ChosenModes; len(got) != 1 || got[0] != "Dragons" {
		t.Fatalf("ChosenModes = %v, want [Dragons]", got)
	}
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Note && ev.Text != "" && len(ev.Text) > 3 && ev.Text[:3] == "ask" {
			t.Fatalf("a default-answered ask was logged: %q", ev.Text)
		}
	}
}

// TestUntilLeavesReturnAsksAsEntersModeMutation proves the test above reaches
// the ask: with the entry ask switched off the Siege enters on the default.
func TestUntilLeavesReturnAsksAsEntersModeMutation(t *testing.T) {
	DebugETBModeNoEntryAsk = true
	defer func() { DebugETBModeNoEntryAsk = false }()
	e := layerEngine(t)
	host := onBoard(t, e, 0, "Name:Prison\nManaCost:2 W\nTypes:Enchantment\nOracle:x\n")
	siege := inExile(t, e, 1, sieglessSiege)
	e.G.Obj(host).ExileReturn = []state.ExileReturnEntry{{Obj: siege, From: state.ZBattlefield}}
	e.emit(events.Event{Kind: events.MoveZone, Obj: host, From: state.ZBattlefield, To: state.ZGraveyard})
	if d := e.Pending(); d != nil && d.Kind == decision.KModes {
		t.Fatalf("the mode was asked with the entry ask switched off: %+v", d)
	}
	if z := e.G.Obj(siege).Zone; z != state.ZBattlefield {
		t.Fatalf("Siege in zone %v, want the battlefield on the default", z)
	}
}
