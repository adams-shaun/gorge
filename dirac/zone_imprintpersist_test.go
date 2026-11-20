package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// impPersistScen is a resolving trigger of an ImpPersist source whose program
// is prog, with one Savannah Lions on the battlefield as target 0.
func impPersistScen(t *testing.T, prog []kop.Instr) (sc *scen, a, srcObj Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Savannah Lions", Count: 4}, {Name: "Plains", Count: 16}}
	set := cardsettest.Set(t, "fdn")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Imprint Persist", Types: kop.TCreature, Power: 1, Toughness: 1,
		ImpPersist: true, Abilities: []kop.Ability{{Kind: kop.AbTriggered, Prog: prog}}})
	sc = newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	s := sc.s
	a = sc.perm(id["Savannah Lions"], 0)
	srcObj = sc.perm(src, 0)
	e := stackEntry{kind: seTrigger, obj: srcObj, srcVer: s.objs[srcObj].ver, card: src, ab: 0, ctrl: 0, trigObj: NoObj, nt: 1}
	e.tgt = [kop.MaxTargets]TCode{TCode(a), NoTarget}
	e.tver = [kop.MaxTargets]uint8{s.objs[a].ver, 0}
	s.sp = 1
	s.stack[0] = e
	s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
	return sc, a, srcObj
}

// TestImpPersistSeedsAfterDeparture: gorge keeps Object.Imprinted through a
// battlefield departure for the source's own leaves trigger (Parallax Nexus,
// Saproling Burst) and clears it at the next battlefield entry. The store
// must therefore survive leaveGen and seed the trigger stamped with the
// departed incarnation (ver+1); a new entry wipes the old records.
func TestImpPersistSeedsAfterDeparture(t *testing.T) {
	sc, a, srcObj := impPersistScen(t, []kop.Instr{imprintExile(kop.RefTarget0, kop.MvImprint)})
	s := sc.s
	s.runProg(sc.k)
	if !s.impStoreHas(srcObj, a, kop.ZoneNone) {
		t.Fatalf("the write was not recorded on the source: %v", srcMemRecs(s))
	}
	// a departure: removePerm bumps the ver, then leaveGen runs.
	sl := s.objs[srcObj].slot
	s.objs[srcObj].zone = kop.ZoneGraveyard
	s.objs[srcObj].ver++
	s.leaveGen(sc.k, sl, srcObj)
	if s.osLen() == 0 {
		t.Fatalf("leaveGen dropped the Imprinted records: %v", srcMemRecs(s))
	}
	// the leaves trigger resolves from the departed incarnation (ver+1).
	le := stackEntry{kind: seTrigger, obj: srcObj, srcVer: s.objs[srcObj].ver, card: s.objs[srcObj].card, ab: 0, ctrl: 0, trigObj: NoObj}
	s.res = resCtx{e: le, active: true, lastTok: NoObj, eachObj: NoObj}
	s.g.nimp = 0
	s.impSeed(&le)
	if s.g.nimp != 1 || s.g.imp[0] != a {
		t.Fatalf("the leaves trigger seeded %v (n %d), want [%d]", s.g.imp[:s.g.nimp], s.g.nimp, a)
	}
	// the next battlefield entry is gorge's clear point.
	s.objs[srcObj].zone = kop.ZoneBattlefield
	s.objs[srcObj].ver++
	rec := srcMemRecs(s)[0]
	if s.osLiveNow(&rec) {
		t.Fatalf("the old record is still live before the entry: %v (ver %d)", rec, s.objs[srcObj].ver)
	}
	before := s.osLen()
	s.impEnter(srcObj)
	if after := s.osLen(); after != 0 {
		t.Fatalf("the next entry kept the old Imprinted records: %v (rec.src %d, srcObj %d, before %d, after %d, ver %d, zone %d)", srcMemRecs(s), rec.src, srcObj, before, after, s.objs[srcObj].ver, s.objs[srcObj].zone)
	}
}
