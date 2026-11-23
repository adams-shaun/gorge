package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// deathRepScen is vanilla's table plus a creature filter, the given hosts
// (one card each, a battlefield dies-to-exile source) and one UntilEOT emblem
// carrying a ReplExileOnLeave over that filter; the emblem sits in player 0's
// slot. The embPrevScen shape (repl_staticprevent_emblem_test.go).
func deathRepScen(t *testing.T, hosts ...func(kop.FilterID) kop.Repl) (*scen, kop.FilterID, []CardID) {
	t.Helper()
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Exprs = make([]kop.Expr, 1) // a generated table: prepareGen keeps g.gen
	tbl.Conds = []kop.Cond{}
	cf := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	var ids []CardID
	for _, r := range hosts {
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Exile Death Host", Types: kop.TEnchantment,
			Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: r(cf)}}})
		ids = append(ids, CardID(len(tbl.Cards)-1))
	}
	tbl.Emblems = append(append([]kop.Emblem{}, v.Table.Emblems...), kop.Emblem{UntilEOT: true,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplExileOnLeave, Filter: cf}}}})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	if !sc.k.embExileLeave || sc.k.gen == nil || sc.k.gen.anyRepl&rbExileOnLeave == 0 {
		t.Fatal("precondition: the emblem's replacement did not reach the table's census")
	}
	idx := uint8(len(tbl.Emblems))
	sc.s.g.emb[0][0], sc.s.g.embC[0][0], sc.s.g.embO[0][0], sc.s.g.embN[0][0], sc.s.g.embT[0][0] = idx, 0, NoObj, 0, 1
	sc.s.g.embF[0] = 0
	sc.s.g.nemb[0] = 1
	return sc, cf, ids
}

// TestEmblemDeathRepExiles: while the emblem lives, a creature's death is
// exiled; after the end-of-turn cleanup drops it (UntilEOT, gorge's
// no-Duration Effect lifetime), a second creature dies to the graveyard. The
// compared values differ: before the cleanup the move reads exile, after it
// the graveyard.
func TestEmblemDeathRepExiles(t *testing.T) {
	sc, _, _ := deathRepScen(t)
	s, k := sc.s, sc.k
	giant := sc.perm(v.HillGiant, 1)
	if !sc.onBF(giant) || s.g.nemb[0] != 1 {
		t.Fatalf("precondition: giant on battlefield %v, %d emblems, want true and 1", sc.onBF(giant), s.g.nemb[0])
	}
	if n := s.embExileLeaveN(k, sc.slot(giant)); n != 1 {
		t.Fatalf("precondition: the emblem walk matches the creature %d times, want 1", n)
	}
	land := sc.perm(v.Plains, 1)
	if n := s.embExileLeaveN(k, sc.slot(land)); n != 0 {
		t.Fatalf("precondition: the emblem walk matches a land %d times, want 0", n)
	}
	s.leave(k, sc.slot(land), kop.ZoneGraveyard, false)
	if z := s.objs[land].zone; z != kop.ZoneGraveyard {
		t.Fatalf("the dying land is in zone %d, want the graveyard (no creature filter match)", z)
	}
	if s.Stalled() {
		t.Fatalf("precondition: already stalled: %s", StallName(s.StallReason()))
	}
	s.leave(k, sc.slot(giant), kop.ZoneGraveyard, false)
	if z := s.objs[giant].zone; z != kop.ZoneExile {
		t.Fatalf("the dying creature is in zone %d, want exile", z)
	}
	if s.Stalled() {
		t.Fatalf("a lone emblem match stalled: %s", StallName(s.StallReason()))
	}
	for _, active := range []uint8{0, 1, 0} {
		s.active = active
		s.cleanupGen(k)
	}
	if s.g.nemb[0] != 0 {
		t.Fatalf("%d emblems after the cleanups, want 0: the UntilEOT emblem was kept", s.g.nemb[0])
	}
	second := sc.perm(v.GrizzlyBears, 1)
	if !sc.onBF(second) {
		t.Fatal("precondition: the second creature is not in play")
	}
	s.leave(k, sc.slot(second), kop.ZoneGraveyard, false)
	if z := s.objs[second].zone; z != kop.ZoneGraveyard {
		t.Fatalf("the second creature is in zone %d, want the graveyard once the emblem is gone", z)
	}
}

// TestEmblemDeathRepCompetes: an emblem beside a card source both matching
// one death records the StallReplLeave cap: gorge poses the CR 616.1 order
// there and the kernel never orders an emblem. The move still reads exile
// (dieZone's n > 1 branch), so the compared zone differs from a plain death.
func TestEmblemDeathRepCompetes(t *testing.T) {
	sc, _, ids := deathRepScen(t, func(cf kop.FilterID) kop.Repl {
		return kop.Repl{Kind: kop.ReplExileOnLeave, Filter: cf}
	})
	s, k := sc.s, sc.k
	if len(ids) != 1 || !sc.k.gen.uses.has(useEmbExileLeave) {
		t.Fatalf("precondition: %d hosts, uses %v, want 1 and the emblem need", len(ids), sc.k.gen.uses)
	}
	h := sc.perm(ids[0], 0)
	giant := sc.perm(v.HillGiant, 1)
	if !sc.onBF(h) || !sc.onBF(giant) {
		t.Fatal("precondition: the host or the creature is not in play")
	}
	if n, _, _, src := s.exileOnLeaveN(k, sc.slot(giant)); n != 2 || src != sc.slot(h) {
		t.Fatalf("precondition: the death counts %d sources (src %d), want the card's 2 with its slot", n, src)
	}
	s.leave(k, sc.slot(giant), kop.ZoneGraveyard, false)
	if st := s.g.stalled; st != StallReplLeave {
		t.Fatalf("an emblem and a card source for one death stalled %s, want %s", StallName(st), StallName(StallReplLeave))
	}
	if z := s.objs[giant].zone; z != kop.ZoneExile {
		t.Fatalf("the dying creature is in zone %d, want exile after the stall", z)
	}
}
