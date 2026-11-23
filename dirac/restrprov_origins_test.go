package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRestrProvBareGraveyardAndLibrary: PredN 8 (wasCastFromGraveyard, any
// caster; gorge bareGraveyardCastAdmitsPending: the zone at the offer, the
// origin at the payment) and PredN 9 (wasCastFromYourLibrary, byYou) of a mana
// restriction or cast offer decode like the other origins, never to a
// constant false. +16 negates.
func TestRestrProvBareGraveyardAndLibrary(t *testing.T) {
	for _, c := range []struct {
		name string
		n    int16
		o    restrOrigin
		want bool
	}{
		{"grave offer from graveyard", 8, restrOrigin{from: kop.ZoneGraveyard}, true},
		{"grave offer from hand", 8, restrOrigin{from: kop.ZoneHand}, false},
		{"grave entry from graveyard", 8, restrOrigin{from: kop.ZoneGraveyard, cast: true, byYou: true}, true},
		{"grave entry from graveyard by another", 8, restrOrigin{from: kop.ZoneGraveyard, cast: true}, true},
		{"grave entry from exile", 8, restrOrigin{from: kop.ZoneExile, cast: true, byYou: true}, false},
		{"negated grave entry from graveyard", 8 | 16, restrOrigin{from: kop.ZoneGraveyard, cast: true, byYou: true}, false},
		{"negated grave entry from hand", 8 | 16, restrOrigin{from: kop.ZoneHand, cast: true, byYou: true}, true},
		{"library offer from library", 9, restrOrigin{from: kop.ZoneLibrary}, true},
		{"library offer from graveyard", 9, restrOrigin{from: kop.ZoneGraveyard}, false},
		{"library entry from library", 9, restrOrigin{from: kop.ZoneLibrary, cast: true, byYou: true}, true},
		{"library entry from library by another", 9, restrOrigin{from: kop.ZoneLibrary, cast: true}, false},
		{"library entry from hand", 9, restrOrigin{from: kop.ZoneHand, cast: true, byYou: true}, false},
		{"negated library entry from library", 9 | 16, restrOrigin{from: kop.ZoneLibrary, cast: true, byYou: true}, false},
	} {
		if got := restrProvHolds(c.n, c.o); got != c.want {
			t.Errorf("%s: restrProvHolds(%d, %+v) = %v, want %v", c.name, c.n, c.o, got, c.want)
		}
	}
}
