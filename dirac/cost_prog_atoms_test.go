package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// The cost-program walker's tests: a hand-built table whose one activated
// ability carries Ability.CostProg = 1, so Table.CostProgs[1] pays it. The
// ability's Cost still names what the build's pick stage asks (a sacrifice,
// a discard, tapped creatures, graveyard exiles); the program fixes what is
// paid, how much, and in what order.

type cpScen struct {
	*scen
	trace       []kop.CostAtomKind // the atoms the walker paid, in order
	src, fodder CardID
	creature    kop.FilterID
}

// cpTable is vanilla's table plus an artifact "Test Prog" whose ability 0
// has cost, CostProg 1 and a gain-life body, a creature "Test Fodder" and
// the CostProgs table [nil, prog].
func cpTable(cost kop.Cost, prog []kop.CostAtom, withProgs bool) (*kop.Table, CardID, CardID, kop.FilterID) {
	tbl, f, ids := newModeTable(
		kop.CardDef{Name: "Test Prog", Types: kop.TArtifact, Abilities: []kop.Ability{{
			Kind: kop.AbActivated, Cost: cost, CostProg: 1,
			Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}},
		}}},
		kop.CardDef{Name: "Test Fodder", Types: kop.TCreature, Power: 1, Toughness: 1},
	)
	if withProgs {
		tbl.CostProgs = [][]kop.CostAtom{nil, prog}
	} else {
		tbl.Cards[ids[0]].Abilities[0].CostProg = 0
	}
	return tbl, ids[0], ids[1], f
}

func newCPScen(t *testing.T, cost func(f kop.FilterID) kop.Cost, prog func(f kop.FilterID) []kop.CostAtom) (*cpScen, Obj) {
	t.Helper()
	// the filter id is only known once the table has one: build twice
	_, _, _, f := cpTable(kop.Cost{}, nil, false)
	tbl, src, fodder, f2 := cpTable(cost(f), prog(f), true)
	if f != f2 {
		t.Fatalf("filter id moved: %d vs %d", f, f2)
	}
	sc := &cpScen{scen: newScenTable(t, tbl, Config{NoElide: true}), src: src, fodder: fodder, creature: f}
	costProgTrace = func(k kop.CostAtomKind) { sc.trace = append(sc.trace, k) }
	t.Cleanup(func() { costProgTrace = nil })
	return sc, sc.perm(src, 0)
}

// settle plays the decisions out until the stack is empty, answering every
// ask with its first option and passing priority.
func (sc *cpScen) settle(t *testing.T) {
	t.Helper()
	seen := false
	for i := 0; i < 24; i++ {
		d := sc.s.Step()
		seen = seen || sc.s.sp > 0
		if d.Kind == DkNone || (seen && sc.s.sp == 0 && d.Kind != DkChoose) {
			return
		}
		a := d.Options[0]
		for _, o := range d.Options[:d.N] {
			if o.Kind() == AkPass {
				a = o
			}
		}
		if d.Kind == DkChoose {
			a = d.Options[0]
		}
		sc.s.Apply(a)
	}
	t.Fatal("the activation never settled")
}

// run activates ability 0 of o if it is offered, plays it out and reports
// whether it was offered.
func (sc *cpScen) run(t *testing.T, o Obj) bool {
	t.Helper()
	if !sc.activateAb(o, 0) {
		return false
	}
	sc.settle(t)
	if len(sc.trace) == 0 {
		t.Fatal("the activation was paid without the cost-program walker")
	}
	return true
}

func cpAtoms(as ...kop.CostAtom) func(kop.FilterID) []kop.CostAtom {
	return func(kop.FilterID) []kop.CostAtom { return as }
}

func cpCost(c kop.Cost) func(kop.FilterID) kop.Cost { return func(kop.FilterID) kop.Cost { return c } }

func (sc *cpScen) gone(o Obj, z kop.Zone) bool { return sc.s.objs[o].zone == z }

