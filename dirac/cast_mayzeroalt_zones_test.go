package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestZeroAltFullListReachesEveryZone: a non-granting alternative whose zone
// list names every zone (kgen marks it StFreeOnly beside StMayZeroAlt: As
// Foretold's Hand,Graveyard,Library,Exile,Command) is the alternative of a
// card in the graveyard too, where a single-zone one (Dragon's Smile's exile,
// no StFreeOnly) reaches only its own zone.
func TestZeroAltFullListReachesEveryZone(t *testing.T) {
	tbl := v.NewTable()
	tbl.Exprs = []kop.Expr{}
	mk := func(name string, fl kop.StaticFlags, z kop.Zone) CardID {
		id := CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: name, Types: kop.TEnchantment,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{
				Kind: kop.StatMayPlayFrom, Zone: z,
				StFlags: kop.StMayZeroAlt | kop.StWithoutManaCost | kop.StOncePerTurn | fl}}}})
		return id
	}
	fullC := mk("Test Full List Alt", kop.StFreeOnly, kop.ZoneHand)
	oneC := mk("Test Hand Only Alt", 0, kop.ZoneHand)
	for _, c := range []struct {
		card CardID
		want int
	}{{fullC, 1}, {oneC, 0}} {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		sc.perm(c.card, 0)
		o := sc.obj(v.ThinkTwice, 0)
		sc.s.place(o, kop.ZoneGraveyard, false)
		if got, _ := sc.s.zeroAlts(sc.k, 0, o); got != c.want {
			t.Errorf("%s: %d alternatives for a graveyard card, want %d", tbl.Cards[c.card].Name, got, c.want)
		}
		h := sc.obj(v.ThinkTwice, 0)
		sc.s.place(h, kop.ZoneHand, false)
		if got, _ := sc.s.zeroAlts(sc.k, 0, h); got != 1 {
			t.Errorf("%s: %d alternatives for a hand card, want 1", tbl.Cards[c.card].Name, got)
		}
	}
}
