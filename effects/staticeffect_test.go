package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// StaticEffect$ <name> (the ChangeZone "return it as a ..." rider) pinned at
// the registration unit: the layers it builds, the Affected$ IsRemembered
// pinning, the check gate, and the fail-loud surfaces. The engine-level
// carrier (Otherworldly Escort) is pinned in rules/otherworldly_escort_test.go.

func staticEffectBoard(t *testing.T) (*fakeHost, *Ctx, state.ObjID) {
	t.Helper()
	h := newHost(t, 2)
	card := mkCard(t, "Name:Fixture Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	o := h.g.AddObject(card, 0)
	h.g.Obj(o.ID).Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), o.ID))
	return h, &Ctx{Source: o.ID, Controller: 0}, o.ID
}

func staticEffectSA(t *testing.T, line string, svars map[string]string) (*cards.SA, *Ctx) {
	t.Helper()
	sa := sa(t, "DB$ ChangeZone | Origin$ Graveyard | Destination$ Battlefield | "+line)
	return sa, &Ctx{Source: 1, Controller: 0, SVars: svars}
}

func staticEffectNotes(h *fakeHost) []string {
	var out []string
	for _, ev := range h.log {
		if ev.Kind == events.Note {
			out = append(out, ev.Text)
		}
	}
	return out
}

func TestStaticEffectRegistersNamedContinuous(t *testing.T) {
	h, _, id := staticEffectBoard(t)
	sa, ctx := staticEffectSA(t, "Defined$ Self | StaticEffect$ Animate", map[string]string{
		"Animate": "Mode$ Continuous | Affected$ Card.IsRemembered | AddType$ Spirit & Detective | RemoveCreatureTypes$ True",
	})
	ctx.Source = id
	applyStaticEffect(h, ctx, sa, state.ZBattlefield, []state.ObjID{id})

	if len(h.continuous) != 1 {
		t.Fatalf("registered %d effects, want 1: %+v", len(h.continuous), h.continuous)
	}
	ce := h.continuous[0]
	if ce.Layer != state.LType || ce.Source != id || ce.Affects != "Card.Self" || ce.Controller != 0 {
		t.Fatalf("type effect mis-scoped: %+v", ce)
	}
	if len(ce.AddTypes) != 2 || ce.AddTypes[0] != "Spirit" || ce.AddTypes[1] != "Detective" {
		t.Fatalf("AddTypes = %v, want [Spirit Detective]", ce.AddTypes)
	}
	if !ce.RemoveCreatureTypes || ce.RemoveCardTypes {
		t.Fatalf("strips wrong: %+v", ce)
	}
	if ns := staticEffectNotes(h); len(ns) != 0 {
		t.Fatalf("fully readable body emitted notes %v", ns)
	}
	// A non-battlefield destination and an empty moved set are silent no-ops.
	applyStaticEffect(h, ctx, sa, state.ZGraveyard, []state.ObjID{id})
	applyStaticEffect(h, ctx, sa, state.ZBattlefield, nil)
	if len(h.continuous) != 1 {
		t.Fatalf("no-op calls registered: %d effects", len(h.continuous))
	}
}

func TestStaticEffectBodyProblemsAreLoud(t *testing.T) {
	t.Run("missing body", func(t *testing.T) {
		h, _, id := staticEffectBoard(t)
		sa, ctx := staticEffectSA(t, "Defined$ Self | StaticEffect$ Ghost", nil)
		ctx.Source = id
		applyStaticEffect(h, ctx, sa, state.ZBattlefield, []state.ObjID{id})
		if len(h.continuous) != 0 {
			t.Fatalf("registered %+v, want none", h.continuous)
		}
		if ns := staticEffectNotes(h); len(ns) != 1 || !strings.Contains(ns[0], "names no SVar body") {
			t.Fatalf("notes = %v, want the missing-body note", ns)
		}
	})
	t.Run("non-Continuous body", func(t *testing.T) {
		h, _, id := staticEffectBoard(t)
		sa, ctx := staticEffectSA(t, "Defined$ Self | StaticEffect$ Weird", map[string]string{
			"Weird": "Mode$ Always | Affected$ Card.Self | AddType$ Zombie",
		})
		ctx.Source = id
		applyStaticEffect(h, ctx, sa, state.ZBattlefield, []state.ObjID{id})
		if len(h.continuous) != 0 {
			t.Fatalf("registered %+v, want none", h.continuous)
		}
		if ns := staticEffectNotes(h); len(ns) != 1 || !strings.Contains(ns[0], "not a Mode$ Continuous static") {
			t.Fatalf("notes = %v, want the non-Continuous note", ns)
		}
	})
	t.Run("unread remainder named once, readable layers still register", func(t *testing.T) {
		h, _, id := staticEffectBoard(t)
		sa, ctx := staticEffectSA(t, "Defined$ Self | StaticEffect$ Animate", map[string]string{
			"Animate": "Mode$ Continuous | Affected$ Card.IsRemembered | AddType$ Land | RemoveCardTypes$ True | SetName$ Moon | AddTrigger$ DealsTrig",
		})
		ctx.Source = id
		applyStaticEffect(h, ctx, sa, state.ZBattlefield, []state.ObjID{id})
		if len(h.continuous) != 1 || h.continuous[0].Layer != state.LType ||
			!h.continuous[0].RemoveCardTypes || len(h.continuous[0].AddTypes) != 1 {
			t.Fatalf("readable layer lost: %+v", h.continuous)
		}
		if ns := staticEffectNotes(h); len(ns) != 1 || !strings.Contains(ns[0], "SetName$") ||
			!strings.Contains(ns[0], "AddTrigger$") {
			t.Fatalf("notes = %v, want ONE note naming both unread params", ns)
		}
		// The unread remainder is named per APPLICATION, not per moved card.
		applyStaticEffect(h, ctx, sa, state.ZBattlefield, []state.ObjID{id})
		if ns := staticEffectNotes(h); len(ns) != 2 {
			t.Fatalf("second application notes = %d (%v), want 2 (one per application)", len(ns), ns)
		}
	})
}

