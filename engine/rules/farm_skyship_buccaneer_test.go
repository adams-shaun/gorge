package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFarmSkyshipBuccaneer pins the FDN-s491 window xe:RAND:0:12:w18 (farm
// class 3way:FX|G, field `permanents`). The first differing value is the
// controller of High Fae Trickster at the setup checkpoint: gorge reported
// `c0 o0 High Fae Trickster`, Forge and XMage reported `c1 o0`, on a board
// where BOTH players' Confiscates ("You control enchanted permanent") are
// attached to it. CR 613.1b applies control-changing effects in timestamp
// order, so the Confiscate that became attached last takes the creature: the
// scenario lists p0's Aura first and p1's second, so p1's is later and p1
// controls the Trickster. Skyship Buccaneer (in p0's hand) is only the farm's
// attribution; the diverging object is the Trickster.
func TestFarmSkyshipBuccaneer(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	scenario := `{
	  "name": "farm-skyship-buccaneer-confiscate-order",
	  "turn": 1,
	  "cr": ["613.1b", "613.7e"],
	  "why": "FDN-s491 xe:RAND:0:12:w18 setup: two Confiscates on one creature",
	  "setup": {
	    "p0": {
	      "battlefield": ["High Fae Trickster", "Confiscate"],
	      "attached": [{"card": "Confiscate", "to": "p0:High Fae Trickster"}],
	      "life": 14
	    },
	    "p1": {
	      "battlefield": ["Confiscate"],
	      "attached": [{"card": "Confiscate", "to": "p0:High Fae Trickster"}],
	      "life": 20
	    }
	  },
	  "steps": [{"op": "pass", "seat": 0}]
	}`
	res, err := RunOracleScenarioJSON(reg, []byte(scenario))
	if err != nil {
		t.Fatalf("scenario JSON: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("%v", res.Fails)
	}
	var setup *OracleSnapshot
	for i := range res.Snapshots {
		if res.Snapshots[i].Checkpoint == "setup" {
			setup = &res.Snapshots[i]
		}
	}
	if setup == nil {
		t.Fatal("no setup snapshot")
	}
	var trickster *OracleSnapPerm
	attachedBy := map[int]bool{}
	for i := range setup.Permanents {
		p := &setup.Permanents[i]
		switch p.Name {
		case "High Fae Trickster":
			trickster = p
		case "Confiscate":
			if p.AttachedTo == "p0:High Fae Trickster" {
				// Precondition: the two Auras are on DIFFERENT seats, so
				// their CR 613.1b timestamp order decides the controller.
				attachedBy[p.Controller] = true
			}
		}
	}
	if trickster == nil || trickster.Owner != 0 {
		t.Fatalf("precondition: trickster = %+v, want p0's High Fae Trickster", trickster)
	}
	if !attachedBy[0] || !attachedBy[1] {
		t.Fatalf("precondition: Confiscates attached to the Trickster by controllers %v, want both 0 and 1", attachedBy)
	}
	// p1's Confiscate is placed (and so timestamped) after p0's: the latest
	// control-changing effect wins (CR 613.1b), matching Forge and XMage.
	if trickster.Controller != 1 {
		t.Fatalf("High Fae Trickster controller = %d, want 1 (the later Confiscate, CR 613.1b)", trickster.Controller)
	}
}

// TestFarmSkyshipBuccaneerSameControllerAuras pins the sibling of the farm
// case: two Confiscates from the SAME seat on one creature. The second is the
// later timestamp, so it must be the tracked grant even though the creature's
// controller already matches it; otherwise destroying the first Aura would
// hand the creature back to its owner for an instant (a visible
// ControlChange) before the second takes it again.
func TestFarmSkyshipBuccaneerSameControllerAuras(t *testing.T) {
	t.Parallel()
	conf, ok := testutil.CorpusRegistry(t).Lookup("Confiscate")
	if !ok {
		t.Fatal("corpus missing Confiscate")
	}
	e, cfg, bear := staticGainControlGame(t, 49118, []*cards.Card{conf, conf}, 1)
	// Confiscate costs {4}{U}{U}; castControlAura's Mind Control budget is
	// one short, so this test pays the sixth pip itself.
	castConfiscate := func() state.ObjID {
		t.Helper()
		id := findAndMoveToHand(t, e, 0, "Confiscate")
		addMana(t, e, 0, "UUUUUU")
		castFromPriority(t, e, id)
		answerKTarget(t, e, bear)
		passUntilStackEmpty(t, e, 60)
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || o.AttachedTo != bear {
			t.Fatalf("precondition: Confiscate %d = %+v, want it attached to bear %d", id, o, bear)
		}
		return id
	}
	first := castConfiscate()
	second := castConfiscate()
	if first == second {
		t.Fatalf("both casts used obj %d, want two Auras", first)
	}
	// Precondition: two Auras on the SAME bearer, both controlled by seat 0,
	// so only their timestamp order can matter.
	for _, id := range []state.ObjID{first, second} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || o.AttachedTo != bear || o.Controller != 0 {
			t.Fatalf("precondition: Confiscate %d = %+v, want seat 0's Aura attached to bear %d", id, o, bear)
		}
	}
	if got := e.G.Obj(bear).Controller; got != 0 {
		t.Fatalf("bearer controller after both Confiscates = %d, want 0", got)
	}
	if ch := controlChangesFor(e, bear); len(ch) != 1 || ch[0].Player != 0 {
		t.Fatalf("ControlChange events after both Confiscates = %+v, want exactly one to seat 0", ch)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: first, From: state.ZBattlefield, To: state.ZGraveyard})
	if got := e.G.Obj(bear).Controller; got != 0 {
		t.Fatalf("bearer controller after the earlier Aura left = %d, want 0 (the later Aura still controls it)", got)
	}
	if ch := controlChangesFor(e, bear); len(ch) != 1 {
		t.Fatalf("ControlChange events after the earlier Aura left = %+v, want no hand-back (the later Aura was already the tracked grant)", ch)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: second, From: state.ZBattlefield, To: state.ZGraveyard})
	if got := e.G.Obj(bear).Controller; got != 1 {
		t.Fatalf("bearer controller after both Auras left = %d, want 1 (its owner)", got)
	}
	replayCheck(t, e, cfg)
}
