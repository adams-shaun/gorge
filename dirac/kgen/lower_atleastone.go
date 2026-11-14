package kgen

// atLeastOneInstance reads a DamageDealtOnce trigger's AtLeastOneInstance$
// <op><n> (Matoc, Lavamancer: "if at least one permanent or player was dealt
// exactly 2 of that damage"; gorge trigmatch.AtLeastOneInstanceMatches over
// each recipient's share of the source's damage batch) into tr.InstGate, a
// kop.DmgGate the kernel reads from Trigger.N (dirac/trig_atleastone.go).
// Only the once-per-source line with no recipient read carries it: a
// ValidTarget$ line queues per recipient in the kernel. Any other mode or
// operand refuses.
func atLeastOneInstance(tp *params, tr *lTrigger, mode string) error {
	v, ok := tp.get("AtLeastOneInstance")
	if !ok {
		return nil
	}
	if mode != "DamageDealtOnce" {
		return refuse("AtLeastOneInstance$ on a %s trigger", mode)
	}
	if _, vt := tp.m["ValidTarget"]; vt {
		return refuse("AtLeastOneInstance$ beside ValidTarget$")
	}
	cmp, n, ok := trigCmp(v)
	if !ok {
		return refuse("AtLeastOneInstance$ %q", v)
	}
	tr.InstGate = trigCmpN(cmp, n)
	return nil
}
