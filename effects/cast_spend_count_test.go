package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
)

// The Count$CastTotalManaSpent branch head (task castprov1): the TOTAL mana
// actually spent to cast the resolving spell, carried by the pay-time
// CastInfo's FlagManaSpent Amount (rules/cast.go's payCast capture -- the
// converge/replicate/multikick pattern). Unit-tested at the eval level the
// Count$Converge / Count$wasCastFromYourHandByYou heads use; the provenance
// itself is pinned end to end on the real engine in rules (the Freestrider
// Commando corpus tests).

func TestCastTotalManaSpentHeadReadsTheCapturedSpend(t *testing.T) {
	h, c := fixtureHost(t)
	// A cheated-in permanent (no CastInfo ever stamped): 0.
	if got := EvalCount(h, c, "Count$CastTotalManaSpent"); got != 0 {
		t.Errorf("uncast CastTotalManaSpent = %d, want 0", got)
	}
	// The pay-time capture: four mana spent to cast the source.
	h.g.Obj(c.Source).ManaSpent = 4
	if got := EvalCount(h, c, "Count$CastTotalManaSpent"); got != 4 {
		t.Errorf("cast CastTotalManaSpent = %d, want 4", got)
	}
	// A zero is a real zero (a convoke-only cast), not an absent one.
	h.g.Obj(c.Source).ManaSpent = 0
	h.g.Obj(c.Source).CastFlags = 1 << 20 // some provenance, not the absence of a cast
	if got := EvalCount(h, c, "Count$CastTotalManaSpent"); got != 0 {
		t.Errorf("zero-spend CastTotalManaSpent = %d, want 0", got)
	}
	// A source object that is gone: 0.
	c2 := &Ctx{Source: 999, Controller: 0}
	if got := EvalCount(h, c2, "Count$CastTotalManaSpent"); got != 0 {
		t.Errorf("absent-source CastTotalManaSpent = %d, want 0", got)
	}
}

// The FILTERED Count$CastTotalManaSpent <Type> form (task castfilter1).
// Snow resolves from the parallel snow tally the pool has always carried
// (Object.ManaSnowSpent, CR 107.4h); every other producer type lacks the
// per-unit provenance and fails closed to 0 rather than returning the
// unfiltered total. Pinned on the real corpus card the Snow family uses
// (Berg Strider's SVar:S:Count$CastTotalManaSpent Snow).
func TestCastTotalManaSpentHeadFiltersByType(t *testing.T) {
	h, c := fixtureHost(t)
	o := h.g.Obj(c.Source)
	// Six mana spent, two of them snow units (a Snow-Covered Forest tapped
	// alongside generic sources).
	o.ManaSpent = 6
	o.ManaSnowSpent = 2
	if got := EvalCount(h, c, "Count$CastTotalManaSpent"); got != 6 {
		t.Errorf("unfiltered CastTotalManaSpent = %d, want 6", got)
	}
	if got := EvalCount(h, c, "Count$CastTotalManaSpent Snow"); got != 2 {
		t.Errorf("CastTotalManaSpent Snow = %d, want 2", got)
	}
	// The ticket's named carrier family: a producer type with no per-unit
	// provenance fails closed to 0, never the unfiltered total.
	if got := EvalCount(h, c, "Count$CastTotalManaSpent Desert"); got != 0 {
		t.Errorf("CastTotalManaSpent Desert = %d, want 0 (no producer-type provenance)", got)
	}
	if got := EvalCount(h, c, "Count$CastTotalManaSpent Treasure"); got != 0 {
		t.Errorf("CastTotalManaSpent Treasure = %d, want 0 (no producer-type provenance)", got)
	}
	// A cast that spent no snow mana is a real zero, not the total.
	o.ManaSnowSpent = 0
	if got := EvalCount(h, c, "Count$CastTotalManaSpent Snow"); got != 0 {
		t.Errorf("no-snow CastTotalManaSpent Snow = %d, want 0", got)
	}
	// A cheated-in permanent never carried either field: both read 0.
	o.ManaSpent = 0
	if got := EvalCount(h, c, "Count$CastTotalManaSpent Snow"); got != 0 {
		t.Errorf("uncast CastTotalManaSpent Snow = %d, want 0", got)
	}
}

// The eval-level test above is synthetic; this one reads the REAL corpus SA
// (Berg Strider's SVar:S) to prove the filtered form survives the compiled
// script path, not just a hand-built string.
func TestCastTotalManaSpentSnowReadsTheRealCorpusSVar(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Berg Strider")
	if !ok {
		t.Fatal("corpus missing Berg Strider")
	}
	body := card.Faces[0].SVars["S"]
	if body != "Count$CastTotalManaSpent Snow" {
		t.Fatalf("Berg Strider SVar S = %q, want the filtered Snow form", body)
	}
	h, c := fixtureHost(t)
	h.g.Obj(c.Source).ManaSpent = 5
	h.g.Obj(c.Source).ManaSnowSpent = 3
	if got := EvalCount(h, c, body); got != 3 {
		t.Errorf("real Berg Strider SVar = %d, want 3", got)
	}
}
