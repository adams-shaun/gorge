package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// The TriggeredCard$CastTotalManaSpent ref-head (task ctms-refhead): the TOTAL
// mana actually spent to cast the card a firing trigger bound, read off the
// same recorded per-cast totals the plain Count$CastTotalManaSpent head reads
// (Object.ManaSpent / ManaSnowSpent / the typed captures). The row's carrier
// population is the 21 corpus SVars of that exact spelling (Aberrant Manawurm,
// Manaform Hellkite, Muse Seeker, Aetherflux Conduit, ...); before this fix
// evalRefProperty had no arm for the property, so every one of them saw 0.

// TestTriggeredCardCastTotalManaSpentReadsTheSnapshot pins the TRIGGER-time
// binding: the spend was paid when the spell was cast, so a spell that has
// left the stack before the trigger resolves (countered, or resolved onto the
// battlefield) must still report its real total even though the live capture
// has been cleared. The snapshot wins for the triggering card.
func TestTriggeredCardCastTotalManaSpentReadsTheSnapshot(t *testing.T) {
	h, c := fixtureHost(t)
	spell := h.g.AddObject(mkCard(t, "Name:Triggering Spell\nTypes:Instant\nOracle:x\n"), 1)
	// Precondition: the spell's LIVE captures really are cleared -- the Move
	// out of the stack zeroes them -- so the snapshot and the live field
	// genuinely differ and this test cannot pass by reading the live field.
	if spell.ManaSpent != 0 {
		t.Fatalf("precondition: live ManaSpent = %d, want 0 (cleared on leaving the stack)", spell.ManaSpent)
	}
	c.TriggerCard = spell.ID
	c.Remembered = []state.Target{{Obj: spell.ID}}
	c.TriggerManaSpent = 5

	if got := EvalCount(h, c, "TriggeredCard$CastTotalManaSpent"); got != 5 {
		t.Errorf("TriggeredCard$CastTotalManaSpent = %d, want 5 (the fire-time snapshot)", got)
	}
	// The filtered forms select from the same snapshot: snow 2 of the 5, and a
	// modelled producer tag from the typed array (state.TypedManaTags order:
	// Treasure, Cave, Desert).
	c.TriggerManaSnowSpent = 2
	c.TriggerManaTyped = [4]int32{3, 0, 1}
	if got := EvalCount(h, c, "TriggeredCard$CastTotalManaSpent Snow"); got != 2 {
		t.Errorf("TriggeredCard$CastTotalManaSpent Snow = %d, want 2", got)
	}
	if got := EvalCount(h, c, "TriggeredCard$CastTotalManaSpent Treasure"); got != 3 {
		t.Errorf("TriggeredCard$CastTotalManaSpent Treasure = %d, want 3", got)
	}
	if got := EvalCount(h, c, "TriggeredCard$CastTotalManaSpent Desert"); got != 1 {
		t.Errorf("TriggeredCard$CastTotalManaSpent Desert = %d, want 1", got)
	}
	// A tag with no captured producer units is a real zero, not the total, and
	// an unknown producer type fails closed -- exactly the plain head's rule.
	if got := EvalCount(h, c, "TriggeredCard$CastTotalManaSpent Cave"); got != 0 {
		t.Errorf("TriggeredCard$CastTotalManaSpent Cave = %d, want 0 (no Cave units)", got)
	}
	if got := EvalCount(h, c, "TriggeredCard$CastTotalManaSpent Clue"); got != 0 {
		t.Errorf("TriggeredCard$CastTotalManaSpent Clue = %d, want 0 (unknown type fails closed)", got)
	}
}

// TestTriggeredCardCastTotalManaSpentFallsBackToTheLiveObject pins the other
// half of the ref: a referenced object that is NOT the triggering card (a
// Remembered referent) reads its own live captured spend, the same read the
// plain head makes, so the two can never disagree about one object.
func TestTriggeredCardCastTotalManaSpentFallsBackToTheLiveObject(t *testing.T) {
	h, c := fixtureHost(t)
	trigger := h.g.AddObject(mkCard(t, "Name:Triggering Card\nTypes:Sorcery\nOracle:x\n"), 1)
	other := h.g.AddObject(mkCard(t, "Name:Remembered Spell\nTypes:Instant\nOracle:x\n"), 1)
	// Precondition: the two objects carry DIFFERENT spends, so the assertion
	// distinguishes the snapshot from the live field rather than passing on
	// whichever one the code happens to read.
	other.ManaSpent = 7
	c.TriggerCard = trigger.ID
	c.TriggerManaSpent = 5
	c.Remembered = []state.Target{{Obj: other.ID}}

	if got := EvalCount(h, c, "TriggeredCard$CastTotalManaSpent"); got != 7 {
		t.Errorf("live referent TriggeredCard$CastTotalManaSpent = %d, want 7 (its own capture)", got)
	}
}

// TestTriggeredCardCastTotalManaSpentReadsTheRealCorpusSVar proves the real
// compiled script path reaches the head: Aberrant Manawurm's SVar:X is exactly
// this spelling, and the same evaluation a trigger resolution makes returns
// the real total instead of the pre-fix zero.
func TestTriggeredCardCastTotalManaSpentReadsTheRealCorpusSVar(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Aberrant Manawurm")
	if !ok {
		t.Fatal("corpus missing Aberrant Manawurm")
	}
	body := card.Faces[0].SVars["X"]
	if body != "TriggeredCard$CastTotalManaSpent" {
		t.Fatalf("Aberrant Manawurm SVar X = %q, want the trigger-relative total form", body)
	}
	h, c := fixtureHost(t)
	spell := h.g.AddObject(mkCard(t, "Name:Cast Spell\nTypes:Instant\nOracle:x\n"), 1)
	c.TriggerCard = spell.ID
	c.Remembered = []state.Target{{Obj: spell.ID}}
	c.TriggerManaSpent = 4
	if got := EvalCount(h, c, body); got != 4 {
		t.Errorf("real Aberrant Manawurm SVar = %d, want 4", got)
	}
}
