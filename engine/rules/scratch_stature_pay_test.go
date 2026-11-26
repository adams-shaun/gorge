package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestScratchStaturePayOrder replicates the lockstep seed-3 board: Stature
// cast this turn (sick), an Island dropped this turn whose object id is the
// SMALLEST of the untapped sources but which is LAST in battlefield zone
// order, and older untapped Islands with larger ids. It prints the order the
// payment taps sources. Scratch debug only; not for commit.
func TestScratchStaturePayOrder(t *testing.T) {
	for _, tc := range []struct{ name, zone string }{
		{"r1exact", "s,l"},
		{"fourlands", "s,l,l,l,l"},
		{"newland-last", "l,s,l,l,l,n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payOrder(t, tc.zone)
		})
	}
}

func payOrder(t *testing.T, zoneSpec string) {
	t.Helper()
	stature := corpusAlternativeCard(t, "Stature, Size Shifter")
	e := handEngine(t)
	var zone []state.ObjID
	stID := state.ObjID(0)
	addLand := func(sick bool) state.ObjID {
		m := card(t, "Name:Island\nTypes:Basic Land Island\nOracle:x\n")
		ob := e.G.AddObject(m, 0)
		ob.Zone = state.ZBattlefield
		ob.EnteredThisTurn = sick
		return ob.ID
	}
	for _, c := range zoneSpec {
		switch c {
		case 's':
			o := e.G.AddObject(stature, 0)
			o.Zone = state.ZBattlefield
			o.EnteredThisTurn = true
			stID = o.ID
			zone = append(zone, stID)
		case 'l':
			zone = append(zone, addLand(false))
		case 'n':
			zone = append(zone, addLand(true))
		}
	}
	e.G.SetZone(state.ZBattlefield, 0, zone)
	pool := &e.G.Players[0].Pool
	pool[state.MU] += int32(1)
	e.priorityRound()
	d := e.Pending()
	if d == nil {
		t.Fatal("no pending decision")
	}
	optIdx := -1
	for _, opt := range d.Options {
		if opt.Kind == "ability" && opt.Obj == stID {
			optIdx = opt.Index
		}
	}
	if optIdx < 0 {
		t.Fatalf("Stature activation not offered: %+v", d.Options)
	}
	submitChoices(t, e, optIdx)
	// the X ask: choose X=0 (the least)
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("want X ask, got %+v", d)
	}
	submitChoices(t, e, d.Options[0].Index)
	// run to the next priority, recording Tap events in order
	for i := 0; i < 200; i++ {
		if e.Pending() != nil && e.Pending().Kind == decision.KPriority {
			break
		}
		e.Advance()
	}
	for j, ev := range e.L.Events {
		if ev.Kind == events.Tap {
			name := ""
			if ob := e.G.Obj(ev.Obj); ob != nil && ob.Face() != nil {
				name = ob.Face().Name
			}
			t.Logf("ev %d TAP obj=%d %s", j, int(ev.Obj), name)
		}
	}
	for _, id := range zone {
		ob := e.G.Obj(id)
		t.Logf("obj %d zone=%d tapped=%v sick=%v", int(id), int(ob.Zone), ob.Tapped, ob.SummonSick)
	}
}
