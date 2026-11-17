package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// assignNoCombatDamageForms covers the Effect-delivered CR 510.1 "assigns no
// combat damage" form: a triggered DB$ Effect whose StaticAbilities$ body is
//
//	Mode$ AssignNoCombatDamage | ValidCard$ Card.EffectSource
//
// with Duration$ UntilHostLeavesPlayOrEOT. effectSourceSelf rewrites the
// Card.EffectSource head to Self (the host the emblem reads as its source),
// effectEmblemDuration carries both boundaries (UntilEOT and WhileHost), and
// static_combat.go lowers the body onto kop.StatNoCombatDamage, the static
// combat.go's combatAmount zeroes. The carrier list is the ticket's
// Ophidian / Carrion Rats / Dwarven Vigilantes set; the other cards the
// ticket names still refuse on their own unrelated ability bodies (for
// example Orcish Squatters' GainControl and Goblin Vandal's Destroy), which
// this static-lowering test does not cover.
var assignNoCombatDamageForms = []formCase{
	{card: "Ophidian", form: "triggered Effect AssignNoCombatDamage over Card.EffectSource, Duration$ UntilHostLeavesPlayOrEOT", lir: "CreateEmblem obj=You"},
	{card: "Carrion Rats", form: "triggered Effect AssignNoCombatDamage over Card.EffectSource, Duration$ UntilHostLeavesPlayOrEOT", lir: "CreateEmblem obj=You"},
	{card: "Dwarven Vigilantes", form: "triggered Effect AssignNoCombatDamage over Card.EffectSource, Duration$ UntilHostLeavesPlayOrEOT", lir: "CreateEmblem obj=You"},
}

func TestAssignNoCombatDamageForms(t *testing.T) {
	// The lowering half: every carrier lowers with no refusal and creates the
	// Effect's emblem.
	checkForms(t, assignNoCombatDamageForms)

	// The static and duration half: the emblem carries exactly the
	// AssignNoCombatDamage static, and the host-leaves-or-EOT duration keeps
	// both boundaries rather than silently collapsing to this-turn-only.
	cardsDir := ratchetCards(t)
	dir := t.TempDir()
	var names []string
	for _, f := range assignNoCombatDamageForms {
		names = append(names, f.card)
	}
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	byName := map[string]*lCard{}
	for _, c := range lw.cards {
		byName[c.Name] = c
	}
	for _, f := range assignNoCombatDamageForms {
		c := byName[f.card]
		if c == nil || len(c.Refusals) != 0 {
			continue // checkForms reports the lowering/no-refusal half
		}
		if len(c.Emblems) == 0 {
			t.Errorf("%s: no emblems, want one per triggering resolution", f.card)
			continue
		}
		found := false
		for i := range c.Emblems {
			e := &c.Emblems[i]
			if !e.UntilEOT || !e.WhileHost || e.UntilNextTurn {
				t.Errorf("%s emblem[%d] = %+v, want UntilEOT and WhileHost", f.card, i, *e)
			}
			for _, st := range e.Statics {
				if st.Kind != "AssignNoCombatDamage" {
					continue
				}
				found = true
				if st.F == nil {
					t.Errorf("%s emblem[%d]: AssignNoCombatDamage has no ValidCard$ filter", f.card, i)
				}
			}
		}
		if !found {
			t.Errorf("%s: no emblem carries the AssignNoCombatDamage static", f.card)
		}
	}
}
