package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

func threeWordsFilter(t *testing.T, expr string) (lClause, error) {
	t.Helper()
	f, err := parseFilter(expr, func(w string) bool { return w == "Spider" || w == "Hero" || w == "Ninja" }, func(string) bool { return false })
	if err != nil {
		return lClause{}, err
	}
	return f.Clauses[0], nil
}

// TestFilterWordNotChosenCard: the bang spelling of the looked-at-but-not-
// chosen filter compiles to the very clause nonChosenCard does.
func TestFilterWordNotChosenCard(t *testing.T) {
	a, err := threeWordsFilter(t, "Card.IsRemembered+!ChosenCard")
	if err != nil {
		t.Fatal(err)
	}
	b, err := threeWordsFilter(t, "Card.IsRemembered+nonChosenCard")
	if err != nil {
		t.Fatal(err)
	}
	if a.Flags&fNotChosenCard == 0 || a.Flags&fRemembered == 0 {
		t.Fatalf("flags = %#x, want the remembered and not-chosen bits", a.Flags)
	}
	if a.String() != b.String() || a.Flags != b.Flags {
		t.Fatalf("!ChosenCard clause %s differs from nonChosenCard %s", a, b)
	}
}

// TestFilterTwoSubtypes: a clause naming two subtypes lowers to Sub plus the
// kernel's PredSubtype; a third refuses, and so does a second subtype beside
// a compare (the clause has one Pred slot).
func TestFilterTwoSubtypes(t *testing.T) {
	cb := &cardBuilder{pt: &poolTables{subIdx: map[string]int{"Spider": 3, "Hero": 5}, missing: map[string]bool{}}}
	c, err := threeWordsFilter(t, "Spider.Hero")
	if err != nil {
		t.Fatal(err)
	}
	if c.Subtype != "Spider" || c.Subtype2 != "Hero" {
		t.Fatalf("subtypes = %q %q", c.Subtype, c.Subtype2)
	}
	for _, zone := range []kop.Zone{kop.ZoneLibrary, kop.ZoneBattlefield} {
		kf, err := cb.clause(c, zone)
		if err != nil {
			t.Fatal(err)
		}
		if kf.Sub != 4 || kf.Pred != kop.PredSubtype || kf.PredN != 5 {
			t.Errorf("zone %v: Sub %d Pred %d PredN %d, want Sub 4 + PredSubtype 5", zone, kf.Sub, kf.Pred, kf.PredN)
		}
	}
	if _, err := threeWordsFilter(t, "Spider.Hero+Ninja"); err == nil || !strings.Contains(err.Error(), "three subtypes") {
		t.Fatalf("three subtypes: got %v", err)
	}
	c, err = threeWordsFilter(t, "Spider.Hero+cmcLE3")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cb.clause(c, kop.ZoneLibrary); err == nil || !strings.Contains(err.Error(), "two subtypes beside a compare") {
		t.Fatalf("two subtypes beside a compare: got %v", err)
	}
}

// effectSourceTriggerForms: Lyra's until-end-of-turn Effect lowers as an
// emblem that remembers its own source; Zimone's Experiment and Radioactive
// Spider lower through the other two words.
var effectSourceTriggerForms = []formCase{
	{card: "Lyra, Tolarian Archangel", form: "the Effect trigger's EffectSource source is the emblem's captured Self", lir: "until=EOT remembers=Self"},
	{card: "Lyra, Tolarian Archangel", form: "the source filter is the type check only", lir: "trigger DamageDone F={types=0x0} F2={players=3}"},
}

func TestEffectSourceTrigger(t *testing.T) {
	checkForms(t, effectSourceTriggerForms)
	// any other EffectSource key or mode in an Effect trigger still refuses
	for _, tc := range []struct {
		mode, key, val string
	}{
		{"DamageDone", "ValidTarget", "Card.EffectSource"},
		{"DamageDone", "ValidSource", "Creature.EffectSource"},
		{"ChangesZone", "ValidCard", "Card.EffectSource"},
		{"DamageDone", "IsPresent", "Card.EffectSource"},
	} {
		tr := cards.Trigger{Mode: tc.mode, Params: map[string]string{tc.key: tc.val}}
		if _, _, err := effectSourceTrigger(newParams(map[string]string{}), tr); err == nil {
			t.Errorf("%s %s$ %s: want a refusal", tc.mode, tc.key, tc.val)
		}
	}
}
