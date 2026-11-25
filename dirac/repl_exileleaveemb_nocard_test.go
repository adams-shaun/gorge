package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// exileAnyHostEmblemScen is vanilla's table plus a printed battlefield-origin
// dies-to-exile host (Repl.C 0, the Kalitas shape: it sets rbExileOnLeave but
// allocates no genPrep.exileAnyC) and an any-zone (Repl.C 1) emblem over lands
// (the Yawgmoth's Will shape), which opens the placeFrom gate on
// prep.embExileAny alone. The emblem is declared in the table but not
// created.
func exileAnyHostEmblemScen(t *testing.T) (*scen, CardID) {
	t.Helper()
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Exprs = make([]kop.Expr, 1) // a generated table: prepareGen keeps g.gen
	tbl.Conds = []kop.Cond{}
	cf := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	lf := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TLand})
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Exile Death Host", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplExileOnLeave, Filter: cf}}}})
	host := CardID(len(tbl.Cards) - 1)
	tbl.Emblems = append(append([]kop.Emblem{}, v.Table.Emblems...), kop.Emblem{UntilEOT: true,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplExileOnLeave, Filter: lf, C: 1}}}})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	g := sc.k.gen
	if g == nil || !sc.k.embExileAny {
		t.Fatal("precondition: the any-zone emblem did not open the placeFrom gate")
	}
	if g.exileAnyC != nil || g.replBits[host]&rbExileOnLeave == 0 {
		t.Fatalf("precondition: want a printed C=0 host (bit set) and no exileAnyC, got bit %v exileAnyC %v",
			g.replBits[host]&rbExileOnLeave != 0, g.exileAnyC != nil)
	}
	return sc, host
}

// TestEmblemExileAnyBesideBattlefieldHost: a land goes to the graveyard while
// a printed C=0 dies-to-exile host is on the battlefield and the table holds
// an any-zone emblem. exileAnyN used to index the nil exileAnyC for the host
// and panic. Without the emblem created the land reaches the graveyard; with
// it created the land is exiled (the compared zones differ).
func TestEmblemExileAnyBesideBattlefieldHost(t *testing.T) {
	sc, host := exileAnyHostEmblemScen(t)
	s, k := sc.s, sc.k
	h := sc.perm(host, 0)
	land := sc.perm(v.Plains, 1)
	if !sc.onBF(h) || !sc.onBF(land) {
		t.Fatal("precondition: the host or the land is not in play")
	}
	s.leave(k, sc.slot(land), kop.ZoneGraveyard, false)
	if z := s.objs[land].zone; z != kop.ZoneGraveyard {
		t.Fatalf("with no emblem created the land is in zone %d, want the graveyard", z)
	}
	idx := uint8(len(k.t.Emblems))
	s.g.emb[0][0], s.g.embC[0][0], s.g.embO[0][0], s.g.embN[0][0], s.g.embT[0][0] = idx, 0, NoObj, 0, 1
	s.g.embF[0] = 0
	s.g.nemb[0] = 1
	land2 := sc.perm(v.Plains, 1)
	if !sc.onBF(land2) {
		t.Fatal("precondition: the second land is not in play")
	}
	s.leave(k, sc.slot(land2), kop.ZoneGraveyard, false)
	if z := s.objs[land2].zone; z != kop.ZoneExile {
		t.Fatalf("with the emblem created the land is in zone %d, want exile", z)
	}
	if s.Stalled() {
		t.Fatalf("stalled: %s", StallName(s.StallReason()))
	}
}
