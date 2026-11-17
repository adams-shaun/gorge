package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// effectStaticDurationForms covers an Effect's Duration$ UntilHostLeavesPlayOrEOT:
// the lifetime is the end of turn OR the host leaving the battlefield, so the
// emblem must carry both boundaries (gorge's EndOfTurnCleanup and effect-source
// expiry). The CanAttackDefender / CantBlockBy carriers lower through the
// modes the kernel already had; the AssignNoCombatDamage carriers
// (Ophidian, Carrion Rats, Dwarven Vigilantes) lower through the
// StatNoCombatDamage static the kernel reads in the combat damage step.
//
// The last two carriers remember the attacker the trigger raised
// (RememberObjects$ TriggeredAttackerLKICopy, ForgetOnMoved$ Battlefield)
// instead of naming the effect's source, so their emblem carries a capture
// and lapses when the remembered attacker leaves the battlefield; the static
// reads the captured attacker, not the effect's source.
var effectStaticDurationForms = []formCase{
	{card: "Harvesttide Sentry", form: "triggered Effect CantBlockBy over Creature.EffectSource, Duration$ UntilHostLeavesPlayOrEOT", lir: "CreateEmblem obj=You\n  emblem[0] trigger=0 statics=1 until=EOT"},
	{card: "Prismari Pledgemage", form: "triggered Effect CanAttackDefender over Card.EffectSource, Duration$ UntilHostLeavesPlayOrEOT", lir: "CreateEmblem obj=You\n  emblem[0] trigger=0 statics=1 until=EOT"},
	{card: "Steelclad Spirit", form: "triggered Effect CanAttackDefender over Card.EffectSource, Duration$ UntilHostLeavesPlayOrEOT", lir: "CreateEmblem obj=You\n  emblem[0] trigger=0 statics=1 until=EOT"},
	{card: "Ophidian", form: "triggered Effect AssignNoCombatDamage over Card.EffectSource, Duration$ UntilHostLeavesPlayOrEOT", lir: "CreateEmblem obj=You"},
	{card: "Carrion Rats", form: "triggered Effect AssignNoCombatDamage over Card.EffectSource, Duration$ UntilHostLeavesPlayOrEOT", lir: "CreateEmblem obj=You"},
	{card: "Dwarven Vigilantes", form: "triggered Effect AssignNoCombatDamage over Card.EffectSource, Duration$ UntilHostLeavesPlayOrEOT", lir: "CreateEmblem obj=You"},
	{card: "Cloak of Confusion", form: "triggered Effect AssignNoCombatDamage over RememberObjects$ TriggeredAttackerLKICopy, ForgetOnMoved$ Battlefield", lir: "emblem[0] trigger=0 statics=1 until=EOT remembers=Triggered lapses"},
	{card: "Laccolith Rig", form: "triggered Effect AssignNoCombatDamage over RememberObjects$ TriggeredAttackerLKICopy, ForgetOnMoved$ Battlefield", lir: "emblem[0] trigger=0 statics=1 until=EOT remembers=Triggered lapses"},
}

func TestEffectStaticDurationForms(t *testing.T) {
	checkForms(t, effectStaticDurationForms)

	// The lowered emblem must carry BOTH end conditions, not just the
	// this-turn one: WhileHost is the host-leaves half the duration names.
	cardsDir := ratchetCards(t)
	dir := t.TempDir()
	var names []string
	for _, f := range effectStaticDurationForms {
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
	noDmg := func(f formCase) bool { return strings.Contains(f.form, "AssignNoCombatDamage") }
	remembered := func(f formCase) bool { return strings.Contains(f.form, "RememberObjects$") }
	for _, f := range effectStaticDurationForms {
		c := byName[f.card]
		if c == nil || len(c.Refusals) != 0 {
			continue // checkForms reports the lowering/no-refusal half
		}
		if len(c.Emblems) == 0 {
			t.Errorf("%s: no emblems, want one per triggering resolution", f.card)
			continue
		}
		for i := range c.Emblems {
			e := &c.Emblems[i]
			if !e.UntilEOT || e.UntilNextTurn {
				t.Errorf("%s emblem[%d] = %+v, want UntilEOT", f.card, i, *e)
			}
			if remembered(f) {
				// the remembered attacker raises the trigger; the emblem
				// captures that object and lapses with it (ForgetOnMoved$
				// Battlefield), and it is not host-bound to the effect source.
				if e.WhileHost {
					t.Errorf("%s emblem[%d] = %+v, want no WhileHost", f.card, i, *e)
				}
				if !e.CaptureSet || e.Capture.Kind != rTriggered || !e.CaptureLapse {
					t.Errorf("%s emblem[%d] = %+v, want Capture=Triggered and CaptureLapse", f.card, i, *e)
				}
			} else if !e.WhileHost {
				t.Errorf("%s emblem[%d] = %+v, want UntilEOT and WhileHost", f.card, i, *e)
			}
			if noDmg(f) {
				for _, st := range e.Statics {
					if st.Kind != "AssignNoCombatDamage" {
						t.Errorf("%s emblem[%d] static %q, want AssignNoCombatDamage", f.card, i, st.Kind)
					}
				}
			}
		}
	}
}
