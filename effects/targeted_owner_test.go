package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Task tgtowner1: `Defined$ TargetedOwner` joined the shared referent
// grammar (definedSpec), so every consumer that resolves a Defined$/
// DefinedPlayer$/Chooser$/RememberObjects$ spelling resolves the target's
// OWNER (CR 108.3) instead of falling back to the resolving source. Chaos
// Warp's DBDig ("The owner of target permanent ... reveals the top card of
// THEIR library") and Lodestone Bauble's delayed slowtrip remember are the
// corpus carriers the rules-side pins exercise.

// TestDefinedTargetedOwnerResolvesToTargetOwner pins the grammar case,
// imitating TestDefinedTriggeredCardOwners' three-seat fixture: the targeted
// object is OWNED by seat 2 while LAST CONTROLLED by seat 1, and the
// resolving source is owned and controlled by seat 0 -- so owner, controller
// and source owner can never be confused, which is the precondition the
// assertions depend on.
func TestDefinedTargetedOwnerResolvesToTargetOwner(t *testing.T) {
	h := newHost(t, 3)
	card := mkCard(t, "Name:Fixture\nTypes:Creature\nPT:1/1\nOracle:x\n")
	src := h.g.AddObject(card, 0) // the resolving ability's source, seat 0
	tgt := h.g.AddObject(card, 2) // the targeted object: owner seat 2
	tgt.Controller = 1            // ... but last controlled by seat 1
	c := &Ctx{Source: src.ID, Controller: 0, Targets: []state.Target{{Obj: tgt.ID}}}

	// Precondition: the fixture really distinguishes owner from controller
	// and from the resolving source's owner.
	if tgt.Owner != 2 || tgt.Controller != 1 || tgt.Owner == src.Owner {
		t.Fatalf("fixture precondition: target owner=%d controller=%d source owner=%d, want owner 2 != controller 1 != source owner 0",
			tgt.Owner, tgt.Controller, src.Owner)
	}

	got := Defined(h, c, &cards.SA{Params: map[string]string{"Defined": "TargetedOwner"}})
	if len(got) != 1 || !got[0].IsPlayer || got[0].Player != 2 {
		t.Fatalf("Defined$ TargetedOwner = %v, want the target's OWNER seat 2 (CR 108.3), not its controller 1 or the source's owner 0", got)
	}

	// A player target maps to itself.
	c.Targets = []state.Target{{Player: 1, IsPlayer: true}}
	got = Defined(h, c, &cards.SA{Params: map[string]string{"Defined": "TargetedOwner"}})
	if len(got) != 1 || !got[0].IsPlayer || got[0].Player != 1 {
		t.Fatalf("TargetedOwner over a player target = %v, want that player [1]", got)
	}

	// No targets: the KNOWN empty set, never the source controller. The
	// ok=true classification is what routes chooserPlayer and
	// effDelayedTrigger through the grammar instead of their fallbacks.
	c.Targets = nil
	ts, ok := knownDefinedTargets(h, c, "TargetedOwner")
	if !ok {
		t.Fatal("TargetedOwner with no targets must classify as KNOWN (ok=true), not fall back to the source")
	}
	if len(ts) != 0 {
		t.Fatalf("TargetedOwner with no targets = %v, want the empty set", ts)
	}
}

// TestDelayedTriggerRememberObjectsTargetedOwner is Lodestone Bauble's
// slowtrip registration shape: `RememberObjects$ TargetedOwner &
// ChosenPlayer` resolves BOTH referents through the shared grammar -- the
// targeted object's OWNER plus the ChoosePlayer answer -- replacing the
// chain capture, with no unmodelled Note.
func TestDelayedTriggerRememberObjectsTargetedOwner(t *testing.T) {
	h := newHost(t, 3)
	card := mkCard(t, "Name:Fixture\nTypes:Creature\nPT:1/1\nOracle:x\n")
	src := h.g.AddObject(card, 0)
	tgt := h.g.AddObject(card, 2) // targeted object: owner seat 2
	tgt.Controller = 1
	c := &Ctx{Source: src.ID, Controller: 0,
		Remembered: []state.Target{{Obj: 7}}, // the chain capture, replaced below
		Targets:    []state.Target{{Obj: tgt.ID}},
		Chosen:     []state.Target{{Player: 1, IsPlayer: true}}, // the ChoosePlayer answer
	}
	if tgt.Owner == 0 || c.Chosen[0].Player == 0 || tgt.Owner == c.Chosen[0].Player {
		t.Fatalf("fixture precondition: owner=%d chosen=%d source owner=0, want both distinct from 0 and from each other",
			tgt.Owner, c.Chosen[0].Player)
	}

	Resolve(h, c, sa(t, "DB$ DelayedTrigger | Mode$ Phase | Phase$ Upkeep | NextTurn$ True | ValidPlayer$ Player | Execute$ X | RememberObjects$ TargetedOwner & ChosenPlayer"))
	if len(h.log) != 1 || h.log[0].Kind != events.DelayedRegister {
		t.Fatalf("log = %+v, want one DelayedRegister registration", h.log)
	}
	ev := h.log[0]
	want := []state.ObjID{state.PlayerRef(tgt.Owner), state.PlayerRef(c.Chosen[0].Player)}
	if len(ev.IDs) != len(want) || ev.IDs[0] != want[0] || ev.IDs[1] != want[1] {
		t.Fatalf("registration IDs = %v, want the target's owner and the chosen player %v (chain capture 7 must be replaced)", ev.IDs, want)
	}
	for _, e := range h.log {
		if e.Kind == events.Note && strings.Contains(e.Text, "RememberObjects$") {
			t.Fatalf("TargetedOwner & ChosenPlayer still degraded: %q", e.Text)
		}
	}
}
