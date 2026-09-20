package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Task prolif1: api:Proliferate (CR 701.27). The primitive was unregistered,
// so every `DB$ Proliferate` / `AB$ Proliferate` / `SP$ Proliferate` line in
// the corpus degraded to one "unimplemented API Proliferate" Note and added
// nothing. These effects-level leaves pin the primitive itself on synthetic
// scripts; the end-to-end real-corpus carriers live in
// rules/proliferate_test.go.

// proliferateSA builds the bare `DB$ Proliferate` shape with any extra
// parameters appended.
func proliferateSA(t *testing.T, extra string) *cards.SA {
	t.Helper()
	line := "DB$ Proliferate"
	if extra != "" {
		line += " | " + extra
	}
	return sa(t, line)
}

func TestProliferateAsksAndAppliesObjectsAndPlayers(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	// Seat 0 permanent with two counter kinds; seat 1 permanent with a
	// counter; seat 1 player with poison. A bear carrying NO counter must be
	// absent from the option list (CR 701.27a).
	withCounters := h.g.AddObject(mkCard(t, "Name:Carrier\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	withCounters.Zone = state.ZBattlefield
	withCounters.AddCounter("P1P1", 2)
	withCounters.AddCounter("CHARGE", 1)
	other := h.g.AddObject(mkCard(t, "Name:Other\nTypes:Creature\nPT:1/1\nOracle:x\n"), 1)
	other.Zone = state.ZBattlefield
	other.AddCounter("P1P1", 1)
	bare := h.g.AddObject(mkCard(t, "Name:Bare\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	bare.Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{withCounters.ID, bare.ID})
	h.g.SetZone(state.ZBattlefield, 1, []state.ObjID{other.ID})
	h.g.Players[1].AddCounter("POISON", 1)

	c := &Ctx{Source: withCounters.ID, Controller: 0}
	Resolve(h, c, proliferateSA(t, ""))
	if h.asked == nil {
		t.Fatal("no proliferate decision was posed")
	}
	d := h.asked
	if d.Kind != decision.KChoose || d.Min != 0 || d.Max != 3 || d.Player != 0 {
		t.Fatalf("decision = %+v, want a Min 0 / Max 3 KChoose for the controller", d)
	}
	if d.ResumeKind != "proliferate" {
		t.Fatalf("ResumeKind = %q, want \"proliferate\"", d.ResumeKind)
	}
	// Options: seat 0 carrier, seat 1 object, seat 1 player. The bare bear is
	// not offered.
	if len(d.Options) != 3 {
		t.Fatalf("options = %+v, want 3 eligible (no zero-counter recipient)", d.Options)
	}
	if d.Options[0].Obj != withCounters.ID || d.Options[1].Obj != other.ID {
		t.Fatalf("option objects = %d,%d; want %d,%d", d.Options[0].Obj,
			d.Options[1].Obj, withCounters.ID, other.ID)
	}
	if d.Options[2].Obj != 0 || d.Options[2].Player != 1 {
		t.Fatalf("player option = %+v, want Obj 0 / Player 1", d.Options[2])
	}
	for _, o := range d.Options {
		if o.Obj == bare.ID {
			t.Fatal("a recipient carrying no counter was offered")
		}
	}
	// Nothing was applied before the answer.
	if withCounters.Counter("P1P1") != 2 || other.Counter("P1P1") != 1 || h.g.Players[1].Counter("POISON") != 1 {
		t.Fatal("counters were applied before the choice was made")
	}

	// Re-entry: the engine's contract, a fresh Ctx carrying the answered
	// picks (object AND player). Choose the carrier, the other object and the
	// player.
	rc := &Ctx{Source: c.Source, Controller: 0,
		Proliferate: []state.Target{
			{Obj: withCounters.ID},
			{Obj: other.ID},
			{Player: 1, IsPlayer: true},
		},
		ProliferateDone: true}
	Resolve(h, rc, proliferateSA(t, ""))
	if got := withCounters.Counter("P1P1"); got != 3 {
		t.Fatalf("carrier P1P1 = %d, want 3 (+1)", got)
	}
	if got := withCounters.Counter("CHARGE"); got != 2 {
		t.Fatalf("carrier CHARGE = %d, want 2 (+1 of the second kind)", got)
	}
	if got := other.Counter("P1P1"); got != 2 {
		t.Fatalf("other P1P1 = %d, want 2 (+1)", got)
	}
	if got := h.g.Players[1].Counter("POISON"); got != 2 {
		t.Fatalf("player POISON = %d, want 2 (+1)", got)
	}
	// The player counter arrived as a real PlayerCounterChange event.
	sawPlayer := false
	for _, ev := range h.log {
		if ev.Kind == events.PlayerCounterChange && ev.Player == 1 &&
			ev.Counter == "POISON" && ev.Amount == 1 {
			sawPlayer = true
		}
	}
	if !sawPlayer {
		t.Fatal("no PlayerCounterChange for the chosen player")
	}
}

func TestProliferateZeroEligibleDoesNotAsk(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	bare := h.g.AddObject(mkCard(t, "Name:Bare\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	bare.Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{bare.ID})
	c := &Ctx{Source: bare.ID, Controller: 0}
	Resolve(h, c, proliferateSA(t, ""))
	if h.asked != nil {
		t.Fatalf("zero eligible recipients posed a decision: %+v", h.asked)
	}
	for _, ev := range h.log {
		if ev.Kind == events.Note {
			t.Fatalf("zero eligible recipients emitted a Note: %q", ev.Text)
		}
	}
}

func TestProliferateSingleEligibleStillAsks(t *testing.T) {
	// The any-number convention, NOT putCounterChoose's strict-supersets
	// gate: "{} vs {that one}" is a real election even with one eligible.
	h := &askHost{}
	h.g = state.NewGame(names(2))
	carrier := h.g.AddObject(mkCard(t, "Name:Carrier\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	carrier.Zone = state.ZBattlefield
	carrier.AddCounter("P1P1", 1)
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{carrier.ID})
	c := &Ctx{Source: carrier.ID, Controller: 0}
	Resolve(h, c, proliferateSA(t, ""))
	if h.asked == nil {
		t.Fatal("a single eligible recipient did not pose the any-number ask")
	}
	if h.asked.Max != 1 || h.asked.Min != 0 {
		t.Fatalf("ask bounds = Min %d / Max %d, want 0/1", h.asked.Min, h.asked.Max)
	}
}

func TestProliferateNoHostTakesAllEligible(t *testing.T) {
	// fakeHost.Ask returns false (AskNoHost): the R-9 stand-in takes every
	// eligible recipient with one Note, the exact mirror of botpolicy's arm.
	h := &fakeHost{}
	h.g = state.NewGame(names(2))
	carrier := h.g.AddObject(mkCard(t, "Name:Carrier\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	carrier.Zone = state.ZBattlefield
	carrier.AddCounter("P1P1", 1)
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{carrier.ID})
	h.g.Players[0].AddCounter("POISON", 1)
	c := &Ctx{Source: carrier.ID, Controller: 0}
	Resolve(h, c, proliferateSA(t, ""))
	if carrier.Counter("P1P1") != 2 {
		t.Fatalf("carrier P1P1 = %d, want 2", carrier.Counter("P1P1"))
	}
	if h.g.Players[0].Counter("POISON") != 2 {
		t.Fatalf("player POISON = %d, want 2", h.g.Players[0].Counter("POISON"))
	}
	notes := 0
	for _, ev := range h.log {
		if ev.Kind == events.Note {
			notes++
		}
	}
	if notes != 1 {
		t.Fatalf("no-host Notes = %d, want exactly 1", notes)
	}
}

func TestProliferateAmountAppliesPerKindPerRecipient(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	carrier := h.g.AddObject(mkCard(t, "Name:Carrier\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	carrier.Zone = state.ZBattlefield
	carrier.AddCounter("P1P1", 1)
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{carrier.ID})
	// Amount$ 2: one ask, +2 of each kind (the documented single-batch read).
	c := &Ctx{Source: carrier.ID, Controller: 0}
	Resolve(h, c, proliferateSA(t, "Amount$ 2"))
	if h.asked == nil {
		t.Fatal("Amount$ 2 did not pose the ask")
	}
	rc := &Ctx{Source: carrier.ID, Controller: 0,
		Proliferate: []state.Target{{Obj: carrier.ID}}, ProliferateDone: true}
	Resolve(h, rc, proliferateSA(t, "Amount$ 2"))
	if got := carrier.Counter("P1P1"); got != 3 {
		t.Fatalf("carrier P1P1 = %d, want 3 (+2)", got)
	}
}

func TestProliferateAmountZeroIsSilent(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	carrier := h.g.AddObject(mkCard(t, "Name:Carrier\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	carrier.Zone = state.ZBattlefield
	carrier.AddCounter("P1P1", 1)
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{carrier.ID})
	c := &Ctx{Source: carrier.ID, Controller: 0}
	Resolve(h, c, proliferateSA(t, "Amount$ 0"))
	if h.asked != nil {
		t.Fatal("Amount$ 0 posed an ask")
	}
	if carrier.Counter("P1P1") != 1 {
		t.Fatal("Amount$ 0 added a counter")
	}
}

func TestProliferateAmountXWithoutAnnouncementIsLoud(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	carrier := h.g.AddObject(mkCard(t, "Name:Carrier\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	carrier.Zone = state.ZBattlefield
	carrier.AddCounter("P1P1", 1)
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{carrier.ID})
	c := &Ctx{Source: carrier.ID, Controller: 0} // c.X == 0, unannounced
	Resolve(h, c, proliferateSA(t, "Amount$ X"))
	if h.asked != nil {
		t.Fatal("an unannounced Amount$ X posed an ask")
	}
	found := false
	for _, ev := range h.log {
		if ev.Kind == events.Note {
			found = true
		}
	}
	if !found {
		t.Fatal("an unannounced Amount$ X did not loud-degrade")
	}
}

func TestProliferateRememberPutRecordsRecipients(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	carrier := h.g.AddObject(mkCard(t, "Name:Carrier\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	carrier.Zone = state.ZBattlefield
	carrier.AddCounter("P1P1", 1)
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{carrier.ID})
	c := &Ctx{Source: carrier.ID, Controller: 0,
		Proliferate: []state.Target{{Obj: carrier.ID}}, ProliferateDone: true}
	Resolve(h, c, proliferateSA(t, "RememberPut$ True"))
	if len(c.Remembered) != 1 || c.Remembered[0].Obj != carrier.ID {
		t.Fatalf("Remembered = %+v, want exactly the countered carrier", c.Remembered)
	}
}

func TestProliferateUnknownParameterIsLoud(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	carrier := h.g.AddObject(mkCard(t, "Name:Carrier\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	carrier.Zone = state.ZBattlefield
	carrier.AddCounter("P1P1", 1)
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{carrier.ID})
	c := &Ctx{Source: carrier.ID, Controller: 0}
	Resolve(h, c, proliferateSA(t, "TotallyUnknown$ 7"))
	if h.asked != nil {
		t.Fatal("an unmodelled parameter still posed an ask")
	}
	found := false
	for _, ev := range h.log {
		if ev.Kind == events.Note {
			found = true
		}
	}
	if !found {
		t.Fatal("an unmodelled parameter did not loud-degrade")
	}
}