func TestStaticEffectLayers(t *testing.T) {
	t.Run("AddColor extends (Rise from the Grave)", func(t *testing.T) {
		h, _, id := staticEffectBoard(t)
		sa, ctx := staticEffectSA(t, "Defined$ Self | StaticEffect$ Animate", map[string]string{
			"Animate": "Mode$ Continuous | Affected$ Card.IsRemembered | AddType$ Zombie | AddColor$ Black",
		})
		ctx.Source = id
		applyStaticEffect(h, ctx, sa, state.ZBattlefield, []state.ObjID{id})
		if len(h.continuous) != 2 {
			t.Fatalf("registered %+v, want the LType + LColor pair", h.continuous)
		}
		col := h.continuous[1]
		if col.Layer != state.LColor || col.OverwriteColors || len(col.AddColors) != 1 || col.AddColors[0] != "B" {
			t.Fatalf("colour effect = %+v, want a plain add of [B]", col)
		}
	})
	t.Run("SetColor overwrites (The Master, Transcendent)", func(t *testing.T) {
		h, _, id := staticEffectBoard(t)
		sa, ctx := staticEffectSA(t, "Defined$ Self | StaticEffect$ Animate", map[string]string{
			"Animate": "Mode$ Continuous | Affected$ Card.IsRemembered | AddType$ Mutant | SetColor$ Green | SetPower$ 3 | SetToughness$ 3 | RemoveCreatureTypes$ True",
		})
		ctx.Source = id
		applyStaticEffect(h, ctx, sa, state.ZBattlefield, []state.ObjID{id})
		if len(h.continuous) != 3 {
			t.Fatalf("registered %+v, want LType + LColor + LPT", h.continuous)
		}
		col := h.continuous[1]
		if col.Layer != state.LColor || !col.OverwriteColors || len(col.AddColors) != 1 || col.AddColors[0] != "G" {
			t.Fatalf("colour effect = %+v, want an overwrite to [G]", col)
		}
		pt := h.continuous[2]
		if pt.Layer != state.LPT || pt.Sub != state.SubSet || !pt.HasSet || !pt.StaticSet ||
			pt.SetPowerExpr != "3" || !pt.SetPowerPresent || pt.SetToughnessExpr != "3" {
			t.Fatalf("P/T effect = %+v, want a 7b SubSet of 3/3", pt)
		}
	})
	t.Run("SVar P/T expression rides the table", func(t *testing.T) {
		h, _, id := staticEffectBoard(t)
		sa, ctx := staticEffectSA(t, "Defined$ Self | StaticEffect$ Animate", map[string]string{
			"Animate": "Mode$ Continuous | Affected$ Card.IsRemembered | SetPower$ X | SetToughness$ X",
			"X":       "5",
		})
		ctx.Source = id
		applyStaticEffect(h, ctx, sa, state.ZBattlefield, []state.ObjID{id})
		if len(h.continuous) != 1 || h.continuous[0].SetPowerExpr != "X" || h.continuous[0].SVars["X"] != "5" {
			t.Fatalf("P/T effect = %+v, want the expr plus the resolution's SVar table", h.continuous)
		}
	})
	t.Run("keyword and ability grants", func(t *testing.T) {
		h, _, id := staticEffectBoard(t)
		sa, ctx := staticEffectSA(t, "Defined$ Self | StaticEffect$ Animate", map[string]string{
			"Animate":     "Mode$ Continuous | Affected$ Card.IsRemembered | AddType$ Enchantment & Aura | RemoveCardTypes$ True | RemoveAllAbilities$ True | AddKeyword$ Enchant:Forest.YouCtrl | AddAbility$ TreasureSac",
			"TreasureSac": "AB$ Sacrifice | Cost$ T | ValidTgts$ Card.Self",
		})
		ctx.Source = id
		applyStaticEffect(h, ctx, sa, state.ZBattlefield, []state.ObjID{id})
		var kw, ab *state.ContinuousEffect
		for i := range h.continuous {
			ce := &h.continuous[i]
			if ce.Layer != state.LAbilities {
				continue
			}
			if len(ce.AddKeywords) > 0 {
				kw = ce
			}
			if len(ce.AddAbilities) > 0 {
				ab = ce
			}
		}
		if kw == nil || len(kw.AddKeywords) != 1 || !strings.HasPrefix(kw.AddKeywords[0], "Enchant:Forest") {
			t.Fatalf("keyword effect missing: %+v", h.continuous)
		}
		foundRemoval := false
		for i := range h.continuous {
			if h.continuous[i].RemoveAbilities {
				foundRemoval = true
			}
		}
		if !foundRemoval {
			t.Fatalf("RemoveAllAbilities not registered: %+v", h.continuous)
		}
		if ab == nil || len(ab.AddAbilities) != 1 || ab.AddAbilities[0] != "TreasureSac" {
			t.Fatalf("ability grant missing: %+v", h.continuous)
		}
	})
}