// TestCostProgPrepared: the program path is armed only by a table that has
// CostProgs, and the table's activation still works without one.
func TestCostProgPrepared(t *testing.T) {
	sc, _ := newCPScen(t, cpCost(kop.Cost{Flags: kop.CostTap}), cpAtoms(kop.CostAtom{K: kop.CostAtomTap}))
	if sc.k.gen == nil || !sc.k.gen.uses.has(useCostProg) {
		t.Fatal("a table with CostProgs did not arm useCostProg")
	}
	tbl, _, _, _ := cpTable(kop.Cost{Flags: kop.CostTap}, nil, false)
	plain := newScenTable(t, tbl, Config{NoElide: true})
	if plain.k.gen != nil && plain.k.gen.uses.has(useCostProg) {
		t.Fatal("a table without CostProgs armed useCostProg")
	}
}

func TestCostProgMana(t *testing.T) {
	mana := kop.ManaCost{Generic: 1}
	for _, pool := range []uint8{0, 1} {
		sc, o := newCPScen(t, cpCost(kop.Cost{Mana: mana}), cpAtoms(kop.CostAtom{K: kop.CostAtomMana}))
		sc.s.pl[0].pool[kop.G] = pool
		if got := sc.run(t, o); got != (pool == 1) {
			t.Fatalf("pool %d: offered %v", pool, got)
		}
		if sc.s.pl[0].pool[kop.G] != 0 && pool == 1 {
			t.Fatalf("the mana atom left %d in the pool", sc.s.pl[0].pool[kop.G])
		}
	}
}

func TestCostProgTap(t *testing.T) {
	sc, o := newCPScen(t, cpCost(kop.Cost{}), cpAtoms(kop.CostAtom{K: kop.CostAtomTap}))
	if !sc.run(t, o) {
		t.Fatal("an untapped source's {T} program is not offered")
	}
	if !sc.tapped(o) {
		t.Fatal("the Tap atom did not tap the source")
	}
	if sc.activateOffered(o, 0) {
		t.Fatal("a tapped source's {T} program is offered")
	}
}

func TestCostProgUntap(t *testing.T) {
	sc, o := newCPScen(t, cpCost(kop.Cost{}), cpAtoms(kop.CostAtom{K: kop.CostAtomUntap}))
	if sc.activateOffered(o, 0) {
		t.Fatal("an untapped source's {Q} program is offered")
	}
	sc.s.perms[sc.slot(o)].flags |= pfTapped
	if !sc.run(t, o) {
		t.Fatal("a tapped source's {Q} program is not offered")
	}
	if sc.tapped(o) {
		t.Fatal("the Untap atom left the source tapped")
	}
}

func TestCostProgSac(t *testing.T) {
	t.Run("self", func(t *testing.T) {
		sc, o := newCPScen(t, cpCost(kop.Cost{}), cpAtoms(kop.CostAtom{K: kop.CostAtomSac}))
		if !sc.run(t, o) || !sc.inGrave(o) {
			t.Fatalf("the Sac atom did not sacrifice the source (zone %d)", sc.s.objs[o].zone)
		}
	})
	t.Run("another", func(t *testing.T) {
		var f kop.FilterID
		sc, o := newCPScen(t, func(fl kop.FilterID) kop.Cost { f = fl; return kop.Cost{Flags: kop.CostSac, N: 1, Filter: fl} },
			func(fl kop.FilterID) []kop.CostAtom {
				return []kop.CostAtom{{K: kop.CostAtomSac, N: kop.Const(1), F: fl}}
			})
		if sc.activateOffered(o, 0) {
			t.Fatal("offered with no creature to sacrifice")
		}
		bear := sc.perm(sc.fodder, 0)
		if f == 0 || !sc.run(t, o) {
			t.Fatal("not offered with a creature to sacrifice")
		}
		if !sc.inGrave(bear) || !sc.onBF(o) {
			t.Fatalf("the Sac atom: fodder zone %d, source on battlefield %v", sc.s.objs[bear].zone, sc.onBF(o))
		}
	})
}

