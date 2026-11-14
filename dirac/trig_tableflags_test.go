package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// tableWithTrig is a one-card table whose only ability is a trigger on ev
// carrying Flags4 f.
func tableWithTrig(ev kop.TrigEvent, f kop.TrigFlags4) *kop.Table {
	return &kop.Table{Cards: []kop.CardDef{{Name: "T", Abilities: []kop.Ability{
		{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: ev, Flags4: f}},
	}}}}
}

// Tf4CauseSpellAb shares bit 3 with Tf4NthFiltered: a LifeLost line carrying
// it must not read as a filtered Nth-spell line (which arms the cast ring).
func TestTableNthFilteredIgnoresLifeLostCauseBit(t *testing.T) {
	if kop.Tf4CauseSpellAb != kop.Tf4NthFiltered {
		t.Fatal("precondition: the two flags no longer share a bit")
	}
	if !tableNthFiltered(tableWithTrig(kop.TrigSpellCast, kop.Tf4NthFiltered)) {
		t.Fatal("a SpellCast Tf4NthFiltered line must read the cast ring")
	}
	if tableNthFiltered(tableWithTrig(kop.TrigLifeLost, kop.Tf4CauseSpellAb)) {
		t.Fatal("a LifeLost Tf4CauseSpellAb line must not arm the cast ring")
	}
}
