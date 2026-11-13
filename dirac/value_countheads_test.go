package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

func kcnt9Scen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 12}}
	set := cardsettest.Set(t, "countmod")
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in the countmod table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{})
	id["T Filter"] = CardID(len(tbl.Filters) - 1)
	return newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true}), id
}

// TestCycledCardReadsX: a cycled card's own Count$xPaid trigger reads the X
// announced for the cycling activation (gorge trigger_referents.go
// e.cast.x), carried by cycleDiscard into the pending entry; a cycle with
// X 0 reads 0. Gorge's planner prices no {X} activation, so the lockstep
// never offers an X cycle (the pool is never floating): the kernel path is
// pinned here.
func TestCycledCardReadsX(t *testing.T) {
	for _, name := range []string{"Valor's Flagship", "Webstrike Elite"} {
		for _, x := range []uint8{0, 3} {
			sc, id := kcnt9Scen(t, name)
			o := sc.hand(id[name], 0)
			sc.s.cycleDiscard(sc.k, o, x)
			if sc.s.npend != 1 {
				t.Fatalf("%s: %d pending triggers after the cycle, want 1", name, sc.s.npend)
			}
			if got := sc.s.pend[0].x; got != x {
				t.Errorf("%s cycled with X %d: the trigger's bound X is %d", name, x, got)
			}
		}
	}
}

// TestModExpr: ExMod is Sub[0] % N with the dividend's sign (gorge
// count_operand.go applyCountOp); DebugModOff returns the operand.
func TestModExpr(t *testing.T) {
	sc, _ := kcnt9Scen(t)
	for _, c := range []struct{ v, n, want int }{{0, 2, 0}, {1, 2, 1}, {2, 2, 0}, {5, 2, 1}, {7, 3, 1}, {-5, 2, -1}} {
		x := kop.Expr{Kind: kop.ExMod, N: int16(c.n), Sub: [2]kop.ValueRef{kop.Const(c.v)}}
		if got := sc.s.exprCold(sc.k, &x, evalCtx{src: NoObj, srcSl: NoSlot}); got != c.want {
			t.Errorf("%d %% %d = %d, want %d", c.v, c.n, got, c.want)
		}
	}
	DebugModOff = true
	defer func() { DebugModOff = false }()
	x := kop.Expr{Kind: kop.ExMod, N: 2, Sub: [2]kop.ValueRef{kop.Const(5)}}
	if got := sc.s.exprCold(sc.k, &x, evalCtx{src: NoObj, srcSl: NoSlot}); got != 5 {
		t.Errorf("DebugModOff: %d, want 5", got)
	}
}

// TestGraveGreatestPower: ExGraveAgg AggGreatestPower is the greatest printed
// power among the matching graveyard cards, 0 when none match.
func TestGraveGreatestPower(t *testing.T) {
	sc, id := kcnt9Scen(t, "Grizzly Bears", "Hill Giant")
	x := kop.Expr{Kind: kop.ExGraveAgg, N: kop.AggGreatestPower, F: kop.FilterID(id["T Filter"])}
	read := func() int { return sc.s.exprCold(sc.k, &x, evalCtx{ctrl: 0, src: NoObj, srcSl: NoSlot}) }
	if got := read(); got != 0 {
		t.Fatalf("empty graveyards: %d, want 0", got)
	}
	sc.s.place(sc.obj(id["Grizzly Bears"], 0), kop.ZoneGraveyard, false)
	if got := read(); got != 2 {
		t.Errorf("Grizzly Bears: %d, want 2", got)
	}
	sc.s.place(sc.obj(id["Hill Giant"], 1), kop.ZoneGraveyard, false)
	if got := read(); got != 3 {
		t.Errorf("Grizzly Bears and the opponent's Hill Giant: %d, want 3", got)
	}
}

