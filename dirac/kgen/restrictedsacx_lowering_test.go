package kgen

import (
	"strings"
	"testing"
)

// TestRestrictedSacXSourceLowers: a sole restricted "{T}, Sacrifice another
// <filter>: Add N of one fixed colour" ability (Master of Dark Rites' "{T},
// Sacrifice another creature: Add {B}{B}{B}. Spend this mana only to cast
// Vampire, Cleric, and/or Demon spells") lowers with its restriction attached:
// the kernel's tap-another search funds it as a census gap (kop.SacXRestrManaAb,
// dirac mana_tapanother.go), its mana floating restricted (CR 106.6).
func TestRestrictedSacXSourceLowers(t *testing.T) {
	lc := lowerCorpusCard(t, "Master of Dark Rites")
	for _, rf := range lc.Refusals {
		t.Errorf("Master of Dark Rites: refused: %s", rf.Reason)
	}
	var m *lMana
	for _, a := range lc.Abilities {
		if a.Kind == akMana && a.Mana != nil {
			m = a.Mana
		}
	}
	if m == nil {
		t.Fatal("Master of Dark Rites: no mana ability lowered")
	}
	if m.Colors != "B" || m.Amount.Kind != vConst || m.Amount.N != 3 || m.Each || m.Choice {
		t.Fatalf("Master of Dark Rites: mana = %q x%v each=%v choice=%v, want B x3 fixed", m.Colors, m.Amount, m.Each, m.Choice)
	}
	if m.Restrict == nil {
		t.Fatal("Master of Dark Rites: the restriction is not attached")
	}
}

// TestRestrictedSacXOtherShapesRefuse pins the neighbours the sac gap arm does
// not admit: the shape is exactly one other permanent for a const one-to-three
// of one fixed colour.
func TestRestrictedSacXOtherShapesRefuse(t *testing.T) {
	// two sacrificed permanents (Grand Architect's shape is a tap; this one is
	// the sacrifice-of-two shape the search models no arm of)
	for _, src := range []string{
		"Name:Probe\nTypes:Artifact\nA:AB$ Mana | Cost$ T Sac<2/Creature.Other> | Produced$ B | Amount$ 3 | RestrictValid$ Spell.Vampire | SpellDescription$ Add {B}{B}{B}.\n",
		"Name:Probe\nTypes:Artifact\nA:AB$ Mana | Cost$ T Sac<1/Creature.Other> | Produced$ B | Amount$ 4 | RestrictValid$ Spell.Vampire | SpellDescription$ Add {B}{B}{B}{B}.\n",
		"Name:Probe\nTypes:Artifact\nA:AB$ Mana | Cost$ T Sac<1/Creature.Other> | Produced$ Any | Amount$ 3 | RestrictValid$ Spell.Vampire | SpellDescription$ Add three mana of any one color.\n",
	} {
		got := lowerScript(t, "Probe", src)
		if len(got) != 1 || !strings.Contains(got[0], "a sacrifice of another permanent") {
			t.Errorf("probe: refusals = %v, want exactly the cost part", got)
		}
	}
}
