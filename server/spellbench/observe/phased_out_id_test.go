package observe_test

import (
	"encoding/json"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/identity"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testgame"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/validate"
)

func battlefieldMountain(obs protocol.Observation) *protocol.ObjectRecord {
	for i := range obs.Players[0].Battlefield {
		rec := &obs.Players[0].Battlefield[i]
		if rec.CardName != nil && *rec.CardName == "Mountain" {
			return rec
		}
	}
	return nil
}

func TestPhasedOutPermanentReturnsWithAFreshID(t *testing.T) {
	reg := testcorpus.Registry(t)
	g := testgame.New(t, reg, "Burn", "Burn", 1, "none")
	tr := identity.New(g.R, g.Secret)
	p := &observe.Projector{R: g.R, IDs: tr}
	var mtn state.ObjID
	for _, id := range g.E.G.Zone(state.ZHand, 0) {
		if o := g.E.G.Obj(id); o.Face() != nil && o.Face().Name == "Mountain" {
			mtn = id
			break
		}
	}
	if mtn == 0 {
		t.Fatal("precondition: p0 has no Mountain in hand")
	}
	events.Emit(g.E.G, g.E.L, events.Event{Kind: events.MoveZone, Obj: mtn, Player: 0,
		From: state.ZHand, To: state.ZBattlefield})
	if err := tr.Sync(g.R); err != nil {
		t.Fatal(err)
	}
	obs1, err := p.Observation(0, observe.State{})
	if err != nil {
		t.Fatal(err)
	}
	rec1 := battlefieldMountain(obs1)
	if rec1 == nil {
		t.Fatal("precondition: the Mountain is not in the first observation")
	}
	events.Emit(g.E.G, g.E.L, events.Event{Kind: events.PhaseOut, Obj: mtn, Amount: 1})
	if err := tr.Sync(g.R); err != nil {
		t.Fatal(err)
	}
	obs2, err := p.Observation(0, observe.State{})
	if err != nil {
		t.Fatal(err)
	}
	if battlefieldMountain(obs2) != nil {
		t.Fatal("precondition: the phased-out permanent is still listed")
	}
	events.Emit(g.E.G, g.E.L, events.Event{Kind: events.PhaseOut, Obj: mtn, Amount: -1})
	if err := tr.Sync(g.R); err != nil {
		t.Fatal(err)
	}
	obs3, err := p.Observation(0, observe.State{})
	if err != nil {
		t.Fatal(err)
	}
	rec3 := battlefieldMountain(obs3)
	if rec3 == nil {
		t.Fatal("the phased-in permanent is missing from the third observation")
	}
	if rec3.ObjectID == rec1.ObjectID {
		t.Fatalf("id %s returned after leaving the viewer's observation", rec1.ObjectID)
	}
	profile := validate.Profile{Kinds: map[string]bool{}, Flags: observe.Flags,
		Extensions: map[string]bool{}}
	for k := range protocol.KindFields {
		profile.Kinds[k] = true
	}
	s := validate.NewStream(profile)
	for i, obs := range []protocol.Observation{obs1, obs2, obs3} {
		sd := protocol.SeatDecision{ActingSeat: "p0", SeatStep: uint64(i),
			Group:       protocol.Group{GroupID: uint64(i), SubstepCount: 1},
			Context:     protocol.Context{Kind: "priority"},
			Observation: obs,
			Candidates:  []protocol.Candidate{{Semantic: protocol.Pass()}},
			Extensions:  map[string]json.RawMessage{}}
		if err := s.Check(sd); err != nil {
			t.Fatalf("decision %d: %v", i, err)
		}
	}
}
