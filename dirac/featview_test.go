package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Baylen's "Tap two untapped tokens you control" mana ability is a source
// only while two tokens are untapped, and the card-view cache must notice:
// the tokens' state is not in Baylen's own hash.
func TestFeatTapOtherManaSource(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Baylen, the Haymaker", Count: 1}, {Name: "Hop to It", Count: 1}, {Name: "Plains", Count: 8}})
	token := CardID(0)
	for i := range sc.k.t.Cards {
		if sc.k.t.Cards[i].Types&kop.TToken != 0 {
			token = CardID(i)
			break
		}
	}
	if token == 0 {
		t.Fatal("no token card in the table")
	}
	baylen := sc.perm(id["Baylen, the Haymaker"], 0)
	if sc.s.FeatManaActivatable(baylen) {
		t.Fatal("Baylen is a mana source with no tokens")
	}
	first, tapped := sc.perm(token, 0), sc.perm(token, 0)
	sc.s.objs[first].of |= ofToken
	sc.s.objs[tapped].of |= ofToken
	if !sc.s.FeatManaActivatable(baylen) {
		t.Fatal("Baylen is not a mana source with two untapped tokens")
	}
	key := sc.s.FeatStaticKey()
	sc.s.perms[sc.slot(tapped)].flags |= pfTapped
	if sc.s.FeatManaActivatable(baylen) {
		t.Fatal("Baylen is a mana source with one untapped token")
	}
	if sc.s.FeatStaticKey() == key {
		t.Fatal("tapping a token left the derived-context key unchanged")
	}
}
