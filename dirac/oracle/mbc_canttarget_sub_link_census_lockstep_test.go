package oracle

import "testing"

// CR 601.2c with CR 602.2b: a spell's or ability's targets -- every chain
// link's beside its root's own -- are announced when it is cast or activated,
// so an ability whose link can name no legal target cannot be announced at
// all. Oko, Thief of Crowns' [-5] targets "an artifact or creature you
// control" and "a creature an opponent controls with power 3 or less"; the
// only opposing creature is Shanna, Sisay's Legacy, which "can't be the
// target of abilities your opponents control". Oko's loyalty is 5, so the
// kernel correctly offers only [+2] and [+1]; gorge's offer census classified
// the exchange link's ValidSA$ by the link's own unclassifiable DB kind
// (cantTargetValidSA failed open) and let Shanna stand as the link's census
// candidate, offering the [-5] (decision 170 of 6, options, gorge leading, on
// the autopay surface).
func TestMBCOkoThiefOfCrownsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "canttarget_sub_link_census")
}
