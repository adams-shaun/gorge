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

// TestFarmSkyshipBuccaneerReversedAttachOrder is the sibling direction of the
// farm case: seat 1's Confiscate attaches FIRST and seat 0's attaches later,
// so CR 613.1b gives the bearer to the seat that comes FIRST in the static
// scan order. Selection in scan order (the previous latest-want filter) would
// keep seat 1's Aura instead; only source-timestamp order gives seat 0 the
// bearer. Removing the later grant then returns the bearer to the surviving
// earlier Aura, not to its owner mid-chain.
func TestFarmSkyshipBuccaneerReversedAttachOrder(t *testing.T) {
	t.Parallel()
	conf, ok := testutil.CorpusRegistry(t).Lookup("Confiscate")
	if !ok {
		t.Fatal("corpus missing Confiscate")
	}
	e, cfg, bear := staticGainControlGame(t, 49119, []*cards.Card{conf}, 1, conf)
	attach := func(seat state.PlayerID) state.ObjID {
		t.Helper()
		id := findAndMoveToHand(t, e, seat, "Confiscate")
		// Entry and attachment in one step, as a resolving Aura spell does:
		// a drain between the two would sweep the unattached Aura (CR 704.5m).
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
		e.emit(events.Event{Kind: events.Attach, Obj: id, IDs: []state.ObjID{bear}})
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || o.AttachedTo != bear || o.Controller != seat {
			t.Fatalf("precondition: seat %d's Confiscate %d = %+v, want it attached to bear %d", seat, id, o, bear)
		}
		return id
	}
	// Precondition: seat 1's Aura attaches first and takes the seat 0-owned
	// bearer; seat 0's later Aura is the later effect.
	earlier := attach(1)
	if got := e.G.Obj(bear).Controller; got != 1 {
		t.Fatalf("precondition: controller after seat 1's Confiscate = %d, want 1", got)
	}
	later := attach(0)
	earlierTS, laterTS := e.G.Obj(earlier).Timestamp, e.G.Obj(later).Timestamp
	if earlierTS >= laterTS {
		t.Fatalf("precondition: Aura timestamps %d and %d, want seat 1's Aura earlier", earlierTS, laterTS)
	}
	// CR 613.1b: the later Confiscate wins even though its seat is first in
	// the static scan order.
	if got := e.G.Obj(bear).Controller; got != 0 {
		t.Fatalf("bearer controller = %d, want 0 (the later Confiscate, CR 613.1b)", got)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: later, From: state.ZBattlefield, To: state.ZGraveyard})
	if got := e.G.Obj(bear).Controller; got != 1 {
		t.Fatalf("bearer controller after the later Confiscate left = %d, want 1 (seat 1's surviving Confiscate)", got)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: earlier, From: state.ZBattlefield, To: state.ZGraveyard})
	if got := e.G.Obj(bear).Controller; got != 1 {
		t.Fatalf("bearer controller after both Confiscates left = %d, want 1 (its owner)", got)
	}
	// The owner is seat 1, so the controller above cannot tell a finished
	// static grant from an unended one: assert the grant itself is gone.
	for i := range e.controlGrants {
		if e.controlGrants[i].Obj == bear {
			t.Fatalf("control grant %+v survives both Confiscates, want none (the last static grant must end)", e.controlGrants[i])
		}
	}
	replayCheck(t, e, cfg)
}

// TestFarmSkyshipBuccaneerEarlierAuraChangesController pins CR 613.7 for the
// case the FDN-s491 board can reach after setup: the EARLIER control static's
// controller changes after the LATER one is already tracked (here seat 1
// takes seat 0's earlier Confiscate, as a third control effect would). Layer
// 2 applies the statics in timestamp order and they are not additive, so the
// later Aura still wins and the bearer must stay with seat 0. Re-deriving the
// wanted set and appending the re-registered earlier grant would hand the
// bearer to seat 1 instead.
func TestFarmSkyshipBuccaneerEarlierAuraChangesController(t *testing.T) {
	t.Parallel()
	conf, ok := testutil.CorpusRegistry(t).Lookup("Confiscate")
	if !ok {
		t.Fatal("corpus missing Confiscate")
	}
	e, cfg, bear := staticGainControlGame(t, 49120, []*cards.Card{conf, conf}, 1)
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
	earlier := castConfiscate()
	later := castConfiscate()
	if earlier == later {
		t.Fatalf("both casts used obj %d, want two Auras", earlier)
	}
	// Preconditions: the Auras are from the same seat, both attached to the
	// seat-1-owned bear, with the first strictly earlier.
	if e.G.Obj(earlier).Timestamp >= e.G.Obj(later).Timestamp {
		t.Fatalf("precondition: Aura timestamps %d and %d, want the first Aura earlier",
			e.G.Obj(earlier).Timestamp, e.G.Obj(later).Timestamp)
	}
	if got := e.G.Obj(bear).Controller; got != 0 {
		t.Fatalf("precondition: bearer controller = %d, want 0 (seat 0's two Confiscates)", got)
	}
	// Seat 1 takes the EARLIER Aura. Its static now names seat 1, but its
	// effect timestamp is still older than the later Aura's, so it does not
	// win layer 2 (CR 613.7a).
	e.emit(events.Event{Kind: events.ControlChange, Obj: earlier, Player: 1})
	if got := e.G.Obj(earlier).Controller; got != 1 {
		t.Fatalf("precondition: earlier Aura controller = %d, want 1", got)
	}
	if got := e.G.Obj(bear).Controller; got != 0 {
		t.Fatalf("bearer controller after seat 1 took the earlier Aura = %d, want 0 (the later Aura still wins, CR 613.7)", got)
	}
	// Ending the later (winning) effect hands the bear to the surviving
	// earlier Aura's CURRENT controller, seat 1.
	e.emit(events.Event{Kind: events.MoveZone, Obj: later, From: state.ZBattlefield, To: state.ZGraveyard})
	if got := e.G.Obj(bear).Controller; got != 1 {
		t.Fatalf("bearer controller after the later Aura left = %d, want 1 (the earlier Aura's controller)", got)
	}
	replayCheck(t, e, cfg)
}
