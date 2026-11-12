package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 303.4f: two Auras returning from until-leaves exile in one departure
// batch each choose what they enchant as they enter. The choices are asked in
// entry order, and each answer is the Aura's own -- a later entry must not
// overwrite the earlier entry's record.
func TestTwoReturningAurasEachChooseTheirHost(t *testing.T) {
	t.Parallel()
	exiler := "Name:Until Host Leaves\nManaCost:2 W\nTypes:Enchantment\nOracle:x\n"
	pacifism := "Name:Pacifism\nManaCost:1 W\nTypes:Enchantment Aura\nK:Enchant creature\nOracle:x\n"
	e := handEngine(t)

	hostA := onBoard(t, e, 0, "Name:Bear A\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	hostB := onBoard(t, e, 0, "Name:Bear B\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	exA := onBoard(t, e, 0, exiler)
	exB := onBoard(t, e, 0, exiler)
	auraA := onBoard(t, e, 0, pacifism)
	auraB := onBoard(t, e, 0, pacifism)

	for _, aura := range []state.ObjID{auraA, auraB} {
		e.emit(events.Event{Kind: events.MoveZone, Obj: aura, From: state.ZBattlefield, To: state.ZExile})
	}
	e.G.Obj(exA).ExileReturn = []state.ExileReturnEntry{{Obj: auraA, From: state.ZBattlefield}}
	e.G.Obj(exB).ExileReturn = []state.ExileReturnEntry{{Obj: auraB, From: state.ZBattlefield}}

	// Both hosts leave in one zone batch: their held Auras return together
	// (CR 610.3d), and each is a separate CR 303.4f choice.
	e.openZoneBatch()
	e.emit(events.Event{Kind: events.MoveZone, Obj: exA, From: state.ZBattlefield, To: state.ZGraveyard})
	e.emit(events.Event{Kind: events.MoveZone, Obj: exB, From: state.ZBattlefield, To: state.ZGraveyard})
	e.closeZoneBatch()

	want := map[state.ObjID]state.ObjID{auraA: hostB, auraB: hostA}
	asked := make([]state.ObjID, 0, 2)
	for i := 0; i < 6; i++ {
		d := e.Pending()
		if d == nil {
			break
		}
		if d.Kind == decision.KPriority {
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{passIndex(t, d)}}); err != nil {
				t.Fatalf("pass priority: %v", err)
			}
			continue
		}
		if d.Kind != decision.KChoose || d.ResumeKind != "etb" {
			t.Fatalf("unexpected decision %s/%s waiting for a returning Aura", d.Kind, d.ResumeKind)
		}
		host, ok := want[d.Source]
		if !ok {
			t.Fatalf("asked for unexpected object %d", d.Source)
		}
		if len(d.Options) < 2 {
			t.Fatalf("Aura %d offered %d hosts, want its two legal creatures", d.Source, len(d.Options))
		}
		asked = append(asked, d.Source)
		idx := -1
		for _, opt := range d.Options {
			if opt.Obj == host {
				idx = opt.Index
			}
		}
		if idx < 0 {
			t.Fatalf("Aura %d not offered its chosen host %d: %+v", d.Source, host, d.Options)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
			t.Fatalf("submit enchant choice: %v", err)
		}
	}

	if len(asked) != 2 {
		t.Fatalf("each returning Aura must choose its own host: %d choice(s) asked (%v)", len(asked), asked)
	}
	if got := e.G.Obj(auraA); got == nil || got.Zone != state.ZBattlefield || got.AttachedTo != hostB {
		t.Fatalf("first Aura = %+v, want on the battlefield attached to its chosen host %d", got, hostB)
	}
	if got := e.G.Obj(auraB); got == nil || got.Zone != state.ZBattlefield || got.AttachedTo != hostA {
		t.Fatalf("second Aura = %+v, want on the battlefield attached to its chosen host %d", got, hostA)
	}
}