func TestCostProgExileFromZone(t *testing.T) {
	t.Run("graveyard", func(t *testing.T) {
		sc, o := newCPScen(t, cpCost(kop.Cost{Flags: kop.CostExileGrave, N: 1}),
			cpAtoms(kop.CostAtom{K: kop.CostAtomExileFromZone, N: kop.Const(1), Z: kop.ZoneGraveyard}))
		if sc.activateOffered(o, 0) {
			t.Fatal("offered with an empty graveyard")
		}
		g := sc.grave(v.GrizzlyBears, 0, 1)
		if !sc.run(t, o) || !sc.gone(g[0], kop.ZoneExile) {
			t.Fatalf("the ExileFromZone atom left the card in zone %d", sc.s.objs[g[0]].zone)
		}
	})
	t.Run("self", func(t *testing.T) {
		sc, o := newCPScen(t, cpCost(kop.Cost{}),
			cpAtoms(kop.CostAtom{K: kop.CostAtomExileFromZone, Z: kop.ZoneBattlefield}))
		if !sc.run(t, o) || !sc.gone(o, kop.ZoneExile) {
			t.Fatalf("the source is in zone %d", sc.s.objs[o].zone)
		}
	})
}

func TestCostProgDiscard(t *testing.T) {
	sc, o := newCPScen(t, cpCost(kop.Cost{Flags: kop.CostDiscard, N: 1}),
		cpAtoms(kop.CostAtom{K: kop.CostAtomDiscard, N: kop.Const(1)}))
	for sc.s.pl[0].handN > 0 {
		sc.s.detach(sc.s.pl[0].hand[0])
	}
	if sc.activateOffered(o, 0) {
		t.Fatal("offered with an empty hand")
	}
	c := sc.hand(v.GrizzlyBears, 0)
	if !sc.run(t, o) || !sc.inGrave(c) {
		t.Fatalf("the Discard atom left the card in zone %d", sc.s.objs[c].zone)
	}
}

func TestCostProgPayLife(t *testing.T) {
	sc, o := newCPScen(t, cpCost(kop.Cost{}), cpAtoms(kop.CostAtom{K: kop.CostAtomPayLife, N: kop.Const(2)}))
	sc.s.pl[0].life = 1
	if sc.activateOffered(o, 0) {
		t.Fatal("offered at 1 life")
	}
	sc.s.pl[0].life = 20
	if !sc.run(t, o) || sc.s.pl[0].life != 19 { // 20 - 2 paid + 1 gained
		t.Fatalf("life %d, want 19", sc.s.pl[0].life)
	}
}

func TestCostProgSubCounter(t *testing.T) {
	sc, o := newCPScen(t, cpCost(kop.Cost{}),
		cpAtoms(kop.CostAtom{K: kop.CostAtomSubCounter, N: kop.Const(2), Ctr: kop.CtrCharge}))
	p := &sc.s.perms[sc.slot(o)]
	p.setCtrN(sc.s, int(kop.CtrCharge), 1)
	if sc.activateOffered(o, 0) {
		t.Fatal("offered with one counter of two")
	}
	p.setCtrN(sc.s, int(kop.CtrCharge), 3)
	if !sc.run(t, o) || p.ctrN(sc.s, int(kop.CtrCharge)) != 1 {
		t.Fatalf("counters %d, want 1", p.ctrN(sc.s, int(kop.CtrCharge)))
	}
}

func TestCostProgAddCounter(t *testing.T) {
	sc, o := newCPScen(t, cpCost(kop.Cost{}),
		cpAtoms(kop.CostAtom{K: kop.CostAtomAddCounter, N: kop.Const(2), Ctr: kop.CtrCharge}))
	p := &sc.s.perms[sc.slot(o)]
	if !sc.run(t, o) || p.ctrN(sc.s, int(kop.CtrCharge)) != 2 {
		t.Fatalf("counters %d, want 2", p.ctrN(sc.s, int(kop.CtrCharge)))
	}
}

func TestCostProgDraw(t *testing.T) {
	sc, o := newCPScen(t, cpCost(kop.Cost{}), cpAtoms(kop.CostAtom{K: kop.CostAtomDraw, N: kop.Const(2)}))
	h := sc.s.pl[0].handN
	if !sc.run(t, o) || int(sc.s.pl[0].handN) != int(h)+2 {
		t.Fatalf("hand %d, want %d", sc.s.pl[0].handN, h+2)
	}
}

