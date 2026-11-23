package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRestrProvGraveyardOrigin: PredN 7 (wasCastFromYourGraveyard) of a mana
// restriction holds for a graveyard offer and for a pushed entry cast from a
// graveyard, and for no other origin; +16 negates it. Lord of the Forsaken.
func TestRestrProvGraveyardOrigin(t *testing.T) {
	for _, c := range []struct {
		name string
		n    int16
		o    restrOrigin
		want bool
	}{
		{"offer from graveyard", 7, restrOrigin{from: kop.ZoneGraveyard}, true},
		{"offer from hand", 7, restrOrigin{from: kop.ZoneHand}, false},
		{"offer from exile", 7, restrOrigin{from: kop.ZoneExile}, false},
		{"entry from graveyard", 7, restrOrigin{from: kop.ZoneGraveyard, cast: true, byYou: true}, true},
		{"entry from hand", 7, restrOrigin{from: kop.ZoneHand, cast: true, byYou: true}, false},
		{"entry from graveyard by another", 7, restrOrigin{from: kop.ZoneGraveyard, cast: true}, false},
		{"negated entry from graveyard", 7 | 16, restrOrigin{from: kop.ZoneGraveyard, cast: true, byYou: true}, false},
		{"negated entry from hand", 7 | 16, restrOrigin{from: kop.ZoneHand, cast: true, byYou: true}, true},
	} {
		if got := restrProvHolds(c.n, c.o); got != c.want {
			t.Errorf("%s: restrProvHolds(%d, %+v) = %v, want %v", c.name, c.n, c.o, got, c.want)
		}
	}
}
