package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// newSetScenAutopay is newSetScenTweak on the autopay surface (the kicked
// potential arm runs only there).
func newSetScenAutopay(t *testing.T, deck []cardset.Entry) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, "mbc")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not supported in the mbc table", e.Name)
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
	pl, ok := id["Plains"]
	if !ok {
		t.Fatal("the scenario deck needs Plains")
	}
	var lib [20]CardID
	for i := range lib {
		lib[i] = pl
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true, GorgeAutopaySurface: true})
	for p := uint8(0); p < 2; p++ {
		pls := &s.pl[p]
		for pls.handN > 0 {
			o := pls.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pls.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	return &scen{s: s, k: s.k()}, id
}

// The kicked potential arm's open-production poses (dr-mbc-bbba's Gemhide
// Sliver class): an any-colour source beside Rain of Filth's grants makes the
// over-bound potential pool unbounded, so gorge's planner prices the kicked
// cast through its census there, each gap source's production once. The
// kernel's arm prices the same per-tap census (kickedPotentialPays).

// openCensusScen builds the pose: Saprazzan Skerry with nCtr depletion
// counters (its bounded rider is {T}, remove one: Add {U}{U}), an Island and
// nSwamps Swamps, Rain of Filth's "Sacrifice this land: Add {B}" live on all
// of them (the cast of the Rain pays its {B}, leaving one land tapped),
// Gemhide Sliver (an any-colour source) untapped on the battlefield when
// gemhide, and Caligo Skin-Witch in hand. Every land is grant-viewed, so the
// cast gate's census (the Island, the untapped Swamp, the Gemhide) cannot pay
// the kicked {4}{B}{B}; the potential walk's units (the Skerry's grant and
// rider, the grants on the tapped lands, the Gemhide's one mana) can.
func openCensusScen(t *testing.T, nSwamps, nCtr, nTap int, gemhide bool) (*scen, Obj) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Rain of Filth", Count: 1},
		{Name: "Saprazzan Skerry", Count: 1},
		{Name: "Caligo Skin-Witch", Count: 1},
		{Name: "Island", Count: 1},
		{Name: "Swamp", Count: nSwamps},
		{Name: "Plains", Count: 8},
	}
	if gemhide {
		deck = append(deck, cardset.Entry{Name: "Gemhide Sliver", Count: 1})
	}
	sc, id := newSetScenAutopay(t, deck)
	skerry := sc.perm(id["Saprazzan Skerry"], 0)
	sl := sc.slot(skerry)
	a, ok := sc.s.riderAbilityOf(sc.k, sl)
	if !ok {
		t.Fatal("precondition: Saprazzan Skerry is not the bounded-rider shape")
	}
	pm := &sc.s.perms[sl]
	pm.subCtrN(sc.s, int(a.ManaRiderCtr), pm.ctrN(sc.s, int(a.ManaRiderCtr)))
	if nCtr > 0 && !pm.addCtrN(sc.s, int(a.ManaRiderCtr), nCtr) {
		t.Fatal("precondition: could not put the depletion counters on")
	}
	pm.flags &^= pfTapped
	var swamps []Obj
	for i := 0; i < nSwamps; i++ {
		swamps = append(swamps, sc.perm(id["Swamp"], 0))
	}
	island := sc.perm(id["Island"], 0)
	tappable := append(append([]Obj(nil), swamps...), island)
	if gemhide {
		sc.perm(id["Gemhide Sliver"], 0)
	}
	caligo := sc.hand(id["Caligo Skin-Witch"], 0)
	rof := sc.hand(id["Rain of Filth"], 0)
	sc.do(t, AkCast, TCode(rof), scAny) // an untapped land pays the {B}
	sc.resolveAll(t)
	if sc.tapped(skerry) || sc.tapped(island) {
		t.Fatal("precondition: the Skerry or the Island paid the Rain's {B}")
	}
	// the cast left one land tapped; tap nTap others (their grants still work)
	for _, o := range tappable {
		if nTap == 0 {
			break
		}
		if sc.tapped(o) {
			continue // this one paid the Rain's {B}
		}
		sc.s.perms[sc.slot(o)].flags |= pfTapped
		nTap--
	}
	if nTap > 0 {
		t.Fatal("precondition: not enough untapped lands to tap")
	}
	if gi := sc.s.grantIdx(sc.k, sc.slot(skerry)); gi < 0 {
		t.Fatal("precondition: Rain of Filth's grant is not live on the Skerry")
	}
	return sc, caligo
}

