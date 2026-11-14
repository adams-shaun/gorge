package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// instGateOK is a DamageDealtOnce line's AtLeastOneInstance$ gate (Trigger.N,
// a kop.DmgGate; kgen lower_atleastone.go): some recipient's share of the
// source's damage batch passes it (gorge trigmatch.AtLeastOneInstanceMatches,
// read as the batch closes; Matoc, Lavamancer). A combat step reads the
// source's events (latch.r: a source deals each recipient one event there),
// damageBatch's aggregate fire each recipient's amount (rl.dn), and a lone
// event its own amount. A line with no gate, or of another event, passes.
func instGateOK(t *kop.Trigger, amt int16, latch *dmgLatch, rl *refLatch) bool {
	if t.Event != kop.TrigDamageDealtOnce || t.N == 0 {
		return true
	}
	switch {
	case latch != nil && latch.r != nil:
		for i := 0; i < latch.r.nev; i++ {
			if ev := &latch.r.ev[i]; ev.src == latch.src && ev.n > 0 && kop.DmgGatePass(t.N, int(ev.n)) {
				return true
			}
		}
		return false
	case rl != nil && rl.agg:
		for _, n := range rl.dn {
			if n > 0 && kop.DmgGatePass(t.N, int(n)) {
				return true
			}
		}
		return false
	}
	return kop.DmgGatePass(t.N, int(amt))
}
