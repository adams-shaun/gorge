package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The api:Animate.RemoveKeywords$ read (the graveyard-enchant ticket): a
// `DB$ Animate` body's RemoveKeywords$ reaches state.ContinuousEffect
// .RemoveKeywords beside the Keywords$ grant, so rules' layer-6 walk strips the
// named entries BEFORE this same effect's AddKeywords apply (Animate Dead's
// "it loses 'enchant creature card in a graveyard'" in the same pass that
// grants "enchant creature put onto the battlefield with CARDNAME").
// AnimateAll's RemoveKeywords$ stays unread (the animateAllUnreadNote
// convention) and is pinned unchanged.

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

// TestAnimateAllKeepsRemoveKeywordsUnread pins the scope boundary: the shared
// parser reads the parameter but AnimateAll's sweep must not apply it -- the
// unread note names it, and no registered effect carries a removal.
func TestAnimateAllKeepsRemoveKeywordsUnread(t *testing.T) {
	h, c := animateRemoveHost(t)
	Resolve(h, c, sa(t, "SP$ AnimateAll | ValidCards$ Creature | RemoveKeywords$ Enchant:Creature.inZoneGraveyard:stale"))
	noted := false
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "RemoveKeywords$") && strings.Contains(ev.Text, "not implemented") {
			noted = true
		}
	}
	if !noted {
		t.Fatalf("no AnimateAll RemoveKeywords$ unread note in the log: %+v", h.log)
	}
	for _, ce := range h.continuous {
		if len(ce.RemoveKeywords) > 0 {
			t.Fatalf("AnimateAll applied RemoveKeywords$ behind the unread note: %+v", ce)
		}
	}
}
