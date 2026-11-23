package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// graveFundBoard is the gravemana scenario with a fee-only repeatable filter
// (Signpost Scarecrow's "{2}: Add one mana of any color", Test Scarecrow), a
// Jack-o'-Lantern shape (Test Jack) in seat 0's graveyard, the named lands
// and the named spell in hand. The Scarecrow is a live census gap when
// the potential pool covers its {2} fee, which starts the prefix search.
func graveFundBoard(t *testing.T, spell string, pool [kop.NumManaKinds]uint8, scarecrow bool, lands ...string) (sc *scen, jack Obj, cast Action) {
	t.Helper()
	set := cardsettest.Set(t, "gravemana")
	tbl0, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl := *tbl0
	tbl.Cards = append([]kop.CardDef(nil), tbl0.Cards...)
	jackID := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Jack", Types: kop.TArtifact, GraveMana: uint8(kop.ColAny)})
	scareID := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Scarecrow", Types: kop.TArtifact, Abilities: []kop.Ability{{
		Kind:       kop.AbMana,
		Cost:       kop.Cost{Mana: kop.ManaCost{Generic: 2}},
		ManaColors: kop.ColAny,
	}}})
	id := map[string]CardID{}
	for c := range tbl0.Cards {
		if _, ok := id[tbl0.Cards[c].Name]; !ok {
			id[tbl0.Cards[c].Name] = CardID(c)
		}
	}
	sc = newScenTableLib(t, &tbl, id["Plains"], Config{NoElide: true})
	jack = sc.grave(jackID, 0, 1)[0]
	for _, l := range lands {
		sc.perm(id[l], 0)
	}
	if scarecrow {
		scare := sc.perm(scareID, 0)
		if !sc.s.scriptFilt(sc.k, 0).has(sc.slot(scare)) {
			t.Fatal("precondition: the Test Scarecrow is no live census filter gap")
		}
	}
	sc.s.pl[0].pool = pool
	bears := sc.hand(id[spell], 0)
	if sc.s.objs[jack].zone != kop.ZoneGraveyard || !graveManaOn(sc.k) {
		t.Fatal("precondition: Test Jack is not a graveyard mana source in the graveyard")
	}
	return sc, jack, MakeAction(AkCast, TCode(bears), 0, 0)
}

// graveFundPath runs the prefix search for cast on sc the way filtScript sets
// it up (a repeatable filter's funding is unbounded) and returns whether it
// found a play and its path.
func graveFundPath(sc *scen, cast Action) (bool, []filtStep) {
	s, k := sc.s, sc.k
	var fs filtSearch
	fs.a, fs.p, fs.gaps, fs.own, fs.sp0 = cast, 0, s.scriptFilt(k, 0), NoSlot, s.sp
	fs.need, _ = s.playNeedPrinted(k, cast)
	fs.useful, fs.usefulOK = s.filtUsefulCost(k, cast)
	fs.other = s.otherGaps(k, 0, cast)
	fs.feeOnly = s.filtRepeatIn(k, fs.gaps)
	fs.floated = poolTotal(&s.pl[0]) > 0
	fs.fees = s.filtFunding(k, 0, fs.gaps)
	fs.fundN = s.filtFunders(k, 0, fs.gaps.or(fs.other))
	t := s.fork()
	t.g.noGrant = true
	s.pool.inFilt++ // (fork made the pool)
	ok := fs.visit(k, t, 0, false)
	s.unfork()
	return ok, append([]filtStep{}, fs.path[:fs.n]...)
}

// TestGraveManaFundSearch: the prefix search funds through a graveyard
// mana source after the battlefield's and the hand's, only where a filter gap
// already runs it (gorge: TestGraveManaGorgeProbe Q1b, Q2).
func TestGraveManaFundSearch(t *testing.T) {
	defer func(old bool) { DebugNoGraveFund = old }(DebugNoGraveFund)
	pool := func(c ...int) (p [kop.NumManaKinds]uint8) {
		for _, k := range c {
			p[k]++
		}
		return p
	}
	for _, c := range []struct {
		name  string
		spell string
		col   int
		pool  [kop.NumManaKinds]uint8
		taps  int // the Mountains tapped before Jack
		want  [kop.NumManaKinds]uint8
	}{
		// two Mountains + Scarecrow ({2}), nothing floating: {1}{G} is short a G (the
		// Scarecrow's RR makes one G and nothing for the {1}). Both Mountains float R, Jack's
		// fee takes one and the cost's G is answered: R + G pay the cost. The battlefield
		// funds before the graveyard, and Jack is never the first step of an empty pool.
		{"nothing floating", "Grizzly Bears", kop.G, pool(), 2, pool(kop.R, kop.G)},
		// {R} floating: {2}{B} is short a B and a mana; the same shape with the pool's R
		{"{R} floating", "Mind Rot", kop.B, pool(kop.R), 2, pool(kop.R, kop.R, kop.B)},
	} {
		sc, jack, cast := graveFundBoard(t, c.spell, c.pool, true, "Mountain", "Mountain")
		if sc.s.graveManaAny(sc.k, 0) != (c.pool != pool()) {
			t.Fatalf("%s: precondition: Jack's activation is listed only while mana floats", c.name)
		}
		ok, path := graveFundPath(sc, cast)
		if !ok {
			t.Errorf("%s: the search found no play: Jack's funding pass is missing", c.name)
			continue
		}
		last := path[len(path)-1]
		if len(path) != c.taps+1 || !last.grave || last.ans-1 != uint8(c.col) || last.gidx != 0 || last.gap {
			t.Errorf("%s: path %+v, want %d Mountain taps then Jack answered colour %d", c.name, path, c.taps, c.col)
		}
		for _, st := range path[:len(path)-1] {
			if st.grave || st.gap {
				t.Errorf("%s: path %+v holds a step before Jack that is no battlefield funding tap", c.name, path)
			}
		}
		// the path played: Jack in exile, the pool as gorge's activation leaves it (the fee's
		// first non-empty colour in W U B R G order, then the answered colour)
		s := sc.s.fork()
		s.filtPlay(sc.k, 0, path)
		if z := s.objs[jack].zone; z != kop.ZoneExile {
			t.Errorf("%s: Jack is in zone %d after the path, want exile", c.name, z)
		}
		if s.pl[0].pool != c.want {
			t.Errorf("%s: pool %v after the path, want %v", c.name, s.pl[0].pool, c.want)
		}
		sc.s.unfork()

		// the fund pass off: no play (the lockstep's mutation)
		DebugNoGraveFund = true
		if ok, path := graveFundPath(sc, cast); ok {
			t.Errorf("%s: DebugNoGraveFund: the search still found path %+v", c.name, path)
		}
		DebugNoGraveFund = false
	}
}

// TestGraveManaFundNoGap: Jack alone never makes a cast affordable (gorge's
// planner prices no graveyard source, TestGraveManaGorgeProbe Q2): with no
// filter gap beside it no search runs, so the {G} cast is not offered.
func TestGraveManaFundNoGap(t *testing.T) {
	sc, _, cast := graveFundBoard(t, "Grizzly Bears", [kop.NumManaKinds]uint8{kop.R: 1}, false, "Mountain")
	if sc.s.scriptFilt(sc.k, 0).any() {
		t.Fatal("precondition: the board has a filter gap")
	}
	if ok, path := graveFundPath(sc, cast); ok {
		t.Errorf("no gap: the search found path %+v through Jack", path)
	}
	d := sc.s.Step()
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkCast && a.Source() == cast.Source() {
			t.Error("Giant Growth is offered with only Jack to pay its {G}")
		}
	}
}
