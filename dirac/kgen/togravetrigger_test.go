package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The multi-origin ChangesZone trigger (lower_trigger.go's Origin$ set):
// "put into a graveyard from anywhere other than the battlefield" — Syr
// Konrad, the Grim, Ultron the Annihilator. gorge matches the move's from-zone
// against the Origin$ set as the move completes (trigmatch
// ZoneChangeMatchesWithCapture); the kernel is one TrigToGraveyard ability
// the placement itself fires (dirac/trig_tograve.go from dirac/zones.go
// placeFromK), Trigger.N the origin zones as a 1<<kop.Zone mask. The probe
// paraphrases carry the parameter shapes; each out-of-shape edit must refuse
// by name.

const tgProbe = `Name:To Graveyard Probe
ManaCost:3 B B
Types:Legendary Creature Human Knight
PT:5/4
T:Mode$ ChangesZone | Origin$ Hand,Graveyard,Exile,Stack,Library,Command | Destination$ Graveyard | ValidCard$ Creature.Other | TriggerZones$ Battlefield | Execute$ TrigDmg | TriggerDescription$ probe
SVar:TrigDmg:DB$ DealDamage | Defined$ Player.Opponent | NumDmg$ 1
Oracle:probe
`

func tgTrigger(t *testing.T, name, src string) *lTrigger {
	t.Helper()
	c := lowerScriptCard(t, name, src)
	for _, rf := range c.Refusals {
		t.Errorf("%s refused: %s", name, rf.Reason)
	}
	for _, a := range c.Abilities {
		if a.Trigger != nil && a.Trigger.Event == "ToGraveyard" {
			return a.Trigger
		}
	}
	t.Fatalf("%s lowered no TrigToGraveyard line", name)
	return nil
}

func TestTrigToGraveyardLowers(t *testing.T) {
	// the probe lowers, to the origin mask the kernel reads
	tr := tgTrigger(t, "To Graveyard Probe", tgProbe)
	want := int16(0)
	for _, z := range []kop.Zone{kop.ZoneHand, kop.ZoneGraveyard, kop.ZoneExile, kop.ZoneStack, kop.ZoneLibrary, kop.ZoneCommand} {
		want |= 1 << uint(z)
	}
	if tr.N != want {
		t.Errorf("probe mask %d, want %d (Hand,Graveyard,Exile,Stack,Library,Command)", tr.N, want)
	}
	if tr.Origin != zAny || tr.Dest != zGraveyard {
		t.Errorf("probe Origin %d Dest %d, want Any->Graveyard", tr.Origin, tr.Dest)
	}

	// Syr Konrad, the Grim, the MBC carrier, lowers with it
	c := lowerCorpusCard(t, "Syr Konrad, the Grim")
	for _, rf := range c.Refusals {
		t.Errorf("Syr Konrad, the Grim refused: %s", rf.Reason)
	}
	sawTrig := false
	for _, a := range c.Abilities {
		if a.Trigger != nil && a.Trigger.Event == "ToGraveyard" {
			sawTrig = true
			if a.Trigger.N != want {
				t.Errorf("Syr Konrad mask %d, want %d", a.Trigger.N, want)
			}
		}
	}
	if !sawTrig {
		t.Error("Syr Konrad, the Grim lowered no TrigToGraveyard line")
	}

	// and it builds, with the kop shape the kernel reads
	_, all, _, err := buildPool(&lowered{cards: []*lCard{lowerScriptCard(t, "To Graveyard Probe", tgProbe)}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	b := &all[0]
	sawKop := false
	for i := range b.def.Abilities {
		ab := &b.def.Abilities[i]
		if ab.Kind == kop.AbTriggered && ab.Trig.Event == kop.TrigToGraveyard {
			sawKop = true
			if ab.Trig.N != want {
				t.Errorf("built trigger N %d, want %d", ab.Trig.N, want)
			}
		}
	}
	if !sawKop {
		t.Error("built card: no TrigToGraveyard ability")
	}

	// each out-of-shape edit refuses by name
	if rs := lowerScript(t, "To Graveyard Probe", strings.Replace(tgProbe, "Hand,Graveyard,Exile,Stack,Library,Command", "Hand,Battlefield", 1)); !anyRefusal(rs, "includes Battlefield") {
		t.Errorf("battlefield in the set: refusals %q, want it refused", rs)
	}
	if rs := lowerScript(t, "To Graveyard Probe", strings.Replace(tgProbe, "Hand,Graveyard,Exile,Stack,Library,Command", "Hand,Phasing", 1)); !anyRefusal(rs, `Origin$ "`) {
		t.Errorf("unknown zone word: refusals %q, want one naming it", rs)
	}
	if rs := lowerScript(t, "To Graveyard Probe", strings.Replace(tgProbe, "Destination$ Graveyard", "Destination$ Exile", 1)); !anyRefusal(rs, "to Exile") {
		t.Errorf("Exile destination: refusals %q, want it refused", rs)
	}
	if rs := lowerScript(t, "To Graveyard Probe", strings.Replace(tgProbe, "Destination$ Graveyard", "Destination$ Graveyard,Hand", 1)); !anyRefusal(rs, "to Graveyard,Hand") {
		t.Errorf("destination set: refusals %q, want it refused", rs)
	}
}
