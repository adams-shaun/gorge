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
// (Object.ManaSnowSpent, CR 107.4h); the typed Treasure/Cave/Desert forms
// resolve from their own captures (castfilter2, tested below), and an
// unknown producer type fails closed to 0 rather than returning the
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
	// A cast whose capture carries no typed units reads a real zero for that
	// tag, never the unfiltered total (castfilter2 gives the typed tags their
	// own fields; an object with no capture reads 0 in every form).
	if got := EvalCount(h, c, "Count$CastTotalManaSpent Desert"); got != 0 {
		t.Errorf("CastTotalManaSpent Desert = %d, want 0 (no Desert units captured)", got)
	}
	if got := EvalCount(h, c, "Count$CastTotalManaSpent Treasure"); got != 0 {
		t.Errorf("CastTotalManaSpent Treasure = %d, want 0 (no Treasure units captured)", got)
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

// The TYPED Count$CastTotalManaSpent <Type> forms (task castfilter2):
// Treasure/Cave/Desert resolve from the per-tag pay-time captures
// (Object.ManaTreasureSpent / ManaCaveSpent / ManaDesertSpent, the
// ManaSnowSpent pattern); an unknown producer type still fails closed to 0
// rather than returning the unfiltered total.
func TestCastTotalManaSpentHeadReadsTypedFields(t *testing.T) {
	h, c := fixtureHost(t)
	o := h.g.Obj(c.Source)
	// Eight mana spent: two Treasure-sourced, one Cave-sourced, the rest
	// plain (one of them snow, which reads its own field).
	o.ManaSpent = 8
	o.ManaSnowSpent = 1
	o.ManaTreasureSpent = 2
	o.ManaCaveSpent = 1
	if got := EvalCount(h, c, "Count$CastTotalManaSpent Treasure"); got != 2 {
		t.Errorf("CastTotalManaSpent Treasure = %d, want 2", got)
	}
	if got := EvalCount(h, c, "Count$CastTotalManaSpent Cave"); got != 1 {
		t.Errorf("CastTotalManaSpent Cave = %d, want 1", got)
	}
	if got := EvalCount(h, c, "Count$CastTotalManaSpent Desert"); got != 0 {
		t.Errorf("CastTotalManaSpent Desert = %d, want 0 (no Desert units spent)", got)
	}
	// A tag with no capture at all is a real zero, not the total.
	o.ManaTreasureSpent = 0
	if got := EvalCount(h, c, "Count$CastTotalManaSpent Treasure"); got != 0 {
		t.Errorf("no-Treasure CastTotalManaSpent Treasure = %d, want 0", got)
	}
	// A producer type no tagging models fails closed to 0, never the total.
	if got := EvalCount(h, c, "Count$CastTotalManaSpent Clue"); got != 0 {
		t.Errorf("unknown-type CastTotalManaSpent Clue = %d, want 0 (fail closed)", got)
	}
	// A source object that is gone: 0, every form.
	c2 := &Ctx{Source: 999, Controller: 0}
	for _, arg := range []string{"", " Snow", " Treasure", " Cave", " Desert"} {
		if got := EvalCount(h, c2, "Count$CastTotalManaSpent"+arg); got != 0 {
			t.Errorf("absent-source CastTotalManaSpent%s = %d, want 0", arg, got)
		}
	}
}

// The typed heads read the REAL corpus SVars, not just hand-built strings:
// Marut's X, Bat Colony's X and Cataclysmic Prospecting's Y are exactly the
// filtered forms the end-to-end rules pins drive.
func TestCastTotalManaSpentTypedReadTheRealCorpusSVars(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		card, svar, body, tag string
		want                  int32
	}{
		{"Marut", "X", "Count$CastTotalManaSpent Treasure", "Treasure", 4},
		{"Bat Colony", "X", "Count$CastTotalManaSpent Cave", "Cave", 5},
		{"Cataclysmic Prospecting", "Y", "Count$CastTotalManaSpent Desert", "Desert", 3},
	} {
		card, ok := reg.Lookup(tc.card)
		if !ok {
			t.Fatalf("corpus missing %s", tc.card)
		}
		if body := card.Faces[0].SVars[tc.svar]; body != tc.body {
			t.Fatalf("%s SVar %s = %q, want %q", tc.card, tc.svar, body, tc.body)
		}
		h, c := fixtureHost(t)
		h.g.Obj(c.Source).ManaSpent = 6
		switch tc.tag {
		case "Treasure":
			h.g.Obj(c.Source).ManaTreasureSpent = 4
		case "Cave":
			h.g.Obj(c.Source).ManaCaveSpent = 5
		case "Desert":
			h.g.Obj(c.Source).ManaDesertSpent = 3
		}
		if got := EvalCount(h, c, tc.body); got != tc.want {
			t.Errorf("real %s SVar = %d, want %d", tc.card, got, tc.want)
		}
	}
}
