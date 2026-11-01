package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// per-card ChangesZone lines on a zone pair the ETB / Dies /
// Leaves split does not name (CR 603.6c, 603.10a, 700.4). gorge reads
// Destination$ as a zone set off the move event
// (trigmatch.ZoneChangeMatchesWithCapture); the kernel carries the set in
// existing table fields and the move's zone on the existing amt argument of
// fireObj, behind genPrep.anyCzPair:
//
//   - a TrigLeaves line with Trigger.N != 0 fires only for a battlefield leave
//     whose destination is in the mask 1<<kop.Zone (Justice: Hand; Syr Vondam:
//     Graveyard and Exile; Three Tree Scribe: Exile, Hand, Library);
//   - a TrigLeaves line whose From is the graveyard is the per-card form of
//     the fireLeavesGraveyard (Along the Crooked Way, Kishla Skimmer), which
//     needs no code here.

// DebugCzPairsNoDest, when set, ignores a Leaves line's destination mask
// (the TestCzpairsMutation mutation: Justice fires on a death).
var DebugCzPairsNoDest bool

// czAmt is the amt a battlefield leave passes to fireObj: the zone moved to
// when the table has a destination-mask line, else the 0 it passed before.
func czAmt(k *prep, z kop.Zone) int16 {
	if k.gen != nil && k.gen.anyCzPair {
		return int16(z)
	}
	return 0
}

// czLeaveOK decides a TrigLeaves line against the destination riding on amt.
func czLeaveOK(a *kop.Ability, amt int16) bool {
	n := a.Trig.N
	return n == 0 || DebugCzPairsNoDest || n&(1<<uint(amt)) != 0
}

// czPairLine reports whether a is a destination-mask line (genPrep.anyCzPair).
func czPairLine(a *kop.Ability) bool {
	return a.Trig.Event == kop.TrigLeaves && a.Trig.N != 0
}
