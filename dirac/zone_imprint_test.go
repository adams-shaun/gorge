package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// imprintScen builds a resolving trigger whose program is prog, with two
// Savannah Lions on the battlefield as its targets 0 and 1.
func imprintScen(t *testing.T, prog []kop.Instr) (sc *scen, a, b Obj) {
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
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Imprint", Types: kop.TCreature, Power: 1, Toughness: 1,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered, Prog: prog}}})
	sc = newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	s := sc.s
	a, b = sc.perm(id["Savannah Lions"], 0), sc.perm(id["Savannah Lions"], 0)
	srcObj := sc.perm(src, 0)
	e := stackEntry{kind: seTrigger, obj: srcObj, srcVer: s.objs[srcObj].ver, card: src, ab: 0, ctrl: 0, trigObj: NoObj, nt: 2}
	e.tgt = [kop.MaxTargets]uint8{uint8(a), uint8(b), NoTarget}
	e.tver = [kop.MaxTargets]uint8{s.objs[a].ver, s.objs[b].ver, 0}
	s.sp = 1
	s.stack[0] = e
	s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
	return sc, a, b
}

func imprintExile(ref kop.Ref, flags int16) kop.Instr {
	return kop.Instr{Op: kop.OpExileEx, Obj: ref, C: flags}
}

// TestImprintSet: Imprint$ writers fill the imprinted set apart from the
// remembered set, RefImprinted reads it, the clear op empties it, a repeat is
// kept once, and a seventeenth object stalls with StallImprint.
func TestImprintSet(t *testing.T) {
	clear := kop.Instr{Op: kop.OpExt, A: kop.ValueRef{N: int16(kop.ExtClearImprinted), K: kop.VConst}}

	sc, a, b := imprintScen(t, []kop.Instr{
		imprintExile(kop.RefTarget0, kop.MvImprint),
		imprintExile(kop.RefTarget1, kop.MvImprint),
		imprintExile(kop.RefImprinted, kop.MvRemember), // reads the set's first object
	})
	s := sc.s
	s.runProg(sc.k)
	if s.g.nimp != 2 || s.g.imp[0] != a || s.g.imp[1] != b {
		t.Fatalf("imprinted set %v (n %d), want [%d %d]", s.g.imp[:s.g.nimp], s.g.nimp, a, b)
	}
	if s.objs[a].zone != kop.ZoneExile || s.objs[b].zone != kop.ZoneExile {
		t.Fatalf("the imprinted cards sit in zones %d, %d, want exile", s.objs[a].zone, s.objs[b].zone)
	}
	if s.g.nrem != 1 || s.g.rem[0] != a {
		t.Fatalf("RefImprinted read %v (n %d) into the remembered set, want its first object [%d]", s.g.rem[:s.g.nrem], s.g.nrem, a)
	}
	if s.g.nimp != 2 {
		t.Fatal("remembering an imprinted object must leave the imprinted set alone")
	}

	// a copy of the state is the same game; one imprint apart is not
	cp := new(State)
	cp.CopyFrom(s)
	if !s.SameGame(cp) {
		t.Fatal("a copy differs from its source")
	}
	cp.g.nimp--
	if s.SameGame(cp) {
		t.Fatal("states with different imprinted sets compare equal")
	}

	// the clear op empties it, and leaves the remembered set
	sc, _, _ = imprintScen(t, []kop.Instr{
		imprintExile(kop.RefTarget0, kop.MvImprint|kop.MvRemember),
		imprintExile(kop.RefTarget1, kop.MvImprint),
		clear,
	})
	sc.s.runProg(sc.k)
	if sc.s.g.nimp != 0 || sc.s.g.nrem != 1 {
		t.Fatalf("after the clear: imprinted %d, remembered %d, want 0 and 1", sc.s.g.nimp, sc.s.g.nrem)
	}

	// a repeat adds nothing; the sixteenth is kept, the seventeenth stalls
	n := new(State)
	n.imprint(3)
	n.imprint(3)
	if n.g.nimp != 1 {
		t.Fatalf("a repeated object counted twice: %d", n.g.nimp)
	}
	for i := 1; i < maxImp; i++ {
		n.imprint(Obj(i + 10))
	}
	if n.g.nimp != maxImp || n.StallReason() != StallNone {
		t.Fatalf("a full set: n %d, stall %q, want %d and none", n.g.nimp, StallName(n.StallReason()), maxImp)
	}
	n.imprint(Obj(100))
	if n.g.nimp != maxImp || n.StallReason() != StallImprint || !n.over || n.win != -1 {
		t.Fatalf("one past the cap: n %d, stall %q over %v win %d, want a counted StallImprint draw", n.g.nimp, StallName(n.StallReason()), n.over, n.win)
	}
	if StallName(StallImprint) != "imprint" || StallClassOf(StallImprint) != StallCapacity {
		t.Fatalf("StallImprint is %q class %d, want a named capacity reason", StallName(StallImprint), StallClassOf(StallImprint))
	}

	// linkImp marks a link and rides the copy
	s.g.linkImp = 1 << 2
	cp.CopyFrom(s)
	if cp.g.linkImp != 1<<2 {
		t.Fatalf("linkImp %b lost in the copy", cp.g.linkImp)
	}
}
