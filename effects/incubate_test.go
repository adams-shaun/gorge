package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// incubatorFixture is an authored two-face Incubator token (never a corpus
// tokenscript copy -- that text is GPL-3.0). It mirrors the real
// incubator_c_0_0_a_phyrexian shape: an Artifact Incubator with the {2}
// transform ability whose ALTERNATE face is a 0/0 Phyrexian artifact
// creature.
const incubatorFixture = "Name:Incubator Token\nManaCost:no cost\nTypes:Artifact Incubator\n" +
	"A:AB$ SetState | Cost$ 2 | Mode$ Transform | SpellDescription$ Transform this token.\n" +
	"AlternateMode:DoubleFaced\nOracle:x\n\nALTERNATE\n\n" +
	"Name:Phyrexian Test Token\nManaCost:no cost\nTypes:Artifact Creature Phyrexian\nPT:0/0\nOracle:\n"

// incubatorHost is fixtureHost's 2-seat game plus the authored Incubator
// token in Game.Tokens under the real corpus stem.
func incubatorHost(t *testing.T) (*fakeHost, *Ctx) {
	t.Helper()
	h, c := fixtureHost(t)
	h.g.Tokens = map[string]*cards.Card{incubatorTokenKey: mkCard(t, incubatorFixture)}
	return h, c
}

// battlefieldTokens returns the Incubator-name tokens seat 0 holds on the
// battlefield.
func battlefieldTokens(h *fakeHost) []*state.Object {
	var out []*state.Object
	for _, id := range h.g.Zone(state.ZBattlefield, 0) {
		if o := h.g.Obj(id); o != nil && o.IsToken && o.Face() != nil && o.Face().Name == "Incubator Token" {
			out = append(out, o)
		}
	}
	return out
}

func TestIncubateMintsTokenWithTheCounters(t *testing.T) {
	h, c := incubatorHost(t)
	Resolve(h, c, sa(t, "DB$ Incubate | Amount$ 2"))
	toks := battlefieldTokens(h)
	if len(toks) != 1 {
		t.Fatalf("Incubate 2 minted %d Incubator tokens, want 1", len(toks))
	}
	if got := toks[0].Counter("P1P1"); got != 2 {
		t.Fatalf("Incubator token carries %d +1/+1 counters, want 2", got)
	}
}

func TestIncubateTimesRepeatsTheMint(t *testing.T) {
	h, c := incubatorHost(t)
	Resolve(h, c, sa(t, "DB$ Incubate | Amount$ 2 | Times$ 3"))
	toks := battlefieldTokens(h)
	if len(toks) != 3 {
		t.Fatalf("Incubate 2 three times minted %d tokens, want 3", len(toks))
	}
	for i, tok := range toks {
		if got := tok.Counter("P1P1"); got != 2 {
			t.Fatalf("token %d carries %d counters, want 2", i, got)
		}
	}
}

func TestIncubateZeroAmountStillMintsWithoutACounterEvent(t *testing.T) {
	h, c := incubatorHost(t)
	Resolve(h, c, sa(t, "DB$ Incubate | Amount$ 0"))
	toks := battlefieldTokens(h)
	if len(toks) != 1 {
		t.Fatalf("Incubate 0 minted %d tokens, want 1 (a zero-amount incubate is a real token)", len(toks))
	}
	if got := toks[0].Counter("P1P1"); got != 0 {
		t.Fatalf("token carries %d counters, want 0", got)
	}
	for _, ev := range h.log {
		if ev.Kind == events.CounterChange {
			t.Fatalf("a zero-amount incubate emitted a CounterChange: %+v", ev)
		}
	}
}

func TestIncubateNegativeAmountCreatesNothing(t *testing.T) {
	h, c := incubatorHost(t)
	Resolve(h, c, sa(t, "DB$ Incubate | Amount$ -1"))
	if toks := battlefieldTokens(h); len(toks) != 0 {
		t.Fatalf("Incubate -1 minted %d tokens, want 0", len(toks))
	}
}

func TestIncubateUnknownTokenScriptNotesAndCreatesNothing(t *testing.T) {
	h, c := incubatorHost(t)
	Resolve(h, c, sa(t, "DB$ Incubate | Amount$ 2 | TokenScript$ no_such_stem"))
	if toks := battlefieldTokens(h); len(toks) != 0 {
		t.Fatalf("unknown token script minted %d tokens, want 0", len(toks))
	}
	loud := false
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "no Incubator token script available") {
			loud = true
		}
	}
	if !loud {
		t.Fatalf("no loud Note for the missing token script; log = %+v", h.log)
	}
}

func TestIncubateDefinedTargetedControllerMintsForTheTargetPlayer(t *testing.T) {
	h, c := incubatorHost(t)
	c.Targets = []state.Target{{Player: 1, IsPlayer: true}}
	Resolve(h, c, sa(t, "DB$ Incubate | Defined$ TargetedController | Amount$ 1"))
	var owned int
	for _, id := range h.g.Zone(state.ZBattlefield, 1) {
		if o := h.g.Obj(id); o != nil && o.IsToken {
			owned++
		}
	}
	if owned != 1 {
		t.Fatalf("targeted player's incubate minted %d tokens under seat 1, want 1", owned)
	}
	if toks := battlefieldTokens(h); len(toks) != 0 {
		t.Fatalf("seat 0 should hold none, holds %d", len(toks))
	}
}

// TestIncubatorTokenTransformsIntoThePhyrexianFace pins the transform half
// end to end through the token script's own {2} ability: the minted token's
// Face().Abilities carries the SetState activation, and resolving it flips
// the object to the 0/0 Phyrexian creature face while the +1/+1 counters
// stay on the object (so a 2-counter Incubator becomes a 2/2).
func TestIncubatorTokenTransformsIntoThePhyrexianFace(t *testing.T) {
	h, c := incubatorHost(t)
	Resolve(h, c, sa(t, "DB$ Incubate | Amount$ 2"))
	toks := battlefieldTokens(h)
	if len(toks) != 1 {
		t.Fatalf("precondition: %d tokens minted, want 1", len(toks))
	}
	tok := toks[0]
	if len(tok.Card.Faces) != 2 {
		t.Fatalf("precondition: token script compiled %d faces, want 2", len(tok.Card.Faces))
	}
	if len(tok.Face().Abilities) == 0 {
		t.Fatalf("precondition: Incubator token carries no abilities; cannot test the {2} transform")
	}
	Resolve(h, &Ctx{Source: tok.ID, Controller: 0, Targets: []state.Target{{Obj: tok.ID}}},
		sa(t, "DB$ SetState | Defined$ Self | Mode$ Transform"))
	after := h.g.Obj(tok.ID)
	if after == nil || after.FaceIdx != 1 {
		t.Fatalf("token did not transform (FaceIdx %d)", tok.FaceIdx)
	}
	if got := after.Counter("P1P1"); got != 2 {
		t.Fatalf("transformed token carries %d +1/+1 counters, want 2 (counters survive the transform)", got)
	}
	if after.Face() == nil || after.Face().Name != "Phyrexian Test Token" {
		t.Fatalf("transformed face = %+v, want the Phyrexian creature face", after.Face())
	}
}
