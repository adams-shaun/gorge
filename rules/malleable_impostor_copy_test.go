package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestMirrorImageCopy(t *testing.T) {
	e := handEngine(t, corpusAlternativeCard(t, "Mirror Image"))
	bear := e.G.AddObject(corpusAlternativeCard(t, "Colossal Dreadmaw"), 0)
	bear.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{bear.ID})
	id := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MC] = 2
	e.G.Players[0].Pool[state.MU] = 1
	castMode(t, e, id, "")
	if d := e.Pending(); d == nil || len(d.Options) < 2 {
		t.Fatalf("expected Mirror Image copy choice, got %+v", d)
	} else {
		submitChoices(t, e, d.Options[0].Index)
	}
	finishCast(t, e, id)
	if d := e.Derived(id); d.Power != 6 || d.Toughness != 6 {
		t.Fatalf("Mirror Image copy has %d/%d, want 6/6", d.Power, d.Toughness)
	}
}

func TestMalleableImpostorCopy(t *testing.T) {
	e := handEngine(t, corpusAlternativeCard(t, "Malleable Impostor"))
	bear := e.G.AddObject(corpusAlternativeCard(t, "Colossal Dreadmaw"), 1)
	bear.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, []state.ObjID{bear.ID})
	id := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MC] = 3
	e.G.Players[0].Pool[state.MU] = 1
	castMode(t, e, id, "")
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected ETB copy choice, got %+v", d)
	}
	var target int
	for _, o := range d.Options {
		if o.Obj == bear.ID {
			target = o.Index
		}
	}
	if target == 0 && (len(d.Options) == 0 || d.Options[0].Obj != bear.ID) {
		t.Fatalf("copy target not offered: %+v", d.Options)
	}
	submitChoices(t, e, target)
	finishCast(t, e, id)
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("impostor did not enter battlefield: %+v", o)
	}
	if d := e.Derived(id); d.Power != 6 || d.Toughness != 6 {
		t.Fatalf("copy has %d/%d, want 6/6", d.Power, d.Toughness)
	}
	if !hasEvent(e, events.ClonePermanent, id) {
		t.Fatal("copy did not emit ClonePermanent")
	}
}