func TestStaticEffectCheckSVarGate(t *testing.T) {
	t.Run("gate not satisfied skips silently (Dance of the Manse X<6)", func(t *testing.T) {
		h, _, id := staticEffectBoard(t)
		sa := sa(t, "DB$ ChangeZone | Origin$ Graveyard | Destination$ Battlefield | StaticEffect$ Animate | StaticEffectCheckSVar$ X | StaticEffectSVarCompare$ GE6")
		ctx := &Ctx{Source: id, Controller: 0, SVars: map[string]string{"X": "3",
			"Animate": "Mode$ Continuous | Affected$ Card.IsRemembered | AddType$ Creature"}}
		applyStaticEffect(h, ctx, sa, state.ZBattlefield, []state.ObjID{id})
		if len(h.continuous) != 0 {
			t.Fatalf("registered %+v, want the failed gate to skip", h.continuous)
		}
		if ns := staticEffectNotes(h); len(ns) != 0 {
			t.Fatalf("notes = %v, want none (a resolved false gate is not a defect)", ns)
		}
	})
	t.Run("gate satisfied registers", func(t *testing.T) {
		h, _, id := staticEffectBoard(t)
		sa := sa(t, "DB$ ChangeZone | Origin$ Graveyard | Destination$ Battlefield | StaticEffect$ Animate | StaticEffectCheckSVar$ X | StaticEffectSVarCompare$ GE6")
		ctx := &Ctx{Source: id, Controller: 0, SVars: map[string]string{"X": "7",
			"Animate": "Mode$ Continuous | Affected$ Card.IsRemembered | AddType$ Creature"}}
		applyStaticEffect(h, ctx, sa, state.ZBattlefield, []state.ObjID{id})
		if len(h.continuous) != 1 || h.continuous[0].Layer != state.LType {
			t.Fatalf("registered %+v, want the gate-satisfied type grant", h.continuous)
		}
	})
}

func TestStaticEffectDurations(t *testing.T) {
	cases := []struct {
		name, dur, wantDur string
		untilEOT           bool
		permanent          bool
		wantUnreadNote     bool
	}{
		{"absent is the source-leaves default", "", "", false, false, false},
		{"Permanent survives cleanup", "Permanent", "Permanent", false, true, false},
		{"UntilEOT is a cleanup drop", "UntilEOT", "UntilEOT", true, false, false},
		{"EndOfTurn spelling", "EndOfTurn", "EndOfTurn", true, false, false},
		{"next-turn spelling keeps the boundary", "UntilYourNextTurn", "UntilYourNextTurn", false, false, false},
		{"unknown duration is loud", "UntilUntaps", "", false, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, _, id := staticEffectBoard(t)
			body := "Mode$ Continuous | Affected$ Card.IsRemembered | AddType$ Zombie"
			if tc.dur != "" {
				body += " | Duration$ " + tc.dur
			}
			sa, ctx := staticEffectSA(t, "Defined$ Self | StaticEffect$ Animate", map[string]string{"Animate": body})
			ctx.Source = id
			applyStaticEffect(h, ctx, sa, state.ZBattlefield, []state.ObjID{id})
			if len(h.continuous) != 1 {
				t.Fatalf("registered %+v, want one", h.continuous)
			}
			ce := h.continuous[0]
			if ce.UntilEOT != tc.untilEOT || ce.Permanent != tc.permanent || ce.Duration != tc.wantDur {
				t.Fatalf("lifetime fields = %+v (dur %q)", ce, tc.dur)
			}
			ns := staticEffectNotes(h)
			if tc.wantUnreadNote != (len(ns) == 1) {
				t.Fatalf("notes = %v, wantUnreadNote = %v", ns, tc.wantUnreadNote)
			}
		})
	}
}