// TestValidAggCountHeads: the ExValidAgg folds (gorge
// count_zone_fold.go): distinct names (CR 201.2), distinct card types on the
// face (CR 205.2a) and the total of one counter kind (CR 122.1).
func TestValidAggCountHeads(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	fdnSet := cardsettest.Set(t, "fdn")
	tbl, _, _, err := fdnSet.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	n := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "T Forest", Types: kop.TLand},
		kop.CardDef{Name: "T Island", Types: kop.TLand},
		kop.CardDef{Name: "T Nursery", Types: kop.TLand},
		kop.CardDef{Name: "Plant", Types: kop.TCreature, Power: 0, Toughness: 1},
		kop.CardDef{Name: "Fractal", Types: kop.TCreature, Power: 0, Toughness: 0},
		kop.CardDef{Name: "T Golem", Types: kop.TArtifact | kop.TCreature, Power: 1, Toughness: 1},
		kop.CardDef{Name: "T Aura", Types: kop.TEnchantment},
		kop.CardDef{Name: "T Bitterblossom", Types: kop.TEnchantment | kop.TKindred},
		kop.CardDef{Name: "T Tarfire", Types: kop.TInstant | kop.TKindred})
	forest, island, nursery, plant, fractal, golem, aura := n, n+1, n+2, n+3, n+4, n+5, n+6
	blossom, tarfire := n+7, n+8
	tbl.Filters = append(tbl.Filters, kop.Filter{Ctrl: kop.CtrlYou})
	f := kop.FilterID(len(tbl.Filters) - 1)
	newSc := func() (*scen, func(int16, kop.CounterKind) int) {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		return sc, func(agg int16, ck kop.CounterKind) int {
			x := kop.Expr{Kind: kop.ExValidAgg, N: agg, F: f, Ctr: ck}
			return sc.s.exprCold(sc.k, &x, evalCtx{ctrl: 0, src: NoObj, srcSl: NoSlot})
		}
	}

	sc, agg := newSc()
	if agg(kop.AggDistinctNames, 0) != 0 || agg(kop.AggCardTypes, 0) != 0 || agg(kop.AggSumCounters, kop.CtrP1P1) != 0 {
		t.Fatal("no match must read 0")
	}
	sc.perm(forest, 0)
	sc.perm(forest, 0)
	sc.perm(island, 0)
	sc.perm(nursery, 1) // the opponent's does not match YouCtrl
	if got := agg(kop.AggDistinctNames, 0); got != 2 {
		t.Errorf("two Forests and an Island: %d distinct names, want 2", got)
	}
	sc.perm(nursery, 0)
	if got := agg(kop.AggDistinctNames, 0); got != 3 {
		t.Errorf("with a Nursery: %d distinct names, want 3", got)
	}

	sc, agg = newSc()
	sc.perm(plant, 0)
	sc.perm(plant, 0)
	if got := agg(kop.AggDistinctNames, 0); got != 1 {
		t.Errorf("two Plants: %d distinct names, want 1", got)
	}
	sc.perm(fractal, 0)
	if got := agg(kop.AggDistinctNames, 0); got != 2 {
		t.Errorf("two Plants and a Fractal: %d distinct names, want 2", got)
	}

	sc, agg = newSc()
	sc.perm(golem, 0)
	sc.perm(aura, 0)
	if got := agg(kop.AggCardTypes, 0); got != 3 {
		t.Errorf("artifact creature + enchantment: %d card types, want 3", got)
	}

	// Kindred is a card type (CR 205.2a, 308.3; gorge): a Kindred
	// Enchantment adds 2 on the battlefield
	sc, agg = newSc()
	sc.perm(blossom, 0)
	if got := agg(kop.AggCardTypes, 0); got != 2 {
		t.Errorf("kindred enchantment: %d card types, want 2", got)
	}

	// the graveyard twin (ExGraveAgg): a Kindred Instant adds 2 card types,
	// and 0 permanent types (CR 110.4: neither Kindred nor Instant is one)
	sc, _ = newSc()
	sc.grave(tarfire, 0, 1)
	gagg := func(agg int16) int {
		x := kop.Expr{Kind: kop.ExGraveAgg, N: agg, F: f}
		return sc.s.exprCold(sc.k, &x, evalCtx{ctrl: 0, src: NoObj, srcSl: NoSlot})
	}
	if got := gagg(kop.AggCardTypes); got != 2 {
		t.Errorf("kindred instant in the graveyard: %d card types, want 2", got)
	}
	if got := gagg(kop.AggPermTypes); got != 0 {
		t.Errorf("kindred instant in the graveyard: %d permanent types, want 0", got)
	}

	// a land animated (CR 205.1b layer-derived creature) still has only its
	// face's Land type: gorge counts o.Face().Types
	sc, agg = newSc()
	o := sc.perm(forest, 0)
	sl := sc.slot(o)
	sc.s.perms[sl].ovr.flags |= ovTypes
	sc.s.perms[sl].ovr.types = kop.TLand | kop.TCreature
	if got := sc.s.typesAt(sc.k, sl); got&kop.TCreature == 0 {
		t.Fatalf("setup: the animated land reads types %b", got)
	}
	if got := agg(kop.AggCardTypes, 0); got != 1 {
		t.Errorf("animated land: %d card types, want 1 (the face's Land)", got)
	}

	// a face-down permanent has no card types (CR 708.2)
	sc, agg = newSc()
	g := sc.perm(golem, 0)
	sc.s.perms[sc.slot(g)].gty |= gtyFaceDown
	if got := agg(kop.AggCardTypes, 0); got != 0 {
		t.Errorf("face-down golem: %d card types, want 0", got)
	}

	sc, agg = newSc()
	a, b := sc.perm(forest, 0), sc.perm(island, 0)
	sc.s.perms[sc.slot(a)].setCtrN(sc.s, int(kop.CtrP1P1), 2)
	sc.s.perms[sc.slot(b)].setCtrN(sc.s, int(kop.CtrP1P1), 3)
	if got := agg(kop.AggSumCounters, kop.CtrP1P1); got != 5 {
		t.Errorf("P1P1 2+3: %d, want 5", got)
	}
	if got := agg(kop.AggSumCounters, kop.CtrM1M1); got != 0 {
		t.Errorf("M1M1 over P1P1 lands: %d, want 0", got)
	}
}

