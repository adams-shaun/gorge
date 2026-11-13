package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// mpocScen builds p0's first main phase from a pool fixture's table with the
// named board permanents untapped and the appended carrier definition in
// exile under a may-play permission (may true) or in hand (may false).
func mpocScen(t *testing.T, pool string, carrier kop.CardDef, cfg Config, may bool, board ...string) (*scen, Obj) {
	t.Helper()
	set := cardsettest.Set(t, pool)
	var deck []cardset.Entry
	seen := map[string]bool{}
	for _, n := range append(board, "Island") {
		if !seen[n] {
			seen[n] = true
			deck = append(deck, cardset.Entry{Name: n, Count: 1})
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
	c := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, carrier)
	if tbl.Exprs == nil {
		tbl.Exprs = []kop.Expr{} // prepareGen builds the generated extension (k.gen non-nil)
	}
	sc := newScenTableLib(t, tbl, id["Island"], cfg)
	for _, n := range board {
		sc.perm(id[n], 0)
	}
	if !may {
		return sc, sc.hand(c, 0)
	}
	o := sc.obj(c, 0)
	s := sc.s
	s.objs[o].zone = kop.ZoneExile
	s.g.may[0] = mayPlay{obj: o, ver: s.objs[o].ver, ctrl: 0, until: uint8(kop.DurEOT), turn: mayWord(s.turn)}
	s.g.nmay = 1
	return sc, o
}

// TestMayPlayOptionalCostLoweringTakesPrefix (CR 601.2g, 605.3a): gorge's
// potential planner composes a may-play cast's cost (potentialModeCastPlan)
// with no "shape:optional_cost" decline, so a cast with an optional
// additional cost (Requiting Hex's blight) that only Springleaf Drum's
// tap-another mana pays is found by the prefix search and offered. A plain
// hand cast is declined at the shape, so it is not lowered.
func TestMayPlayOptionalCostLoweringTakesPrefix(t *testing.T) {
	carrier := kop.CardDef{
		Name: "Test OptionalCost", Types: kop.TCreature,
		Cost:    kop.ManaCost{Generic: 3, Pips: [6]uint8{0, 1, 0, 0, 0, 0}},
		OptCost: kop.Cost{Flags: kop.CostBlight, N: 1},
	}
	for _, may := range []bool{false, true} {
		sc, o := mpocScen(t, "tapall", carrier, Config{GorgeLowering: true, NoElide: true}, may,
			"Springleaf Drum", "Coral Merfolk", "Island", "Island", "Forest")
		s, k := sc.s, sc.k
		if k.gen == nil || !k.gen.anyOptCost || !optCostCard(k, s.objs[o].card) {
			t.Fatalf("mayPlay %v: the carrier is not an OptionalCost card to the generated extension", may)
		}
		if s.plannerPaysCast(k, 0, s.objs[o].card, o) {
			t.Fatalf("mayPlay %v: the census already prices the cast, so the prefix arm is not reached", may)
		}
		s.g.availX |= availGaps // the scripted walk (scriptOffers)
		cost := s.featCost(k, 0, s.objs[o].card, o, k.cards[s.objs[o].card].cost)
		if !s.affordsFor(k, 0, &cost) {
			t.Fatalf("mayPlay %v: the window cannot pay %+v through Springleaf Drum", may, cost)
		}
		if got := s.gorgeLowers(0, o, may); got != may {
			t.Errorf("mayPlay %v: gorgeLowers = %v, want %v (a may-play cast is composed and prefix-searched; a hand cast is declined at its shape)", may, got, may)
		}
	}
}

// TestMayPlayAnyTypeRiderPaysEitherOrTotal (CR 601.2f-h): Demand Answers'
// {1}{R} with an either-or additional cost, cast through Outrageous Robbery's
// any-type permission with {G}{G}{G} floating, is offered: the rider covers the
// total the either-or check recomposes from the unreduced cost, not only the
// spell's own mana gate.
func TestMayPlayAnyTypeRiderPaysEitherOrTotal(t *testing.T) {
	carrier := kop.CardDef{
		Name: "Test EitherOr", Types: kop.TInstant,
		Cost:      kop.ManaCost{Generic: 1, Pips: [6]uint8{0, 0, 0, 1, 0, 0}},
		SpellCost: kop.Cost{Flags: kop.CostDiscard, N: 9}, // unpayable: the other part is the one paid
		HasAltAdd: true,
	}
	sc, o := mpocScen(t, "tapall", carrier, Config{}, true)
	s, k := sc.s, sc.k
	if k.gen == nil {
		t.Fatal("the generated extension is absent")
	}
	if !k.t.Cards[s.objs[o].card].HasAltAdd {
		t.Fatal("the carrier has no either-or additional cost")
	}
	s.pl[0].pool[kop.G] = 3
	var av avail
	s.availFor(k, 0, &av)
	printed := k.cards[s.objs[o].card].cost
	if av.affords(&printed, pipMaskOf(&printed), 0) {
		t.Fatalf("{G}{G}{G} pays %+v without the rider, so the rider is not what is tested", printed)
	}
	any := anyType(printed)
	if !av.affords(&any, pipMaskOf(&any), 0) {
		t.Fatalf("{G}{G}{G} cannot pay %+v even under the rider", any)
	}
	var d Decision
	s.offerPlay(k, &d, 0, o, castMayPlay, false, false, true, false, false, &av)
	if !hasOption(&d, MakeAction(AkCast, TCode(o), 0, castMayPlay)) {
		t.Errorf("the any-type may-play cast is not offered: options %v", d.Options[:d.N])
	}
}
