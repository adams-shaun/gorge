package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTargetsValidAnyZone: a SpellCast TargetsValid$ zone-free object
// alternative (Danitha, Spear of Agony's Creature.OppCtrl) lowers and marks
// its Filter2 clause kop.ZoneAny, the kernel's any-zone read; an
// inZoneBattlefield alternative keeps the battlefield lowering (ZoneNone),
// and a line mixing the two stays refused because the kernel would read the
// battlefield clause in any zone too.
func TestTargetsValidAnyZone(t *testing.T) {
	const tail = " | TriggerZones$ Battlefield | Execute$ TrigGain"
	line := func(v string) string {
		return "T:Mode$ SpellCast | ValidActivatingPlayer$ You | TargetsValid$ " + v + tail
	}
	// Danitha, Spear of Agony's exact trigger (the card's SpellCast line).
	why, tr, pt := ktqBuild(t, line("Opponent,Creature.OppCtrl"))
	if why != "" {
		t.Fatalf("Danitha's TargetsValid$ refused: %s", why)
	}
	if tr.Flags2&kop.Tf2TargetsFilter2 == 0 {
		t.Fatal("TargetsValid$ did not set Tf2TargetsFilter2")
	}
	if tr.Filter2 == 0 {
		t.Fatal("TargetsValid$ built no Filter2")
	}
	sawCreature, sawPlayer := false, false
	for f := tr.Filter2; f != 0; f = pt.filters[f].Or {
		fl := &pt.filters[f]
		if fl.Flags&kop.FPlayer != 0 {
			sawPlayer = true
			if fl.Ctrl != kop.CtrlOpp {
				t.Errorf("player alternative Ctrl = %d, want CtrlOpp", fl.Ctrl)
			}
			continue
		}
		if fl.Types != kop.TCreature {
			t.Errorf("object alternative types = %#x, want TCreature", fl.Types)
		}
		if fl.Ctrl != kop.CtrlOpp {
			t.Errorf("object alternative Ctrl = %d, want CtrlOpp", fl.Ctrl)
		}
		if fl.Zone != kop.ZoneAny {
			t.Errorf("object alternative Zone = %d, want ZoneAny", fl.Zone)
		}
		sawCreature = true
	}
	if !sawCreature || !sawPlayer {
		t.Fatalf("Filter2 chain: creature %v, player %v", sawCreature, sawPlayer)
	}
	// The same alternative with inZoneBattlefield is still battlefield-only,
	// and must not pick up the any-zone marker.
	why, tr, pt = ktqBuild(t, line("Opponent,Creature.OppCtrl+inZoneBattlefield"))
	if why != "" {
		t.Fatalf("inZoneBattlefield alternative refused: %s", why)
	}
	for f := tr.Filter2; f != 0; f = pt.filters[f].Or {
		fl := &pt.filters[f]
		if fl.Flags&kop.FPlayer != 0 {
			continue
		}
		if fl.Zone != kop.ZoneNone {
			t.Errorf("battlefield alternative Zone = %d, want ZoneNone", fl.Zone)
		}
	}
	// A mixed line stays refused: the battlefield clause would otherwise be
	// read in any zone.
	for _, v := range []string{"Opponent,Creature.OppCtrl,Creature.inZoneBattlefield", "Opponent,Creature.OppCtrl,Artifact.YouCtrl+inZoneBattlefield"} {
		if why, _, _ := ktqBuild(t, line(v)); !strings.Contains(why, `TargetsValid "`+v+`"`) {
			t.Errorf("mixed TargetsValid %q: refusal %q", v, why)
		}
	}
}
