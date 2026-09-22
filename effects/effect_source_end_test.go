package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestChangeZoneCommandSelfExileEndsTheBoundEffectFrame pins the one-shot
// Effect self-exile idiom (`DB$ ChangeZone | Defined$ Self | Origin$ Command
// | Destination$ Exile`, Deflecting Palm's ExileEffect) run inside an
// Effect-created replacement body: Ctx.EffectFrame names the registration
// the body belongs to, and the shape ends exactly that registration -- not
// the same source's OTHER registration, not another source's. Both spellings
// the corpus uses for the effect object (Defined$ Self, and an absent
// Defined$) are pinned.
func TestChangeZoneCommandSelfExileEndsTheBoundEffectFrame(t *testing.T) {
	for _, defined := range []string{"Defined$ Self | ", ""} {
		h := newHost(t, 2)
		src := h.g.AddObject(mkCard(t, "Name:Frame\nTypes:Instant\nOracle:x\n"), 0)
		h.AddContinuous(state.ContinuousEffect{Source: src.ID, Timestamp: 7, Controller: 0, Affects: "Card.Self"})
		h.AddContinuous(state.ContinuousEffect{Source: src.ID, Timestamp: 8, Controller: 0, Affects: "Card.Self"})
		h.AddContinuous(state.ContinuousEffect{Source: 9999, Timestamp: 7, Controller: 1, Affects: "Card.Self"})
		if len(h.continuous) != 3 {
			t.Fatalf("precondition: %d registrations, want 3", len(h.continuous))
		}

		c := &Ctx{Source: src.ID, Controller: 0, EffectFrame: EffectFrame{Source: src.ID, Stamp: 7}}
		Resolve(h, c, sa(t, "DB$ ChangeZone | "+defined+"Origin$ Command | Destination$ Exile"))

		if len(h.continuous) != 2 {
			t.Fatalf("%q: registrations = %+v, want the frame (src,7) dropped and the other two kept", defined, h.continuous)
		}
		for _, ce := range h.continuous {
			if ce.Source == src.ID && ce.Timestamp == 7 {
				t.Fatalf("%q: the bound frame survived its self-exile", defined)
			}
		}
		if got := h.g.Obj(src.ID).Zone; got == state.ZExile {
			t.Fatalf("%q: the effect frame was exiled as a card", defined)
		}
	}
}

// TestChangeZoneCommandSelfExileWithoutAFrameEndsNothing pins the gate's
// other side: the same shape resolved outside an Effect-created replacement
// body (no Ctx.EffectFrame -- an ordinary spell, ability or printed
// replacement) ends no registration; the ordinary move path runs unchanged.
func TestChangeZoneCommandSelfExileWithoutAFrameEndsNothing(t *testing.T) {
	h := newHost(t, 2)
	src := h.g.AddObject(mkCard(t, "Name:Frame\nTypes:Instant\nOracle:x\n"), 0)
	h.AddContinuous(state.ContinuousEffect{Source: src.ID, Timestamp: 7, Controller: 0, Affects: "Card.Self"})

	Resolve(h, &Ctx{Source: src.ID, Controller: 0}, sa(t, "DB$ ChangeZone | Defined$ Self | Origin$ Command | Destination$ Exile"))

	if len(h.continuous) != 1 {
		t.Fatalf("an unframed self-exile ended a registration: %+v", h.continuous)
	}
}

// TestChooseSourceFreshEntryReplacesAStaleChosenCard pins effChooseSource's
// fresh-entry normalisation (effChooseCard's setChosenCards read): a Ctx
// already carrying a chosen CARD from an earlier choice on the same
// resolution ends with only the newly chosen source, matching the Choose
// "chosen" fold that REPLACES the source object's list -- so the ctx read
// (Defined$ ChosenCard) and the object read (a replacement's ValidSource$
// Card.ChosenCardStrict) agree.
func TestChooseSourceFreshEntryReplacesAStaleChosenCard(t *testing.T) {
	h := newHost(t, 2)
	spell := h.g.AddObject(mkCard(t, "Name:Palm\nTypes:Instant\nOracle:x\n"), 0)
	stale := h.g.AddObject(mkCard(t, "Name:Stale\nTypes:Artifact\nOracle:x\n"), 0)
	stale.Zone = state.ZGraveyard
	src := h.g.AddObject(mkCard(t, "Name:Aggressor\nTypes:Creature\nPT:2/2\nOracle:x\n"), 1)
	src.Zone = state.ZBattlefield

	c := &Ctx{Source: spell.ID, Controller: 0, Chosen: []state.Target{{Obj: stale.ID}}, ChosenValid: true}
	Resolve(h, c, sa(t, "SP$ ChooseSource | Choices$ Card"))

	if len(c.Chosen) != 1 || c.Chosen[0].Obj != src.ID {
		t.Fatalf("ctx chosen = %+v, want only the newly chosen source %d (stale %d dropped)", c.Chosen, src.ID, stale.ID)
	}
}
