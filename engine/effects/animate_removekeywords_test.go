package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// The api:Animate.RemoveKeywords$ read (the graveyard-enchant ticket): a
// `DB$ Animate` body's RemoveKeywords$ reaches state.ContinuousEffect
// .RemoveKeywords beside the Keywords$ grant, so rules' layer-6 walk strips the
// named entries BEFORE this same effect's AddKeywords apply (Animate Dead's
// "it loses 'enchant creature card in a graveyard'" in the same pass that
// grants "enchant creature put onto the battlefield with CARDNAME").
// AnimateAll's RemoveKeywords$/RemoveAllAbilities$ are READ since cr-g134:
// the sweep carries them per object through registerAnimateEffects, so the
// registered unit-level shape is pinned here beside the single-target read.

func animateRemoveHost(t *testing.T) (*fakeHost, *Ctx) {
	t.Helper()
	h := newHost(t, 2)
	card := mkCard(t, "Name:Vanilla\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	o := h.g.AddObject(card, 0)
	h.g.Obj(o.ID).Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), o.ID))
	return h, &Ctx{Source: o.ID, Controller: 0}
}

// TestAnimateRemoveKeywordsRegistersLayer6Removal pins the read on the card's
// exact DB$ Animate shape: exactly one layer-6 effect, carrying BOTH lists.
func TestAnimateRemoveKeywordsRegistersLayer6Removal(t *testing.T) {
	h, c := animateRemoveHost(t)
	Resolve(h, c, sa(t, "DB$ Animate | Defined$ Self | Keywords$ Enchant:Creature.IsRemembered:granted enchant | "+
		"RemoveKeywords$ Enchant:Creature.inZoneGraveyard:stale graveyard enchant | Duration$ Permanent"))
	if len(h.continuous) != 1 {
		t.Fatalf("continuous = %d effects, want exactly the layer-6 grant: %+v", len(h.continuous), h.continuous)
	}
	ce := h.continuous[0]
	if ce.Layer != state.LAbilities {
		t.Fatalf("layer %v, want LAbilities: %+v", ce.Layer, ce)
	}
	if len(ce.AddKeywords) != 1 || ce.AddKeywords[0] != "Enchant:Creature.IsRemembered:granted enchant" {
		t.Fatalf("AddKeywords = %v, want the Keywords$ grant", ce.AddKeywords)
	}
	if len(ce.RemoveKeywords) != 1 || ce.RemoveKeywords[0] != "Enchant:Creature.inZoneGraveyard:stale graveyard enchant" {
		t.Fatalf("RemoveKeywords = %v, want the RemoveKeywords$ read", ce.RemoveKeywords)
	}
}

// TestAnimateRemoveKeywordsOnlyBodyStillRegisters pins the Takklemaggot shape
// (`RemoveKeywords$ Enchant:Creature`, no Keywords$): the removal alone must
// still register the layer-6 effect, not be swallowed by the old
// keywords-only gate.
func TestAnimateRemoveKeywordsOnlyBodyStillRegisters(t *testing.T) {
	h, c := animateRemoveHost(t)
	Resolve(h, c, sa(t, "DB$ Animate | Defined$ Self | RemoveKeywords$ Enchant:Creature.inZoneGraveyard:stale | Duration$ Permanent"))
	if len(h.continuous) != 1 {
		t.Fatalf("continuous = %+v, want one layer-6 effect for the removal-only body", h.continuous)
	}
	if got := h.continuous[0].RemoveKeywords; len(got) != 1 || got[0] != "Enchant:Creature.inZoneGraveyard:stale" {
		t.Fatalf("RemoveKeywords = %v, want the read entry", got)
	}
	if got := h.continuous[0].AddKeywords; len(got) != 0 {
		t.Fatalf("AddKeywords = %v, want none", got)
	}
}