// warpScen is a scenario over the warp pool with the named cards and Plains.
func warpScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 12}}
	warpSet := cardsettest.Set(t, "warp")
	for _, n := range names {
		if i, ok := warpSet.Lookup(n); !ok || !warpSet.Supported()[i] {
			t.Fatalf("%s not supported in the warp table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := warpSet.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	id["T Rock"] = CardID(len(tbl.Cards)) // a free artifact: the trigger's graveyard target
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "T Rock", Types: kop.TArtifact})
	return newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true}), id
}

// castToTrigger casts o (the cast option whose mode has the want bits),
// answers every choice with its first option and passes priority until a
// trigger is on top of the stack.
func (sc *scen) castToTrigger(t *testing.T, o Obj, want uint8) *stackEntry {
	t.Helper()
	d := sc.s.Step()
	cast := false
	for i := 0; i < int(d.N) && !cast; i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(o)) && a.Mode()&want == want {
			sc.s.Apply(a)
			cast = true
		}
	}
	if !cast {
		t.Fatalf("cast of %d (mode %d) not offered: %v", o, want, d.Options[:d.N])
	}
	for i := 0; i < 40; i++ {
		if sc.s.sp > 0 && sc.s.stack[sc.s.sp-1].kind == seTrigger {
			return &sc.s.stack[sc.s.sp-1]
		}
		d := sc.s.Step()
		if d.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		} else {
			sc.s.Apply(d.Options[0])
		}
	}
	t.Fatalf("the enters trigger never reached the stack (sp %d, npend %d, last decision kind %d)", sc.s.sp, sc.s.npend, sc.s.Step().Kind)
	return nil
}

