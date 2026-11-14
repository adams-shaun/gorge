package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// tableTrigFlag2 reports whether some trigger of t on event ev carries
// Flags2 bit f.
func tableTrigFlag2(t *kop.Table, ev kop.TrigEvent, f kop.TrigFlags2) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbTriggered && a.Trig.Event == ev && a.Trig.Flags2&f != 0 {
				return true
			}
		}
	}
	return false
}

// tableNthFiltered reports a Tf4NthFiltered line in t: it reads the cast ring.
// The bit is shared with Tf4CauseSpellAb (a LifeLost line), so the scan is
// scoped to SpellCast triggers, the only event that sets Tf4NthFiltered.
func tableNthFiltered(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if a := &t.Cards[i].Abilities[j]; a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigSpellCast && a.Trig.Flags4&kop.Tf4NthFiltered != 0 {
				return true
			}
		}
	}
	return false
}