// TestAnimateAllRemoveKeywordsRegistersPerObject pins the sweep's read of
// RemoveKeywords$: every creature the ValidCards$ filter matches gets its OWN
// LAbilities effect carrying the removal (the same registration the
// single-target Animate uses), and no Note names RemoveKeywords$ -- it is a
// read parameter now, not an unread gap.
func TestAnimateAllRemoveKeywordsRegistersPerObject(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	Resolve(h, &Ctx{Controller: 0},
		sa(t, "SP$ AnimateAll | ValidCards$ Creature | RemoveKeywords$ Indestructible"))
	want := map[state.ObjID]bool{ids["myBear"]: true, ids["myFlier"]: true, ids["theirBig"]: true}
	seen := map[state.ObjID]bool{}
	for _, ce := range h.continuous {
		if !want[ce.Source] {
			t.Fatalf("grant landed on a non-creature (source %d): %+v", ce.Source, ce)
		}
		if seen[ce.Source] {
			t.Fatalf("two continuous effects for creature %d: %+v", ce.Source, ce)
		}
		seen[ce.Source] = true
		if ce.Layer != state.LAbilities {
			t.Fatalf("layer %v, want LAbilities: %+v", ce.Layer, ce)
		}
		if len(ce.RemoveKeywords) != 1 || ce.RemoveKeywords[0] != "Indestructible" {
			t.Fatalf("RemoveKeywords = %v, want [Indestructible]: %+v", ce.RemoveKeywords, ce)
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("creatures swept = %v, want all of %v", seen, want)
	}
	for _, n := range notesOf(h) {
		if strings.Contains(n, "RemoveKeywords$") {
			t.Fatalf("RemoveKeywords$ is read: no note may name it, got %q", n)
		}
	}
}

// TestAnimateAllRemoveAllAbilitiesRegistersPerObject pins the Curious
// Colossus body on the sweep: RemoveAllAbilities$ True plus the base 1/1 and
// the Coward type grant register per swept creature as the THREE effects the
// shared registration makes (layer-6 removal, layer-4 type add, layer-7b base
// P/T), and no Note names RemoveAllAbilities$.
func TestAnimateAllRemoveAllAbilitiesRegistersPerObject(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	Resolve(h, &Ctx{Controller: 0},
		sa(t, "DB$ AnimateAll | ValidCards$ Creature | Power$ 1 | Toughness$ 1 | "+
			"RemoveAllAbilities$ True | Types$ Coward | Duration$ Permanent"))
	want := map[state.ObjID]bool{ids["myBear"]: true, ids["myFlier"]: true, ids["theirBig"]: true}
	saw := map[state.ObjID]map[string]bool{}
	for _, ce := range h.continuous {
		if !want[ce.Source] {
			t.Fatalf("grant landed on a non-creature (source %d): %+v", ce.Source, ce)
		}
		kind := ""
		switch {
		case ce.Layer == state.LAbilities:
			if !ce.RemoveAbilities {
				t.Fatalf("LAbilities effect without RemoveAbilities: %+v", ce)
			}
			kind = "abilities"
		case ce.Layer == state.LType:
			if len(ce.AddTypes) != 1 || ce.AddTypes[0] != "Coward" {
				t.Fatalf("type effect = %+v, want the Coward add", ce)
			}
			kind = "types"
		case ce.Layer == state.LPT:
			if ce.Sub != state.SubSet || !ce.HasSet || ce.SetPower != 1 || ce.SetToughness != 1 {
				t.Fatalf("P/T effect = %+v, want the 1/1 base set", ce)
			}
			kind = "pt"
		default:
			t.Fatalf("unexpected layer %d: %+v", ce.Layer, ce)
		}
		if !ce.Permanent || ce.UntilEOT {
			t.Fatalf("Duration$ Permanent registered %+v", ce)
		}
		if saw[ce.Source] == nil {
			saw[ce.Source] = map[string]bool{}
		}
		saw[ce.Source][kind] = true
	}
	for id := range want {
		for _, kind := range []string{"abilities", "types", "pt"} {
			if !saw[id][kind] {
				t.Fatalf("creature %d missing its %s effect: %+v", id, kind, h.continuous)
			}
		}
	}
	for _, n := range notesOf(h) {
		if strings.Contains(n, "RemoveAllAbilities$") {
			t.Fatalf("RemoveAllAbilities$ is read: no note may name it, got %q", n)
		}
	}
}
