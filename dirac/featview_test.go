package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// A spell on the stack is controlled by its caster, not its owner, for the
// off-battlefield P/T the features read (gorge's Derived reads the
// controller): FDN pair 180, p1 casts p0's Mocking Sprite through Etali and
// p0's Empyrean Eagle ("other creatures you control with flying get
// +1/+1") does not pump it; pair 6229, Consuming Aberration cast by its
// owner's opponent counts the caster's opponent's graveyard.
func TestFeatStackSpellController(t *testing.T) {
	deck := []cardset.Entry{{Name: "Empyrean Eagle", Count: 1}, {Name: "Mocking Sprite", Count: 1}, {Name: "Island", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	for _, caster := range []uint8{0, 1} {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		sc.perm(id["Empyrean Eagle"], 0)
		sp := sc.obj(id["Mocking Sprite"], 0)
		sc.s.objs[sp].zone = kop.ZoneStack
		sc.s.push(stackEntry{kind: seSpell, obj: sp, ctrl: caster, card: id["Mocking Sprite"], trigObj: NoObj})
		dp, dt, _, _ := sc.s.FeatStaticPT(sp)
		want := 0
		if caster == 0 {
			want = 1
		}
		if dp != want || dt != want {
			t.Fatalf("caster %d: Mocking Sprite on the stack gets %+d/%+d, want %+d/%+d", caster, dp, dt, want, want)
		}
	}
}

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