// TestCastSpentBoundToSelfETB: Astelli Reclaimer's enters trigger reads the
// mana spent to cast the entering spell (CR 601.2h; gorge manaSpentTotalsOf
// the source, carried stack to battlefield): 5 hard cast, 3 warped, 0 for a
// permanent that was never cast; the trigger's projected X stays 0 (it has
// no {X}: gorge's stack object carries none).
func TestCastSpentBoundToSelfETB(t *testing.T) {
	for _, c := range []struct {
		name string
		mode uint8
		want uint8
	}{{"hard cast", 0, 5}, {"warp", castWarp, 3}} {
		sc, id := warpScen(t, "Astelli Reclaimer")
		sc.lands(0, id["Plains"], 5)
		sc.s.place(sc.obj(id["T Rock"], 0), kop.ZoneGraveyard, false)
		e := sc.castToTrigger(t, sc.hand(id["Astelli Reclaimer"], 0), c.mode)
		if e.x != c.want {
			t.Errorf("%s: the trigger's bound spend is %d, want %d", c.name, e.x, c.want)
		}
		var p Projection
		sc.s.Project(&p)
		if x := p.Stack[p.NStack-1].X; x != 0 {
			t.Errorf("%s: the trigger's projected X is %d, want 0", c.name, x)
		}
		if got := sc.s.exprCold(sc.k, &kop.Expr{Kind: kop.ExCastManaSpent}, evalCtx{e: e, src: NoObj, srcSl: NoSlot}); got != int(c.want) {
			t.Errorf("%s: ExCastManaSpent on the trigger reads %d, want %d", c.name, got, c.want)
		}
		// the source leaves before the trigger resolves: what it was as it left
		sc.s.place(e.obj, kop.ZoneGraveyard, false)
		if got := sc.s.exprCold(sc.k, &kop.Expr{Kind: kop.ExCastManaSpent}, evalCtx{e: e, src: NoObj, srcSl: NoSlot}); got != int(c.want) {
			t.Errorf("%s: after the source left, ExCastManaSpent reads %d, want %d", c.name, got, c.want)
		}
	}
	sc, id := warpScen(t, "Astelli Reclaimer")
	o := sc.perm(id["Astelli Reclaimer"], 0)
	if got := sc.s.spellSpent(o); got != 0 {
		t.Errorf("a permanent that was never cast binds %d, want 0", got)
	}
}

// TestRememberedValidCount: Remembered$Valid <spec> (gorge evalRefProperty)
// counts the remembered objects that match the spec where they are now (Cache
// Grab's returned card, in hand); a non-matching one reads 0, and the count
// is of the matches, not of the remembered set.
func TestRememberedValidCount(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	fdnSet := cardsettest.Set(t, "fdn")
	tbl, _, _, err := fdnSet.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	n := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "T Squirrel", Types: kop.TCreature, Power: 1, Toughness: 1})
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	f := kop.FilterID(len(tbl.Filters) - 1)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	x := kop.Expr{Kind: kop.ExRefCount, Ref: kop.RefRemembered, F: f}
	read := func() int {
		return sc.s.exprCold(sc.k, &x, evalCtx{ctrl: 0, src: NoObj, srcSl: NoSlot})
	}
	sq, land := sc.hand(n, 0), sc.hand(sc.s.objs[sc.s.pl[0].lib[0]].card, 0) // the library card is a Plains
	sc.s.g.rem[0], sc.s.g.nrem = sq, 1
	if got := read(); got != 1 {
		t.Errorf("a creature card returned to hand: %d, want 1", got)
	}
	sc.s.g.rem[0], sc.s.g.nrem = land, 1
	if got := read(); got != 0 {
		t.Errorf("a land card returned to hand: %d, want 0", got)
	}
	sc.s.g.rem[0], sc.s.g.rem[1], sc.s.g.nrem = land, sq, 2
	if got := read(); got != 1 {
		t.Errorf("a land and a creature remembered: %d, want 1 (the matches only)", got)
	}
	sc.s.g.nrem = 0
	if got := read(); got != 0 {
		t.Errorf("nothing remembered: %d, want 0", got)
	}
}
