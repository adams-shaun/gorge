package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestMaxHandLatestTimestampWins: two maximum-hand-size statics of one
// player apply in timestamp order (CR 613.7), so the later permanent's sets
// the maximum (gorge maxHandSizeFor): "no maximum" then "ten" is ten, the
// reverse is no maximum, and neither stalls. An emblem's differing value
// still stalls (no emblem timestamp here).
func TestMaxHandLatestTimestampWins(t *testing.T) {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Exprs = make([]kop.Expr, 1) // force the generated path
	hand := func(name string, a int) kop.CardDef {
		return kop.CardDef{Name: name, Types: kop.TArtifact, Abilities: []kop.Ability{{Kind: kop.AbStatic,
			Static: kop.Static{Kind: kop.StatNoMaxHandSize, A: int16(a)}}}}
	}
	tbl.Cards = append(tbl.Cards, hand("Test Tower", 0), hand("Test Rings", 11))
	tower, rings := CardID(len(tbl.Cards)-2), CardID(len(tbl.Cards)-1)
	tbl.Emblems = append(append([]kop.Emblem(nil), v.Table.Emblems...), kop.Emblem{Abilities: []kop.Ability{{Kind: kop.AbStatic,
		Static: kop.Static{Kind: kop.StatNoMaxHandSize, A: 5}}}})
	for _, tc := range []struct {
		name          string
		first, second CardID
		emblem        bool
		want          int
		stall         uint8
	}{
		{"unlimited then ten", tower, rings, false, 10, 0},
		{"ten then unlimited", rings, tower, false, handUnlimited, 0},
		{"emblem disagrees", rings, tower, true, 0, StallHandSize},
	} {
		sc := newScenTable(t, &tbl, Config{NoElide: true})
		if sc.k.gen == nil || !sc.k.gen.anyHandSize {
			t.Fatalf("%s: precondition: the generated path does not see a hand-size static", tc.name)
		}
		a, b := sc.perm(tc.first, 0), sc.perm(tc.second, 0)
		if ta, tb := sc.s.layerTs(sc.slot(a)), sc.s.layerTs(sc.slot(b)); tb <= ta {
			t.Fatalf("%s: precondition: second layer stamp %d not after first %d", tc.name, tb, ta)
		}
		if tc.emblem {
			sc.s.g.nemb[0] = 1
			sc.s.g.emb[0][0] = uint8(len(tbl.Emblems))
		}
		sc.s.g.stalled = 0
		got := sc.s.maxHand(sc.k, 0)
		if sc.s.g.stalled != tc.stall {
			t.Fatalf("%s: stalled %d, want %d", tc.name, sc.s.g.stalled, tc.stall)
		}
		if tc.stall == 0 && got != tc.want {
			t.Errorf("%s: maxHand = %d, want %d (the later static, CR 613.7)", tc.name, got, tc.want)
		}
		if got := sc.s.maxHand(sc.k, 1); tc.stall == 0 && got != 7 {
			t.Errorf("%s: opponent maxHand = %d, want 7", tc.name, got)
		}
	}
}
