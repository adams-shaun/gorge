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