// TestCostProgReveal: the self-reveal atom is payable exactly where
// CostRevealSelf's offer gate says (costTokenPartsPayable) and moves nothing.
func TestCostProgReveal(t *testing.T) {
	sc, o := newCPScen(t, cpCost(kop.Cost{}), cpAtoms(kop.CostAtom{K: kop.CostAtomReveal}))
	a := &sc.k.t.Cards[sc.src].Abilities[0]
	want := sc.s.costTokenPartsPayable(sc.k, 0, o, &kop.Cost{RevealSelf: true})
	if got := sc.s.canPayCostProg(sc.k, 0, sc.slot(o), a); got != want {
		t.Fatalf("canPayCostProg %v, the RevealSelf gate says %v", got, want)
	}
	if want && (!sc.run(t, o) || !sc.onBF(o)) {
		t.Fatal("the Reveal atom moved the source")
	}
}

func TestCostProgReturn(t *testing.T) {
	sc, o := newCPScen(t, cpCost(kop.Cost{}), cpAtoms(kop.CostAtom{K: kop.CostAtomReturn}))
	if !sc.run(t, o) || !sc.gone(o, kop.ZoneHand) {
		t.Fatalf("the Return atom left the source in zone %d", sc.s.objs[o].zone)
	}
}

func TestCostProgTapType(t *testing.T) {
	sc, o := newCPScen(t, func(f kop.FilterID) kop.Cost { return kop.Cost{Flags: kop.CostTapX, N: 1, Filter: f} },
		func(f kop.FilterID) []kop.CostAtom {
			return []kop.CostAtom{{K: kop.CostAtomTapType, N: kop.Const(1), F: f}}
		})
	if sc.activateOffered(o, 0) {
		t.Fatal("offered with no creature to tap")
	}
	bear := sc.perm(sc.fodder, 0)
	if !sc.run(t, o) || !sc.tapped(bear) {
		t.Fatal("the TapType atom did not tap the creature")
	}
}

func TestCostProgMill(t *testing.T) {
	sc, o := newCPScen(t, cpCost(kop.Cost{}), cpAtoms(kop.CostAtom{K: kop.CostAtomMill, N: kop.Const(2)}))
	g := sc.s.pl[0].graveN
	if !sc.run(t, o) || int(sc.s.pl[0].graveN) != int(g)+2 {
		t.Fatalf("graveyard %d, want %d", sc.s.pl[0].graveN, g+2)
	}
}

func TestCostProgExert(t *testing.T) {
	sc, o := newCPScen(t, cpCost(kop.Cost{}), cpAtoms(kop.CostAtom{K: kop.CostAtomExert}))
	if !sc.run(t, o) || sc.s.perms[sc.slot(o)].flags&pfSkipUntap == 0 {
		t.Fatal("the Exert atom did not mark the source")
	}
}

// TestCostProgDeferredAtomsAreNotOffered: an atom kind the kernel has no
// payer for yet (Behold, ExiledMoveToGrave) is never payable, so the ability
// is never offered rather than offered and paid short.
func TestCostProgDeferredAtomsAreNotOffered(t *testing.T) {
	for _, k := range []kop.CostAtomKind{kop.CostAtomBehold, kop.CostAtomExiledMoveToGrave} {
		sc, o := newCPScen(t, cpCost(kop.Cost{}), cpAtoms(kop.CostAtom{K: k, N: kop.Const(1)}))
		if sc.activateOffered(o, 0) {
			t.Fatalf("atom kind %d is offered", k)
		}
	}
}

