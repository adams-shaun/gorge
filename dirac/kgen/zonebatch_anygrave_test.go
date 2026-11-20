package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestChangesZoneAllAnyToGraveyardLowers: "whenever one or more ... cards
// are put into your graveyard from anywhere" (Mode$ ChangesZoneAll, Origin$
// Any, Destination$ Graveyard) is one batch ToGraveyard trigger that the
// placement fires for every origin, not the Dies half plus a
// non-battlefield half. Titania, Voice of Gaea; Turntimber Sower, Sand Scout,
// Crawling Sensation, The Gitrog Monster.
func TestChangesZoneAllAnyToGraveyardLowers(t *testing.T) {
	const probe = `Name:Any Grave Batch Probe
ManaCost:1 G
Types:Creature Elemental
PT:1/1
T:Mode$ ChangesZoneAll | ValidCards$ Land.YouOwn+!token | Origin$ Any | Destination$ Graveyard | TriggerZones$ Battlefield | Execute$ TrigGain | TriggerDescription$ Whenever one or more land cards are put into your graveyard from anywhere, you gain 2 life.
SVar:TrigGain:DB$ GainLife | LifeAmount$ 2
Oracle:probe
`
	c := lowerScriptCard(t, "Any Grave Batch Probe", probe)
	for _, rf := range c.Refusals {
		t.Errorf("probe refused: %s", rf.Reason)
	}
	tr := anyGraveBatchTrigger(t, "Any Grave Batch Probe", c)
	if !tr.Batch {
		t.Error("probe: ToGraveyard trigger has no Batch flag")
	}
	if tr.N != 0 {
		t.Errorf("probe: ToGraveyard origin mask %d, want 0 (any origin)", tr.N)
	}
	if tr.Origin != zAny || tr.Dest != zGraveyard {
		t.Errorf("probe: Origin %d Dest %d, want Any->Graveyard", tr.Origin, tr.Dest)
	}

	// the built table shape: one TrigToGraveyard, From none (any), Tf2Batch
	_, all, _, err := buildPool(&lowered{cards: []*lCard{lowerScriptCard(t, "Any Grave Batch Probe", probe)}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	b := &all[0]
	var got *kop.Trigger
	for i := range b.def.Abilities {
		a := &b.def.Abilities[i]
		if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigToGraveyard {
			got = &a.Trig
		}
	}
	if got == nil {
		t.Fatal("built card: no TrigToGraveyard ability")
	}
	if got.From != kop.ZoneNone {
		t.Errorf("built trigger From %d, want ZoneNone (any origin)", got.From)
	}
	if got.Flags2&kop.Tf2Batch == 0 {
		t.Error("built trigger: no Tf2Batch")
	}

	// the corpus carrier lowers with the same shape
	sower := lowerCorpusCard(t, "Turntimber Sower")
	for _, rf := range sower.Refusals {
		t.Errorf("Turntimber Sower refused: %s", rf.Reason)
	}
	sowerTrig := anyGraveBatchTrigger(t, "Turntimber Sower", sower)
	if !sowerTrig.Batch || sowerTrig.N != 0 {
		t.Errorf("Turntimber Sower: Batch %v N %d, want batch, any origin", sowerTrig.Batch, sowerTrig.N)
	}
}

func anyGraveBatchTrigger(t *testing.T, name string, c *lCard) *lTrigger {
	t.Helper()
	var tr *lTrigger
	for _, a := range c.Abilities {
		if a.Trigger != nil && a.Trigger.Event == "ToGraveyard" {
			if tr != nil {
				t.Fatalf("%s: more than one ToGraveyard trigger", name)
			}
			tr = a.Trigger
		}
	}
	if tr == nil {
		t.Fatalf("%s: no ToGraveyard trigger lowered", name)
	}
	return tr
}
