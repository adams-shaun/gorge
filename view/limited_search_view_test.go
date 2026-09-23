package view

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

func TestOmniscientViewIncludesReadOnlyPendingDecision(t *testing.T) {
	g := state.NewGame([]string{"alice", "bob"})
	d := &decision.Decision{Player: 0, Kind: decision.KChoose, Min: 1, Max: 1,
		Options: []decision.Option{{Index: 0, Kind: "search", Label: "Top Card", Obj: 7}}}

	v := ProjectFor(g, nil, NoSeat, Omniscient, d)
	if v.Decision == nil || len(v.Decision.Options) != 1 || v.Decision.Options[0].Obj != 7 {
		t.Fatalf("omniscient view lost pending decision: %+v", v.Decision)
	}
	if v.Decision == d {
		t.Fatal("omniscient view aliases the engine decision")
	}

	public := ProjectFor(g, nil, NoSeat, Public, d)
	if public.Decision != nil {
		t.Fatalf("public view exposed pending decision: %+v", public.Decision)
	}
}
