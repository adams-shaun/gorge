package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLandwalk: a creature with printed landwalk (CardDef.Landwalk, CR
// 702.14) can't be blocked while the defending player controls a land
// matching the filter; it can once that land is gone, or once it lost its
// abilities (gorge LandwalkEvades reads its current keywords).
func TestLandwalk(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	sub := len(tbl.Subtypes)
	tbl.Subtypes = append(append([]string(nil), tbl.Subtypes...), "Plains")
	var sm kop.SubMask
	sm[sub>>6] |= 1 << (sub & 63)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TLand, Sub: uint8(sub + 1)})
	f := kop.FilterID(len(tbl.Filters) - 1)
	plains, walker, bear := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1), CardID(len(tbl.Cards)+2)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Plains", Types: kop.TLand, Subtypes: sm},
		kop.CardDef{Name: "Test Plainswalker", Types: kop.TCreature, Power: 2, Toughness: 2, Landwalk: f},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	a := sc.perm(walker, 0)
	b := sc.perm(bear, 1)
	if !sc.s.canBlock(sc.k, sc.slot(b), sc.slot(a)) {
		t.Fatal("blocked without a Plains: want blockable")
	}
	sc.perm(plains, 1)
	if sc.s.canBlock(sc.k, sc.slot(b), sc.slot(a)) {
		t.Fatal("the defender controls a Plains: want unblockable")
	}
	sc.s.perms[sc.slot(a)].ovr.flags |= ovLoseAllEOT
	if !sc.s.canBlock(sc.k, sc.slot(b), sc.slot(a)) {
		t.Fatal("lost its abilities: want blockable")
	}
}

// kwEvasionScen is a BLB scenario with two vanilla-ish creatures on each side.
func kwEvasionScen(t *testing.T) (*scen, Obj, Obj) {
	t.Helper()
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 2}, {Name: "Plains", Count: 8}})
	fc := id["Valley Floodcaller"]
	return sc, sc.perm(fc, 0), sc.perm(fc, 1)
}

// TestFearShadowBlock: canBlock's fear and shadow arms (gorge
// combat.CanBlock, CR 702.28, 702.36).
func TestFearShadowBlock(t *testing.T) {
	sc, a, b := kwEvasionScen(t)
	as, bs := sc.slot(a), sc.slot(b)
	if !sc.s.canBlock(sc.k, bs, as) {
		t.Fatal("plain creatures: no block")
	}
	sc.s.perms[as].eotKw |= kop.KwShadow
	if sc.s.canBlock(sc.k, bs, as) {
		t.Fatal("shadow attacker blocked by a non-shadow creature")
	}
	sc.s.perms[bs].eotKw |= kop.KwShadow
	if !sc.s.canBlock(sc.k, bs, as) {
		t.Fatal("shadow attacker not blockable by a shadow creature")
	}
	sc.s.perms[as].eotKw &^= kop.KwShadow
	if sc.s.canBlock(sc.k, bs, as) {
		t.Fatal("shadow creature blocked a non-shadow attacker")
	}
	sc.s.perms[bs].eotKw &^= kop.KwShadow
	sc.s.perms[as].eotKw |= kop.KwFear
	if sc.s.colorsAt(sc.k, bs)&kop.ColB != 0 {
		t.Fatal("the scenario blocker is black")
	}
	if sc.s.canBlock(sc.k, bs, as) {
		t.Fatal("fear attacker blocked by a nonartifact, nonblack creature")
	}
}

// TestFearBlackBlocker: a black creature blocks a fear attacker.
func TestFearBlackBlocker(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Agate-Blade Assassin", Count: 2}, {Name: "Plains", Count: 8}})
	c := id["Agate-Blade Assassin"]
	a, b := sc.perm(c, 0), sc.perm(c, 1)
	sc.s.perms[sc.slot(a)].eotKw |= kop.KwFear
	if !sc.s.canBlock(sc.k, sc.slot(b), sc.slot(a)) {
		t.Fatal("a black creature can't block a fear attacker")
	}
}

// TestEvasion: printed horsemanship (blocked only by horsemanship) and
// intimidate (blocked only by an artifact creature or one sharing a
// colour), off once the attacker lost its abilities (gorge CanBlock).
func TestEvasion(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	n := CardID(len(tbl.Cards))
	horse, horse2, intim, redB, blueB, art := n, n+1, n+2, n+3, n+4, n+5
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Horse", Types: kop.TCreature, Power: 2, Toughness: 2, Evasion: kop.EvHorsemanship},
		kop.CardDef{Name: "Test Horse Blocker", Types: kop.TCreature, Power: 2, Toughness: 2, Evasion: kop.EvHorsemanship},
		kop.CardDef{Name: "Test Intimidator", Types: kop.TCreature, Power: 2, Toughness: 2, Colors: kop.ColR, Evasion: kop.EvIntimidate},
		kop.CardDef{Name: "Test Red", Types: kop.TCreature, Power: 2, Toughness: 2, Colors: kop.ColR},
		kop.CardDef{Name: "Test Blue", Types: kop.TCreature, Power: 2, Toughness: 2, Colors: kop.ColU},
		kop.CardDef{Name: "Test Golem", Types: kop.TCreature | kop.TArtifact, Power: 2, Toughness: 2})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	h, h2, i := sc.perm(horse, 0), sc.perm(horse2, 1), sc.perm(intim, 0)
	r, u, g := sc.perm(redB, 1), sc.perm(blueB, 1), sc.perm(art, 1)
	for _, c := range []struct {
		b, a Obj
		want bool
	}{{u, h, false}, {h2, h, true}, {h, u, true}, {u, i, false}, {r, i, true}, {g, i, true}} {
		if got := sc.s.canBlock(sc.k, sc.slot(c.b), sc.slot(c.a)); got != c.want {
			t.Errorf("blocker %d attacker %d: canBlock %v, want %v", c.b, c.a, got, c.want)
		}
	}
	sc.s.perms[sc.slot(h)].ovr.flags |= ovLoseAllEOT
	if !sc.s.canBlock(sc.k, sc.slot(u), sc.slot(h)) {
		t.Error("lost its abilities: want blockable")
	}
}

// TestSkulk: a printed skulk attacker (CardDef.Evasion EvSkulk) can't be
// blocked by a creature with greater power (CR 702.118a; gorge CanBlock's
// derived powers); equal or smaller power blocks; lost abilities lift it.
func TestSkulk(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	n := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Skulker", Types: kop.TCreature, Power: 2, Toughness: 2, Evasion: kop.EvSkulk},
		kop.CardDef{Name: "Test Big", Types: kop.TCreature, Power: 3, Toughness: 3},
		kop.CardDef{Name: "Test Even", Types: kop.TCreature, Power: 2, Toughness: 5})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	a, big, even := sc.perm(n, 0), sc.perm(n+1, 1), sc.perm(n+2, 1)
	if sc.s.canBlock(sc.k, sc.slot(big), sc.slot(a)) {
		t.Error("a greater-power creature blocked a skulker")
	}
	if !sc.s.canBlock(sc.k, sc.slot(even), sc.slot(a)) {
		t.Error("an equal-power creature could not block a skulker")
	}
	sc.s.perms[sc.slot(a)].ovr.flags |= ovLoseAllEOT
	if !sc.s.canBlock(sc.k, sc.slot(big), sc.slot(a)) {
		t.Error("lost its abilities: want blockable")
	}
}
