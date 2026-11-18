package oracle

import "testing"

// CR 702.2b with CR 615.1: a deathtouch source whose combat damage is wholly
// prevented dealt no damage, so it leaves no deathtouch mark. Baleful Strix
// and Filth both block Plagon, Lord of the Beach; Benevolent Ancestor's
// "prevent the next 1 damage" shield absorbs the Strix's 1 damage whole. The
// kernel recomputed the mark in preventCombatEv while the prevented event
// still counted, so Plagon kept a Deathtouched mark gorge never added (decision
// 354 of dr-mbc-aaab pair 139, projection Cards[23].Counters[3]).
func TestMBCPlagonLordOfTheBeachReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "prevented_deathtouch_event_leaves_no_mark")
}
