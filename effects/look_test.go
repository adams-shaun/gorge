package effects

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The round-2 review's class fix (Gitaxian Probe's Look$ leak): every
// looker-scoped effect records its private look through effects' emitLook —
// one Secret Note per looker, Player = that looker. This file pins that
// contract across ALL of them, table-driven, so the next look-shaped effect
// that bypasses the helper fails here.

// lookBoard builds a 2-seat game; seat 1's hand holds a creature, a land and
// an instant (in that order); seat 0's library holds one land.
func lookBoard(t *testing.T) (*askHost, []state.ObjID, state.ObjID) {
	t.Helper()
	h := &askHost{}
	h.g = state.NewGame(names(2))
	hand := []state.ObjID{
		h.g.AddObject(mkCard(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n"), 1).ID,
		h.g.AddObject(mkCard(t, "Name:Isle\nTypes:Basic Land Island\nOracle:x\n"), 1).ID,
		h.g.AddObject(mkCard(t, "Name:Bolt\nTypes:Instant\nOracle:x\n"), 1).ID,
	}
	for _, id := range hand {
		h.g.Obj(id).Zone = state.ZHand
	}
	h.g.SetZone(state.ZHand, 1, hand)
	mtn := h.g.AddObject(mkCard(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n"), 0).ID
	mtn2 := h.g.AddObject(mkCard(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n"), 0).ID
	bear0 := h.g.AddObject(mkCard(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0).ID
	for _, id := range []state.ObjID{mtn, mtn2, bear0} {
		h.g.Obj(id).Zone = state.ZLibrary
	}
	h.g.SetZone(state.ZLibrary, 0, []state.ObjID{mtn, mtn2, bear0})
	return h, hand, mtn
}

// secretLookNotes returns the Secret look Notes (emitLook's shape).
func secretLookNotes(log []events.Event) []events.Event {
	var out []events.Event
	for _, e := range log {
		if e.Kind == events.Note && e.Secret {
			out = append(out, e)
		}
	}
	return out
}

// TestEveryLookerScopedEffectRecordsItsLookThroughEmitLook is the class
// fix's table: each looker-scoped effect in the engine, resolved through its
// own synthetic SA (the real corpus SAs are pinned in revealhand_test.go and
// below), must record its look as exactly one Secret Note scoped to the
// entitled looker, and — where the effect also asks — the ask rides the look.
func TestEveryLookerScopedEffectRecordsItsLookThroughEmitLook(t *testing.T) {
	cases := []struct {
		name    string
		line    string
		hand    bool // the pool is the target's hand, not the looker's library
		wantIDs func(hand []state.ObjID) []state.ObjID
		wantAsk bool
	}{
		{
			name:    "RevealHand Look$ (the Gitaxian Probe shape)",
			line:    "SP$ RevealHand | ValidTgts$ Player | Look$ True",
			hand:    true,
			wantIDs: func(hand []state.ObjID) []state.ObjID { return hand },
		},
		{
			name:    "Dig ask-path look",
			line:    "SP$ Dig | Defined$ You | DigNum$ 3 | ChangeNum$ 1 | ChangeValid$ Land",
			wantIDs: nil, // the whole top-3 window, asserted separately below
			wantAsk: true,
		},
		{
			name:    "RearrangeTopOfLibrary look",
			line:    "ST$ RearrangeTopOfLibrary | Defined$ You | NumCards$ 2",
			wantIDs: func(_ []state.ObjID) []state.ObjID { return nil },
			wantAsk: true,
		},
		{
			name:    "Scry look",
			line:    "SP$ Scry | Defined$ You | ScryNum$ 2",
			wantIDs: func(_ []state.ObjID) []state.ObjID { return nil },
			wantAsk: true,
		},
		{
			name:    "Surveil look",
			line:    "SP$ Surveil | Defined$ You | Amount$ 2",
			wantIDs: func(_ []state.ObjID) []state.ObjID { return nil },
			wantAsk: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, hand, _ := lookBoard(t)
			ctx := &Ctx{Controller: 0}
			if tc.hand {
				ctx.Targets = []state.Target{{Player: 1, IsPlayer: true}}
			}
			Resolve(h, ctx, sa(t, tc.line))
			if !tc.wantAsk && h.asked != nil && h.asked.ResumeKind == "look_ack" {
				// A bare private look (lookack) now gates on its pacing ack:
				// answer it the way rules' resume arm does — set Ctx.LookAck
				// and re-enter — before the note exists to count.
				ctx.LookAck = true
				h.asked = nil
				Resolve(h, ctx, sa(t, tc.line))
			}
			notes := secretLookNotes(h.log)
			if len(notes) != 1 {
				t.Fatalf("got %d Secret look Notes (%+v), want exactly one", len(notes), notes)
			}
			n := notes[0]
			if n.Player != 0 {
				t.Fatalf("look Note Player = %d, want the entitled looker (seat 0)", n.Player)
			}
			if !n.Secret {
				t.Fatal("look Note is not Secret")
			}
			if n.Text == "" && n.From == state.ZLibrary {
				t.Fatalf("a library-top look must carry its Text clause, got %+v", n)
			}
			if tc.wantIDs != nil {
				if want := tc.wantIDs(hand); !slices.Equal(n.IDs, want) {
					t.Fatalf("look ids = %v, want %v", n.IDs, want)
				}
			}
			if tc.wantAsk && h.asked == nil {
				t.Fatal("no ask was posed alongside the look")
			}
		})
	}
}

// TestDigLookCarriesTheWholeWindow pins the one table row whose ids need
// their own assertion: the Dig look carries the WINDOW (every card looked
// at), not only the ChangeValid$-eligible ones.
func TestDigLookCarriesTheWholeWindow(t *testing.T) {
	h, _, _ := lookBoard(t)
	Resolve(h, &Ctx{Controller: 0},
		sa(t, "SP$ Dig | Defined$ You | DigNum$ 3 | ChangeNum$ 1 | ChangeValid$ Land"))
	notes := secretLookNotes(h.log)
	if len(notes) != 1 {
		t.Fatalf("got %d Secret look Notes, want exactly one", len(notes))
	}
	want := h.g.Zone(state.ZLibrary, 0)
	if !slices.Equal(notes[0].IDs, want) {
		t.Fatalf("look ids = %v, want the whole window %v", notes[0].IDs, want)
	}
}

// TestSlayersBountyLookShowsOnlyCreatureCards pins RevealType$ on its REAL
// compiled corpus SA (Slayer's Bounty's ETB trigger: "look at the creature
// cards in target opponent's hand"): of a hand holding a creature, a land
// and an instant, the look carries the creature alone.
func TestSlayersBountyLookShowsOnlyCreatureCards(t *testing.T) {
	h, hand, _ := lookBoard(t)
	sa2 := corpusSAByAPI(t, "Slayer's Bounty", "DB", "RevealHand")
	if sa2.Params["Look"] != "True" || sa2.Params["RevealType"] != "Creature" {
		t.Fatalf("corpus pin moved: Look=%q RevealType=%q", sa2.Params["Look"], sa2.Params["RevealType"])
	}
	src := h.g.AddObject(mkCard(t, "Name:Slayer's Bounty\nManaCost:W\nTypes:Legendary Artifact Clue\nOracle:x\n"), 0)
	ctx := &Ctx{Source: src.ID, Controller: 0,
		Targets: []state.Target{{Player: 1, IsPlayer: true}}}
	Resolve(h, ctx, sa2)
	if h.asked == nil || h.asked.ResumeKind != "look_ack" || h.asked.Player != 0 {
		t.Fatalf("the bare look posed %+v, want a look_ack for the looker (seat 0)", h.asked)
	}
	if !strings.Contains(h.asked.Prompt, "Bear") {
		t.Fatalf("ack prompt = %q, want it to name the creature card", h.asked.Prompt)
	}
	// Answer the ack the way rules' resume arm does and re-enter.
	ctx.LookAck = true
	h.asked = nil
	Resolve(h, ctx, sa2)
	notes := secretLookNotes(h.log)
	if len(notes) != 1 {
		t.Fatalf("got %d Secret look Notes, want exactly one", len(notes))
	}
	if notes[0].Player != 0 || !notes[0].Secret || notes[0].From != state.ZHand {
		t.Fatalf("look Note = %+v, want Secret to the activator over the hand", notes[0])
	}
	if !slices.Equal(notes[0].IDs, hand[:1]) {
		t.Fatalf("look ids = %v, want only the creature %v", notes[0].IDs, hand[:1])
	}
}

// TestLiarsPendulumMayRevealIsAsked pins Optional$ True on the real
// RevealHand shape Liar's Pendulum carries ("you may reveal your hand",
// asked of the player whose hand it is): the ask is posed, a declined reveal
// emits nothing, and an accepted one emits the ordinary PUBLIC reveal Note.
func TestLiarsPendulumMayRevealIsAsked(t *testing.T) {
	line := "DB$ RevealHand | Defined$ You | Optional$ True"
	h := &askHost{}
	h.g = state.NewGame(names(2))
	hand := []state.ObjID{
		h.g.AddObject(mkCard(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0).ID,
		h.g.AddObject(mkCard(t, "Name:Isle\nTypes:Basic Land Island\nOracle:x\n"), 0).ID,
	}
	for _, id := range hand {
		h.g.Obj(id).Zone = state.ZHand
	}
	h.g.SetZone(state.ZHand, 0, hand)
	src := h.g.AddObject(mkCard(t, "Name:T\nTypes:Artifact\nOracle:x\n"), 0)

	// Fresh walk: the yes/no ask is posed to the hand's owner and suspends.
	Resolve(h, &Ctx{Source: src.ID, Controller: 0}, sa(t, line))
	if h.asked == nil {
		t.Fatal("no may-reveal ask was posed")
	}
	d := h.asked
	if d.Player != 0 || d.Kind != decision.KChoose || d.ResumeKind != "reveal_optional" {
		t.Fatalf("ask = %+v, want a reveal_optional KChoose for seat 0", d)
	}
	if len(d.Options) != 2 || d.Options[0].Kind != "yes" || d.Options[0].Label != "Yes — reveal" || d.Options[1].Kind != "no" {
		t.Fatalf("options = %+v, want Yes — reveal / No", d.Options)
	}
	if len(h.log) != 0 {
		t.Fatalf("a suspended may-reveal emitted %v before its answer", h.log)
	}

	// Declined: nothing is revealed.
	h2 := &askHost{fakeHost: fakeHost{g: h.g}}
	Resolve(h2, &Ctx{Source: src.ID, Controller: 0, RevealOpt: "no"}, sa(t, line))
	if notes := secretLookNotes(h2.log); len(notes) != 0 {
		t.Fatalf("a declined reveal emitted %v", notes)
	}
	for _, e := range h2.log {
		if e.Kind == events.Note && len(e.IDs) > 0 {
			t.Fatalf("a declined reveal emitted a note naming cards: %+v", e)
		}
	}

	// Accepted: the ordinary public reveal Note (Ruling T23-w's shape).
	h3 := &askHost{fakeHost: fakeHost{g: h.g}}
	Resolve(h3, &Ctx{Source: src.ID, Controller: 0, RevealOpt: "yes"}, sa(t, line))
	var note *events.Event
	for i := range h3.log {
		if h3.log[i].Kind == events.Note {
			note = &h3.log[i]
		}
	}
	if note == nil {
		t.Fatal("an accepted reveal emitted no Note")
	}
	if note.Secret || note.Player != 0 || !slices.Equal(note.IDs, hand) {
		t.Fatalf("accepted reveal Note = %+v, want public, Player 0, the whole hand", note)
	}
}