// TestCostProgOrderMatchesGorge: the walker pays in the order of gorge's
// payCast ability arm (engine/rules/cast_commit.go, cast_asks.go) whatever
// order the program lists the atoms in: the tapped creatures at their ask,
// then life, {T}, discards, exiles, the mill, draws, the return, {Q}, the
// counters, the exert and the sacrifice last.
func TestCostProgOrderMatchesGorge(t *testing.T) {
	want := []kop.CostAtomKind{
		kop.CostAtomTapType, kop.CostAtomMana, kop.CostAtomPayLife, kop.CostAtomTap, kop.CostAtomDiscard,
		kop.CostAtomExileFromZone, kop.CostAtomMill, kop.CostAtomDraw, kop.CostAtomReturn, kop.CostAtomReveal,
		kop.CostAtomUntap, kop.CostAtomSubCounter, kop.CostAtomAddCounter, kop.CostAtomExert, kop.CostAtomSac,
	}
	// the program lists them reversed
	var prog []kop.CostAtom
	for i := len(want) - 1; i >= 0; i-- {
		prog = append(prog, kop.CostAtom{K: want[i]})
	}
	var got []kop.CostAtomKind
	for _, i := range costProgOrder(prog) {
		got = append(got, prog[i].K)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("order %v, want gorge's %v", got, want)
	}

	// and the walker runs them in that order: Sac, AddCounter, PayLife and Tap
	// listed backwards
	sc, o := newCPScen(t, cpCost(kop.Cost{}),
		cpAtoms(kop.CostAtom{K: kop.CostAtomSac}, kop.CostAtom{K: kop.CostAtomAddCounter, N: kop.Const(1), Ctr: kop.CtrCharge},
			kop.CostAtom{K: kop.CostAtomTap}, kop.CostAtom{K: kop.CostAtomPayLife, N: kop.Const(1)}))
	if !sc.run(t, o) {
		t.Fatal("not offered")
	}
	wantRun := []kop.CostAtomKind{kop.CostAtomPayLife, kop.CostAtomTap, kop.CostAtomAddCounter, kop.CostAtomSac}
	if !slices.Equal(sc.trace, wantRun) {
		t.Fatalf("paid %v, want %v", sc.trace, wantRun)
	}
}

// TestCostProgOffer: offerCostProg is canPayCostProg for a card being cast: a
// discard leaves the cast card out, life and the graveyard price as they do on
// an activation, and an atom that works on a permanent source is unpayable.
func TestCostProgOffer(t *testing.T) {
	offer := func(t *testing.T, at kop.CostAtom, setup func(sc *cpScen, cast Obj)) bool {
		t.Helper()
		sc, _ := newCPScen(t, cpCost(kop.Cost{}), cpAtoms(at))
		cast := sc.hand(sc.src, 0)
		if setup != nil {
			setup(sc, cast)
		}
		return sc.s.offerCostProg(sc.k, 0, cast, &sc.k.t.Cards[sc.src].Abilities[0])
	}
	empty := func(sc *cpScen, cast Obj) {
		for i := 0; i < int(sc.s.pl[0].handN); {
			if o := sc.s.pl[0].hand[i]; o != cast {
				sc.s.detach(o)
			} else {
				i++
			}
		}
	}
	disc := kop.CostAtom{K: kop.CostAtomDiscard, N: kop.Const(1)}
	if offer(t, disc, empty) {
		t.Error("a discard was offered with only the cast card in hand")
	}
	if !offer(t, disc, func(sc *cpScen, cast Obj) { empty(sc, cast); sc.hand(v.GrizzlyBears, 0) }) {
		t.Error("a discard was not offered with another card in hand")
	}
	life := kop.CostAtom{K: kop.CostAtomPayLife, N: kop.Const(3)}
	if offer(t, life, func(sc *cpScen, _ Obj) { sc.s.pl[0].life = 2 }) || !offer(t, life, nil) {
		t.Error("pay 3 life is not priced by the payer's life")
	}
	for _, k := range []kop.CostAtomKind{kop.CostAtomTap, kop.CostAtomUntap, kop.CostAtomSubCounter, kop.CostAtomExert, kop.CostAtomReturn} {
		if offer(t, kop.CostAtom{K: k, N: kop.Const(1)}, nil) {
			t.Errorf("atom kind %d works on a permanent source and was offered for a card", k)
		}
	}
}
