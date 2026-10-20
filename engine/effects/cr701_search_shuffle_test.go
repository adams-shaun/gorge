package effects

import "testing"

// cr-g09: the R-9 no-host path of a library search that carries Forge's
// ShuffleNonMandatory$ (Path to Exile, Stoneforge Mystic, Ghost Quarter --
// 209 corpus lines). The flag never makes "then shuffle" optional (CR
// 701.23 / 701.24d), so a host that cannot ask still shuffles, and nothing
// is asked at all.
func TestCR70124NoHostSearchShuffleNonMandatoryShuffles(t *testing.T) {
	h, c := fixtureHost(t)
	s := sa(t, "SP$ ChangeZone | Origin$ Library | Destination$ Hand | ChangeType$ Card.namedNothing | ShuffleNonMandatory$ True")
	effChangeZone(h, c, s)
	if got := shuffleEvents(h, 0); got != 1 {
		t.Fatalf("no-host fail-to-find search with ShuffleNonMandatory$ shuffled %d time(s), want exactly 1 (CR 701.24d): %+v", got, h.log)
	}
}
