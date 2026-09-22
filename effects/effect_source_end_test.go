package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestChangeZoneCommandSelfExileEndsEffectSource pins the corpus's universal
// one-shot Effect self-exile idiom (api:ChooseSource's RPreventNextFromSource,
// Unlucky Witness's exile-play frame, Words of Wind, Kor Dirge): Forge keeps
// every DB$ Effect in an implicit effect object in the Command zone, and a
// body that exiles it (`DB$ ChangeZone | Origin$ Command | Destination$
// Exile`) ends the effect after one use. This build has no such object, so the
// shape is recognised structurally and ends the source's registered effects.
//
// The object is named by whichever source-alias spelling the carrier uses --
// an absent Defined$ (the majority of raw lines), Self, or OriginalHost -- so
// all three are pinned; the check is on the RESOLVED target being the source,
// not on a list of literals, so a future alias cannot slip past it.
func TestChangeZoneCommandSelfExileEndsEffectSource(t *testing.T) {
	cases := []struct{ name, defined string }{
		{"no Defined$", ""},
		{"Defined$ Self", "Defined$ Self | "},
		{"Defined$ OriginalHost", "Defined$ OriginalHost | "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHost(t, 2)
			src := h.g.AddObject(mkCard(t, "Name:Frame\nTypes:Sorcery\nOracle:x\n"), 0)
			h.AddContinuous(state.ContinuousEffect{Source: src.ID, Controller: 0, Affects: "Card.Self"})
			h.AddContinuous(state.ContinuousEffect{Source: 9999, Controller: 1, Affects: "Card.Self"})

			line := "DB$ ChangeZone | " + tc.defined + "Origin$ Command | Destination$ Exile"
			Resolve(h, &Ctx{Source: src.ID, Controller: 0}, sa(t, line))

			for _, ce := range h.continuous {
				if ce.Source == src.ID {
					t.Fatalf("%s: the source's continuous effect survived the one-shot ender", tc.name)
				}
			}
			if len(h.continuous) != 1 || h.continuous[0].Source != 9999 {
				t.Fatalf("%s: an unrelated source's effect was dropped: %+v", tc.name, h.continuous)
			}
			// No card move was attempted: a real card never sits in Command
			// under this shape, and the ender must not emit a zone move.
			if got := h.g.Obj(src.ID).Zone; got == state.ZExile {
				t.Fatalf("%s: the effect frame was exiled as a card", tc.name)
			}
		})
	}
}

// TestChangeZoneCommandSelfExileKeepsARealCommandCard pins the guard's other
// side: a card that genuinely IS in the Command zone under the same shape (a
// companion/emblem-frame card, or an ST$ PayUp special action) keeps its
// ordinary move and its registered effects -- the structural ender only fires
// when the source is NOT in Command.
func TestChangeZoneCommandSelfExileKeepsARealCommandCard(t *testing.T) {
	h := newHost(t, 2)
	src := h.g.AddObject(mkCard(t, "Name:Commander\nTypes:Legendary Creature\nPT:2/2\nOracle:x\n"), 0)
	src.Zone = state.ZCommand
	h.AddContinuous(state.ContinuousEffect{Source: src.ID, Controller: 0, Affects: "Card.Self"})

	Resolve(h, &Ctx{Source: src.ID, Controller: 0}, sa(t, "DB$ ChangeZone | Origin$ Command | Destination$ Exile"))

	if len(h.continuous) != 1 {
		t.Fatalf("a real command-zone card lost its effect: %+v", h.continuous)
	}
	if got := h.g.Obj(src.ID).Zone; got != state.ZExile {
		t.Fatalf("command-zone card zone = %s, want %s (ordinary move)", got, state.ZExile)
	}
}

// TestChangeZoneCommandSelfExileLeavesImprinted pins the one spelling the
// structural guard deliberately excludes: Defined$ Imprinted names real exiled
// cards, not the effect frame. The only way the exclusion is reachable is the
// degenerate self-imprint (the source's own Imprinted list holds the source),
// so that is the fixture -- without the exclusion the resolved target WOULD be
// the source and the frame would be ended wrongly.
func TestChangeZoneCommandSelfExileLeavesImprinted(t *testing.T) {
	h := newHost(t, 2)
	src := h.g.AddObject(mkCard(t, "Name:Frame\nTypes:Sorcery\nOracle:x\n"), 0)
	h.AddContinuous(state.ContinuousEffect{Source: src.ID, Controller: 0, Affects: "Card.Self"})
	src.Imprinted = []state.ObjID{src.ID}

	Resolve(h, &Ctx{Source: src.ID, Controller: 0}, sa(t, "DB$ ChangeZone | Defined$ Imprinted | Origin$ Command | Destination$ Exile"))

	if len(h.continuous) != 1 {
		t.Fatalf("Defined$ Imprinted must not end the effect frame: %+v", h.continuous)
	}
}