// castModeOffered reports whether the priority decision offers the object's
// cast of the mode (the plain cast's 0, the kicked cast's 1).
func castModeOffered(sc *scen, o Obj, mode uint8) bool {
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == AkCast && Obj(a.Source()) == o && a.Mode() == mode {
			return true
		}
	}
	return false
}

// offeredKickedCast is castModeOffered's kicked cast (the cast action's mode
// bit 1, offerKickedHand's).
func offeredKickedCast(sc *scen, o Obj) bool {
	return castModeOffered(sc, o, 1)
}

// TestKickedOpenCensusOffered: the Gemhide pose (the Island, one untapped
// Swamp, one tapped one, the Skerry rider). The cast gate's census cannot pay
// the kicked {4}{B}{B}, and the per-tap census (the Skerry's grant and rider,
// the Island's, the untapped Swamp's, the grants on the tapped lands, the
// Gemhide's one mana) can, so the kicked cast is offered.
func TestKickedOpenCensusOffered(t *testing.T) {
	sc, caligo := openCensusScen(t, 1, 1, 1, true)
	if !offeredKickedCast(sc, caligo) {
		t.Fatal("the kicked cast is not offered")
	}
}

// TestKickedOpenCensusAllTappedOffered: the same pose with every Swamp and
// the Island tapped. The census (the Skerry's grant and rider and the grants
// on the tapped lands) still pays the kicked cast and it is offered.
func TestKickedOpenCensusAllTappedOffered(t *testing.T) {
	sc, caligo := openCensusScen(t, 2, 1, 2, true)
	if !offeredKickedCast(sc, caligo) {
		t.Fatal("the kicked cast is not offered")
	}
}

// TestKickedOpenCensusShortDeclined: no Skerry rider and no Island; the
// census (the Skerry's grant, the untapped Swamp's and the Gemhide's mana)
// is short of the six the kicked cast needs, so it stays unoffered: the
// bounded census must not accept on the over-bound pool an open production
// leaves unbounded.
func TestKickedOpenCensusShortDeclined(t *testing.T) {
	sc, caligo := openCensusScen(t, 1, 0, 0, true)
	if offeredKickedCast(sc, caligo) {
		t.Fatal("the kicked cast is offered on a census that cannot pay")
	}
}

// TestKickedGrantCensusOffered: without the Gemhide the walk prices no open
// production, so the over-bound pool is the planner's own bound and the same
// board offers the kicked cast through the grants (the pair-98 shape) — a
// regression guard for the bounded census beside it.
func TestKickedGrantCensusOffered(t *testing.T) {
	sc, caligo := openCensusScen(t, 2, 1, 1, false)
	if !offeredKickedCast(sc, caligo) {
		t.Fatal("the kicked cast is not offered through the grants")
	}
}

// TestKickedSecondAbilityIsOneTap: two untapped Ramunap
// Ruins ("{T}: Add {C}" beside "{T}, Pay 1 life: Add {R}") and Agate
// Instigator in hand (plain {1}{R}, Offspring {1}{R}). A source taps once, so
// the census prices each Ruins as one unit of either colour (CR 118.3c): the
// plain {1}{R} is payable ({R} from one Ruins' second ability, {1} from the
// other's {C}), while the offspring {2}{R}{R} needs four mana the two lands
// cannot make and stays unoffered, as gorge's potential planner declines it
// (the second ability is no extra unit: Lupinflower Village's {C} beside its
// restricted {W} made the kernel offer a kicked Intrepid Rabbit gorge did not,
// the BLB repros).
func TestKickedSecondAbilityIsOneTap(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Agate Instigator", Count: 1},
		{Name: "Ramunap Ruins", Count: 2},
		{Name: "Plains", Count: 8},
	}
	sc, id := newSetScenAutopay(t, deck)
	sc.perm(id["Ramunap Ruins"], 0) // perm's second arg is the player: both under p0
	sc.perm(id["Ramunap Ruins"], 0)
	sc.resolveAll(t)
	agate := sc.hand(id["Agate Instigator"], 0)
	if !castModeOffered(sc, agate, 0) {
		t.Fatal("the plain cast is not offered on two Ruins")
	}
	if offeredKickedCast(sc, agate) {
		t.Fatal("the offspring cast is offered on two lands that make two mana")
	}
}
