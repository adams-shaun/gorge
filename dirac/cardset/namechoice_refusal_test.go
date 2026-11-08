package cardset

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A matchup holding an any-card name chooser (Sorcerous Spyglass) beside a card
// that activates from off the battlefield (Suspicious Shambler) is a typed pair
// refusal, which kfuzz counts as refused-pair, not a plain error.
func TestNameChooserOffBattlefieldRefusesPair(t *testing.T) {
	withPairGuard(t, nil)
	d := guardPool()
	choose := kop.ExtInstr(kop.ExtChooseName)
	choose.B = kop.Const(1) // any card name, lands included
	d.Defs[0].Abilities = []kop.Ability{{Kind: kop.AbStatic, Prog: []kop.Instr{choose}}}
	d.Defs[1].Abilities = []kop.Ability{{Kind: kop.AbActivated, ActZone: kop.ZoneGraveyard}}
	if _, any := choosesNameKind(&d.Defs[0]); !any || !activatesOffBattlefield(&d.Defs[1]) {
		t.Fatal("fixture: Card A must choose any card name and Card B activate off the battlefield")
	}
	a, b := []Entry{{Name: "Card A", Count: 1}}, []Entry{{Name: "Card B", Count: 1}}
	for _, alone := range [][]Entry{a, b} {
		if _, _, _, err := NewDataSet(d).Matchup(alone, alone); err != nil {
			t.Fatalf("%s alone: %v", alone[0].Name, err)
		}
	}
	tb, da, db, err := NewDataSet(d).Matchup(a, b)
	var r *InteractionRefusal
	if !errors.As(err, &r) {
		t.Fatalf("err %v, want an InteractionRefusal", err)
	}
	if tb != nil || da != nil || db != nil {
		t.Error("a refused matchup returns nothing to play")
	}
	if r.Reason != "namechoice" || len(r.Cards) != 2 || r.Cards[0] != "Card A" || r.Cards[1] != "Card B" {
		t.Errorf("refusal %+v", r)
	}
}
