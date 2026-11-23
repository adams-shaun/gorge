package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMayPlayIgnoreColorCensusPricesTheRider (CR 601.2f, 609.4b): gorge's
// potential planner folds a may-play cast's MayPlayIgnoreColor$ rider into
// the price before its census reads it (potential_plan.go's MayPlayRider
// arm, pay.PlanManaUnderRider), so a {4}{B} creature exiled under an
// ignore-colour permission is priced {5} and offered over five Islands,
// which pay no black. A gate that prices the census strictly drops the cast
// gorge prices (review r2's probe).
func TestMayPlayIgnoreColorCensusPricesTheRider(t *testing.T) {
	carrier := kop.CardDef{
		Name: "Test IgnoreColor Cast", Types: kop.TCreature,
		Cost: kop.ManaCost{Generic: 4, Pips: [6]uint8{0, 0, 1, 0, 0, 0}}, // {4}{B}
	}
	sc, o := mpocScen(t, "tapall", carrier, Config{GorgeLowering: true, NoElide: true}, true,
		"Island", "Island", "Island", "Island", "Island")
	s, k := sc.s, sc.k
	c := s.objs[o].card
	printed := k.cards[c].cost
	var av avail
	s.availFor(k, 0, &av)
	if av.affords(&printed, pipMaskOf(&printed), 0) {
		t.Fatalf("five Islands pay %+v: the rider is not what prices the cast", printed)
	}
	folded := s.mayRiderCost(printed, true)
	if folded.Generic != 5 || folded.MV() != 5 {
		t.Fatalf("the rider fold produced %+v, want {5}", folded)
	}
	if !av.affords(&folded, pipMaskOf(&folded), 0) {
		t.Fatalf("five Islands cannot pay the rider-folded %+v", folded)
	}
	if s.gorgeLowersRider(0, o, true, false) {
		t.Fatal("the strict gate prices the cast: the rider is not what is tested")
	}
	if !s.gorgeLowersRider(0, o, true, true) {
		t.Fatal("the rider gate prices the census strictly: gorge's planner folds MayPlayIgnoreColor$ before its census and prices {5} (pay.PlanManaUnderRider; CR 609.4b)")
	}
	var d Decision
	s.offerPlay(k, &d, 0, o, castMayPlay, true, false, false, true, false, &av)
	if !hasOption(&d, MakeAction(AkCast, TCode(o), 0, castMayPlay)) {
		t.Errorf("the ignore-colour may-play cast is not offered: options %v", d.Options[:d.N])
	}
	var d2 Decision
	s.offerPlay(k, &d2, 0, o, castMayPlay, true, false, false, false, false, &av)
	if hasOption(&d2, MakeAction(AkCast, TCode(o), 0, castMayPlay)) {
		t.Error("the strict walk offers the {4}{B} cast, which five Islands cannot pay")
	}
}

// TestMayPlayIgnoreColorPrefixPricesTheRider (CR 601.2g, 609.4b): the prefix
// read prices the rider too. Battery Bearer's grant gives Ornithopter
// "{T}: Add {C}", so the prefix window of a {1}{U} artifact creature exiled
// under an ignore-colour permission pays the folded {2} (the Plains' white
// plus the granted colourless) where the strict {1}{U} stays unpayable: no
// blue source and a colourless grant, as the reviewer's probe's
// rider-folded census read.
func TestMayPlayIgnoreColorPrefixPricesTheRider(t *testing.T) {
	set := cardsettest.Set(t, "grantedrestrict")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		id[tbl.Cards[c].Name] = CardID(c)
	}
	carrier := kop.CardDef{
		Name: "Test IgnoreColor Artifact", Types: kop.TArtifact | kop.TCreature,
		Cost: kop.ManaCost{Generic: 1, Pips: [6]uint8{0, 1, 0, 0, 0, 0}}, // {1}{U}
	}
	c := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, carrier)
	if tbl.Exprs == nil {
		tbl.Exprs = []kop.Expr{} // prepareGen builds the generated extension (k.gen non-nil)
	}
	sc := newScenTableLib(t, tbl, id["Plains"], Config{GorgeLowering: true, NoElide: true})
	sc.perm(id["Battery Bearer"], 0)
	orn := sc.perm(id["Ornithopter"], 0)
	sc.perm(id["Plains"], 0)
	if g := sc.s.grantIdx(sc.k, sc.slot(orn)); g < 0 || sc.k.gen.grantRestr[g] == 0 {
		t.Fatalf("precondition: Ornithopter holds no restricted granted mana ability (grant %d)", g)
	}
	o := sc.obj(c, 0)
	s, k := sc.s, sc.k
	s.objs[o].zone = kop.ZoneExile
	s.g.may[0] = mayPlay{obj: o, ver: s.objs[o].ver, ctrl: 0, until: uint8(kop.DurEOT), turn: mayWord(s.turn)}
	s.g.nmay = 1
	s.g.availX |= availGaps // the scripted walk (scriptOffers)

	printed := k.cards[c].cost
	folded := s.mayRiderCost(printed, true)
	if folded.MV() != 2 {
		t.Fatalf("the rider fold produced %+v, want {2}", folded)
	}
	if s.ungrantedPaysNoDH(k, 0, c, o, &folded) {
		t.Fatal("the census pays the folded {2} from the lone Plains: the prefix arm is not the gate's deciding read")
	}
	if !s.mayPlayPrefixPays(k, 0, c, o, true) {
		t.Fatal("the rider gate prices the prefix strictly: gorge's search taps Battery Bearer's granted {C} toward the folded {2} (pay.PlanManaUnderRider; CR 609.4b)")
	}
	if s.mayPlayPrefixPays(k, 0, c, o, false) {
		t.Fatal("the strict prefix pays {1}{U}, which no blue source funds")
	}
	if !s.gorgeLowersRider(0, o, true, true) {
		t.Fatal("the rider gate drops the cast its prefix search prices")
	}
	if s.gorgeLowersRider(0, o, true, false) {
		t.Fatal("the strict gate prices the cast: the rider is not what is tested")
	}
}
