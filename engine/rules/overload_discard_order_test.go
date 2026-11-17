package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// mindRakeSrc is the Mind Rake script, inline so the caster can be seat 1's
// own deck card.
const mindRakeSrc = "Name:Mind Rake\nManaCost:2 B\nTypes:Sorcery\n" +
	"A:SP$ Discard | ValidTgts$ Player | NumCards$ 2 | Mode$ TgtChoose | SpellDescription$ Target player discards two cards.\n" +
	"K:Overload:1 B\nOracle:x\n"

// TestOverloadedDiscardAsksCasterFirst: an overloaded "each player discards"
// is asked in APNAP order from the spell's controller (CR 101.4: the caster of
// a sorcery is the active player), not in seat order from seat 0.
func TestOverloadedDiscardAsksCasterFirst(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 7313, nil, nil, []string{mindRakeSrc})
	driveToStep(t, e, e.G.Turn+1, 1, state.StepMain1)
	if e.G.Active != 1 {
		t.Fatalf("active = %d, want seat 1 to hold the turn", e.G.Active)
	}
	var id state.ObjID
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, c := range e.G.Zone(z, 1) {
			if o := e.G.Obj(c); o.Face() != nil && o.Face().Name == "Mind Rake" {
				id = c
			}
		}
	}
	if id == 0 {
		t.Fatal("Mind Rake not found in seat 1's hand or library")
	}
	if o := e.G.Obj(id); o.Zone != state.ZHand {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZHand})
		e.pending = nil
		e.priorityRound()
	}
	if z := e.G.Obj(id).Zone; z != state.ZHand || e.G.Obj(id).Controller != 1 {
		t.Fatalf("Mind Rake zone %s controller %d, want seat 1's hand", z, e.G.Obj(id).Controller)
	}
	addMana(t, e, 1, "BB")
	submitChoices(t, e, castModeOption(t, e, id, "overloaded"))
	d := passUntilNonPriority(t, e, 20)
	for _, seat := range []state.PlayerID{1, 0} {
		if d == nil || d.Kind != decision.KModes || d.ResumeKind != "discard" || d.Player != seat {
			t.Fatalf("pending = %+v, want seat %d's two-card discard pick", d, seat)
		}
		n := len(d.Options)
		submitChoices(t, e, d.Options[n-1].Index, d.Options[n-2].Index)
		d = e.Pending()
	}
	passUntilStackEmpty(t, e, 20)
	replayCheck(t, e, cfg)
}
