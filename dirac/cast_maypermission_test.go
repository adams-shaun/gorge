package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// castMaypermissionKindTable is the vanilla table plus one card whose program grants a
// permission kind, so prepareGen sets genPrep.anyMayKind (the bit the hot
// walks guard on) without pulling the generated mayplaykind pool into this package.
func castMaypermissionKindTable() *kop.Table {
	t := v.NewTable()
	// a non-nil expression table makes prepareGen build the generated
	// extension (vanilla alone has none, so genPrep.anyMayKind would not run)
	t.Exprs = []kop.Expr{}
	t.Cards = append(t.Cards, kop.CardDef{
		Name:  "Test Permission Kind",
		Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
			{Op: kop.OpMayPlay, Obj: kop.RefRemembered, A: kop.Const(kop.DurEOT), B: kop.Const(kop.MayPlayAnyType), C: int16(kop.ZoneExile)},
		}}},
	})
	return t
}

// hasMayCast reports whether d offers a may-play cast of o.
func hasMayCast(d *Decision, o Obj) bool {
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == AkCast && Obj(a.Source()) == o && a.Mode()&castMayPlay != 0 {
			return true
		}
	}
	return false
}

// TestMayPlayAnyType: a MayPlayIgnoreType record offers a cast whose
// printed pip the controller's sources cannot pay, and the spent record's
// sfAnyMana makes the pip generic (payable by the off-color sources). A
// record without the kind is not offered, so the rider is the only reason
// the cast is legal here (gorge's pipRider.AnyType).
func TestMayPlayAnyType(t *testing.T) {
	sc := newScenTable(t, castMaypermissionKindTable(), Config{NoElide: true})
	s := sc.s
	// two Mountains cannot pay Think Twice's {U} pip, but pay {2} generic
	sc.perm(v.Mountain, 0)
	sc.perm(v.Mountain, 0)
	o := sc.obj(v.ThinkTwice, 0)
	s.objs[o].zone = kop.ZoneExile
	s.g.may[0] = mayPlay{obj: o, ver: s.objs[o].ver, ctrl: 0, until: uint8(kop.DurEOT), turn: mayWord(s.turn)}
	s.g.nmay = 1

	var d Decision
	var av avail
	s.priorityGen(sc.k, &d, 0, true, &av, false)
	if hasMayCast(&d, o) {
		t.Fatal("a plain permission offered an off-color pip the Mountains cannot pay")
	}

	s.g.may[0].kind = kop.MayPlayAnyType
	d = Decision{}
	s.priorityGen(sc.k, &d, 0, true, &av, false)
	if !hasMayCast(&d, o) {
		t.Fatalf("MayPlayIgnoreType did not offer the cast; options %v", d.Options[:d.N])
	}

	// the cast spends the any-type record and carries sfAnyMana
	*s.bld() = build{e: stackEntry{kind: seSpell, obj: o, card: v.ThinkTwice, ctrl: 0}}
	s.castFlagsGen(sc.k, o, castMayPlay)
	if s.g.nmay != 0 {
		t.Fatalf("the any-type permission was not spent (%d records left)", s.g.nmay)
	}
	if s.bld().e.flags&sfAnyMana == 0 {
		t.Fatal("the spent any-type record did not set sfAnyMana")
	}
	cost, ok := s.buildCostGen(sc.k)
	if !ok {
		t.Fatal("buildCostGen did not price the may-play cast")
	}
	if cost.Pips != [6]uint8{} || cost.Generic != 2 {
		t.Fatalf("any-type cost %+v, want {2} with no pips", cost)
	}
}

// TestMayPlayKindInterp: the non-remembered OpMayPlay path (an object
// reference, not RefRemembered) stores the kind rider too.
func TestMayPlayKindInterp(t *testing.T) {
	sc := newScenTable(t, castMaypermissionKindTable(), Config{NoElide: true})
	s := sc.s
	o := sc.obj(v.ThinkTwice, 0)
	s.objs[o].zone = kop.ZoneExile
	s.res.e.obj, s.res.e.ctrl = o, 0
	s.g.nmay = 0
	in := kop.Instr{Op: kop.OpMayPlay, Obj: kop.RefSelf, A: kop.Const(kop.DurEOT), B: kop.Const(kop.MayPlayFree), C: int16(kop.ZoneExile)}
	if ok, done := s.execGen(sc.k, &in); !ok || !done {
		t.Fatalf("OpMayPlay not handled (%v, %v)", ok, done)
	}
	if s.g.nmay != 1 || s.g.may[0].obj != o || s.g.may[0].kind != kop.MayPlayFree {
		t.Fatalf("permission %d (%v), want one free record for the object", s.g.nmay, s.g.may[:s.g.nmay])
	}
}

// castMaypermissionMayCast spends o through castFlagsGen as a may-play cast and returns
// the entry's priced cost (buildCostGen), failing when it cannot price it.
func castMaypermissionMayCast(t *testing.T, s *State, k *prep, o Obj) kop.ManaCost {
	t.Helper()
	*s.bld() = build{e: stackEntry{kind: seSpell, obj: o, card: s.objs[o].card, ctrl: 0}}
	s.castFlagsGen(k, o, castMayPlay)
	cost, ok := s.buildCostGen(k)
	if !ok {
		t.Fatal("buildCostGen did not price the may-play cast")
	}
	return cost
}

// TestSpendMayKindStaleRecord: CR 400.7. A permission record made for
// the object before it left and returned (same Obj, bumped ver) is stale;
// the fresh any-type record is the one that offers and must be the one
// spent, carrying sfAnyMana. Before the fix the stale plain record was
// spent by the fallback and the any-type rider was lost, so the {U} pip
// stayed unpayable.
func TestSpendMayKindStaleRecord(t *testing.T) {
	sc := newScenTable(t, castMaypermissionKindTable(), Config{NoElide: true})
	s := sc.s
	sc.perm(v.Mountain, 0)
	sc.perm(v.Mountain, 0)
	o := sc.obj(v.ThinkTwice, 0)
	s.place(o, kop.ZoneExile, false)
	stale := s.objs[o].ver
	s.g.may[0] = mayPlay{obj: o, ver: stale, ctrl: 0, until: uint8(kop.DurEOT), turn: mayWord(s.turn)}
	// leave and return: the object is a new object (CR 400.7)
	s.detach(o)
	s.place(o, kop.ZoneGraveyard, false)
	s.detach(o)
	s.place(o, kop.ZoneExile, false)
	if s.objs[o].ver == stale {
		t.Fatal("leave-and-return did not bump the object version")
	}
	s.g.may[1] = mayPlay{obj: o, ver: s.objs[o].ver, ctrl: 0, until: uint8(kop.DurEOT), kind: kop.MayPlayAnyType, turn: mayWord(s.turn)}
	s.g.nmay = 2

	var d Decision
	var av avail
	s.priorityGen(sc.k, &d, 0, true, &av, false)
	if !hasMayCast(&d, o) {
		t.Fatalf("the fresh any-type record did not offer the cast; options %v", d.Options[:d.N])
	}

	cost := castMaypermissionMayCast(t, s, sc.k, o)
	if s.bld().e.flags&sfAnyMana == 0 {
		t.Fatal("the stale plain record was spent: sfAnyMana not set")
	}
	if s.g.nmay != 1 || s.g.may[0].ver != stale {
		t.Fatalf("the stale record was spent (%d records, may[0].ver %d), want the stale one kept", s.g.nmay, s.g.may[0].ver)
	}
	if cost.Pips != [6]uint8{} || cost.Generic != 2 {
		t.Fatalf("any-type cost %+v, want {2} with no pips", cost)
	}
}

// TestSpendMayKindPlainFirst: two live records for the same object, a
// plain one first and the any-type one second. The plain record cannot pay
// the printed {U}, so only the any-type record makes the cast legal; the
// spend must take that record and keep the plain one, not take the first
// match and lose the rider.
func TestSpendMayKindPlainFirst(t *testing.T) {
	sc := newScenTable(t, castMaypermissionKindTable(), Config{NoElide: true})
	s := sc.s
	sc.perm(v.Mountain, 0)
	sc.perm(v.Mountain, 0)
	o := sc.obj(v.ThinkTwice, 0)
	s.objs[o].zone = kop.ZoneExile
	s.g.may[0] = mayPlay{obj: o, ver: s.objs[o].ver, ctrl: 0, until: uint8(kop.DurEOT), turn: mayWord(s.turn)}
	s.g.may[1] = mayPlay{obj: o, ver: s.objs[o].ver, ctrl: 0, until: uint8(kop.DurEOT), kind: kop.MayPlayAnyType, turn: mayWord(s.turn)}
	s.g.nmay = 2

	var d Decision
	var av avail
	s.priorityGen(sc.k, &d, 0, true, &av, false)
	if !hasMayCast(&d, o) {
		t.Fatalf("the any-type record did not offer the cast; options %v", d.Options[:d.N])
	}

	cost := castMaypermissionMayCast(t, s, sc.k, o)
	if s.bld().e.flags&sfAnyMana == 0 {
		t.Fatal("the plain record was spent: sfAnyMana not set")
	}
	if s.g.nmay != 1 || s.g.may[0].kind != 0 {
		t.Fatalf("the any-type record was not the one spent (%d records, may[0].kind %d)", s.g.nmay, s.g.may[0].kind)
	}
	if cost.Pips != [6]uint8{} || cost.Generic != 2 {
		t.Fatalf("any-type cost %+v, want {2} with no pips", cost)
	}
}

// TestSpendMayKindFallbackVer: the plain-record fallback filters stale
// records too. Without a rider record the spend takes the fresh plain
// record, not the stale one ahead of it.
func TestSpendMayKindFallbackVer(t *testing.T) {
	sc := newScenTable(t, castMaypermissionKindTable(), Config{NoElide: true})
	s := sc.s
	o := sc.obj(v.ThinkTwice, 0)
	s.place(o, kop.ZoneExile, false)
	stale := s.objs[o].ver
	s.g.may[0] = mayPlay{obj: o, ver: stale, ctrl: 0, until: uint8(kop.DurEOT), turn: mayWord(s.turn)}
	s.detach(o)
	s.place(o, kop.ZoneGraveyard, false)
	s.detach(o)
	s.place(o, kop.ZoneExile, false)
	s.g.may[1] = mayPlay{obj: o, ver: s.objs[o].ver, ctrl: 0, until: uint8(kop.DurEOT), turn: mayWord(s.turn)}
	s.g.nmay = 2

	castMaypermissionMayCast(t, s, sc.k, o)
	if s.g.nmay != 1 || s.g.may[0].ver != stale {
		t.Fatalf("the stale plain record was spent (%d records, may[0].ver %d), want the stale one kept", s.g.nmay, s.g.may[0].ver)
	}
}

// castMaypermissionAnyColorTable is castMaypermissionKindTable plus a {C}-pip spell and a colourless
// land, for the any-colour rider's colourless-pip strictness.
func castMaypermissionAnyColorTable() (*kop.Table, kop.CardID, kop.CardID) {
	t := castMaypermissionKindTable()
	cspell := kop.CardID(len(t.Cards))
	t.Cards = append(t.Cards, kop.CardDef{
		Name:      "Test Colorless Pip",
		Types:     kop.TSorcery,
		Cost:      kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.C: 1}},
		Timing:    kop.TimingSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell}},
	})
	wastes := kop.CardID(len(t.Cards))
	t.Cards = append(t.Cards, kop.CardDef{
		Name:      "Test Wastes",
		Types:     kop.TLand | kop.TBasic,
		Timing:    kop.TimingLand,
		Abilities: []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColC, ManaN: 1}},
	})
	return t, cspell, wastes
}

// TestMayPlayAnyColor: a MayPlayIgnoreColor record offers a cast whose
// printed coloured pip the controller's sources cannot pay in that colour,
// and pays it with off-colour mana. A plain record does not, and a {C} pip
// stays colourless-only: any colour is not any type (CR 107.4c).
func TestMayPlayAnyColor(t *testing.T) {
	sc := newScenTable(t, castMaypermissionKindTable(), Config{NoElide: true})
	s := sc.s
	// two Mountains cannot pay Think Twice's {U} pip, but pay {1} and, under
	// the rider, the pip too
	sc.perm(v.Mountain, 0)
	sc.perm(v.Mountain, 0)
	o := sc.obj(v.ThinkTwice, 0)
	s.objs[o].zone = kop.ZoneExile
	s.g.may[0] = mayPlay{obj: o, ver: s.objs[o].ver, ctrl: 0, until: uint8(kop.DurEOT), turn: mayWord(s.turn)}
	s.g.nmay = 1

	var d Decision
	var av avail
	s.priorityGen(sc.k, &d, 0, true, &av, false)
	if hasMayCast(&d, o) {
		t.Fatal("a plain permission offered an off-colour pip the Mountains cannot pay")
	}

	s.g.may[0].kind = kop.MayPlayIgnoreColor
	d = Decision{}
	s.priorityGen(sc.k, &d, 0, true, &av, false)
	if !hasMayCast(&d, o) {
		t.Fatalf("MayPlayIgnoreColor did not offer the cast; options %v", d.Options[:d.N])
	}

	// the cast spends the record and carries the build-scoped rider
	*s.bld() = build{e: stackEntry{kind: seSpell, obj: o, card: v.ThinkTwice, ctrl: 0}}
	s.castFlagsGen(sc.k, o, castMayPlay)
	if s.g.nmay != 0 {
		t.Fatalf("the any-colour permission was not spent (%d records left)", s.g.nmay)
	}
	if s.bld().bfl&bflAnyColor == 0 {
		t.Fatal("the spent any-colour record did not set bflAnyColor")
	}
	cost, ok := s.buildCostGen(sc.k)
	if !ok {
		t.Fatal("buildCostGen did not price the may-play cast")
	}
	if cost.Pips[kop.U] != 1 || cost.Generic != 1 {
		t.Fatalf("any-colour cost %+v, want the printed {1}{U}", cost)
	}
	// the rider pays the {U} pip from the Mountains' off-colour mana (this
	// test prices payment only; TestAnyColorPlanUnderRider covers the plan)
	s.payRider(sc.k, 0, &cost, 0, NoObj, true)
	if s.pl[0].pool != [kop.NumManaKinds]uint8{} {
		t.Fatalf("pool after the any-colour payment %v, want empty", s.pl[0].pool)
	}
}

// TestMayPlayAnyColorColorless: {C} is not a colour, so the rider never
// pays it with coloured mana; a colourless source still pays it.
func TestMayPlayAnyColorColorless(t *testing.T) {
	tbl, cspell, wastes := castMaypermissionAnyColorTable()
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	sc.perm(v.Mountain, 0)
	o := sc.obj(cspell, 0)
	s.objs[o].zone = kop.ZoneExile
	s.g.may[0] = mayPlay{obj: o, ver: s.objs[o].ver, ctrl: 0, until: uint8(kop.DurEOT), turn: mayWord(s.turn), kind: kop.MayPlayIgnoreColor}
	s.g.nmay = 1

	var d Decision
	var av avail
	s.priorityGen(sc.k, &d, 0, true, &av, false)
	if hasMayCast(&d, o) {
		t.Fatal("the any-colour rider paid a {C} pip with coloured mana")
	}
	cost := kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.C: 1}}
	s.availFor(sc.k, 0, &av)
	if av.affordsAnyColor(&cost, 0) {
		t.Fatal("affordsAnyColor accepted a Mountain for a {C} pip")
	}

	// a colourless land pays it (and the record offers the cast)
	sc.perm(wastes, 0)
	d = Decision{}
	s.availFor(sc.k, 0, &av)
	s.priorityGen(sc.k, &d, 0, true, &av, false)
	if !hasMayCast(&d, o) {
		t.Fatalf("a colourless source did not pay the {C} pip; options %v", d.Options[:d.N])
	}
	s.payRider(sc.k, 0, &cost, 0, NoObj, true)
}

// castMaypermissionLimitTable is castMaypermissionKindTable plus a permission record with a
// MayPlayLimit cap, so prepareGen sets genPrep.anyMayLimit and anyHist.
func castMaypermissionLimitTable() *kop.Table {
	t := castMaypermissionKindTable()
	t.Cards = append(t.Cards, kop.CardDef{
		Name:  "Test Permission Limit",
		Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
			{Op: kop.OpMayPlay, Obj: kop.RefRemembered, A: kop.Const(kop.DurEOT), B: kop.Const(1 << kop.MayPlayLimitShift), C: int16(kop.ZoneExile)},
		}}},
	})
	return t
}

// TestMayPlayLimit: a limit record is offered while its grant is under
// the cap and withheld at it; the may-play count is per owner (a cast
// of an opponent's card counts for the owner), and a land moved from a
// graveyard or exile onto the battlefield counts too (gorge
// scanMayPlaysThisTurn).
func TestMayPlayLimit(t *testing.T) {
	sc := newScenTable(t, castMaypermissionLimitTable(), Config{NoElide: true})
	s := sc.s
	if !sc.k.gen.anyMayLimit || !sc.k.gen.anyHist {
		t.Fatal("prepareGen did not arm the limit walk and the history bank")
	}
	sc.perm(v.Mountain, 0)
	o := sc.obj(v.RagingGoblin, 0)
	s.objs[o].zone = kop.ZoneExile
	s.g.may[0] = mayPlay{obj: o, ver: s.objs[o].ver, ctrl: 0, until: uint8(kop.DurEOT), turn: mayWord(s.turn), kind: 1 << kop.MayPlayLimitShift}
	s.g.nmay = 1

	var d Decision
	var av avail
	s.priorityGen(sc.k, &d, 0, true, &av, false)
	if !hasMayCast(&d, o) {
		t.Fatalf("the limit record did not offer the first cast; options %v", d.Options[:d.N])
	}
	// the cap is the grant's one use taken
	// through the grant, not the player's may-play count, withholds it
	s.g.may[0].until += mayUntilOne
	d = Decision{}
	s.priorityGen(sc.k, &d, 0, true, &av, false)
	if hasMayCast(&d, o) {
		t.Fatal("the limit record offered a cast at the cap")
	}

	// a may-play cast of an opponent's card counts for its owner, not the
	// caster (gorge's owner filter)
	own := sc.obj(v.ThinkTwice, 1)
	s.g.hist[0][histMayPlay], s.g.hist[1][histMayPlay] = 0, 0
	s.histCast(sc.k, &stackEntry{kind: seSpell, obj: own, ctrl: 0, flags: sfMayPlay}, &kop.ManaCost{})
	if s.g.hist[1][histMayPlay] != 1 || s.g.hist[0][histMayPlay] != 0 {
		t.Fatalf("may-play count %v, want the owner's slot only", s.g.hist)
	}

	// a land from a graveyard or exile counts; from the hand it does not
	s.g.hist[1][histMayPlay] = 0
	land := sc.obj(v.Mountain, 1)
	s.place(land, kop.ZoneGraveyard, false)
	s.countMayPlayLand(sc.k, land, kop.ZoneGraveyard)
	if s.g.hist[1][histMayPlay] != 1 {
		t.Fatalf("graveyard land did not count (%d)", s.g.hist[1][histMayPlay])
	}
	s.countMayPlayLand(sc.k, land, kop.ZoneHand)
	if s.g.hist[1][histMayPlay] != 1 {
		t.Fatal("a land from the hand counted")
	}
	// a nonland does not count
	nonland := sc.obj(v.ThinkTwice, 1)
	s.place(nonland, kop.ZoneExile, false)
	s.countMayPlayLand(sc.k, nonland, kop.ZoneExile)
	if s.g.hist[1][histMayPlay] != 1 {
		t.Fatal("a nonland counted")
	}
}

// TestAnyColorPlanUnderRider: the may-play any-colour rider reaches the plan
// (gorge's planCastPaymentChecked, pay.PlanManaUnderRider), so a {R} cast is {1}
// to it and the first source pays, whichever colour it makes: the Island when it
// enters first, the Mountain when it does.
func TestAnyColorPlanUnderRider(t *testing.T) {
	for _, islandFirst := range []bool{true, false} {
		b10AnyColorPlan(t, islandFirst)
	}
}

func b10AnyColorPlan(t *testing.T, islandFirst bool) {
	sc := newScenTable(t, castMaypermissionKindTable(), Config{NoElide: true})
	s := sc.s
	var isl, mtn Obj
	if islandFirst {
		isl = sc.perm(v.Island, 0)
		mtn = sc.perm(v.Mountain, 0)
	} else {
		mtn = sc.perm(v.Mountain, 0)
		isl = sc.perm(v.Island, 0)
	}
	o := sc.obj(v.RagingGoblin, 0) // {R}
	s.place(o, kop.ZoneExile, false)
	s.g.may[0] = mayPlay{obj: o, ver: s.objs[o].ver, ctrl: 0, until: uint8(kop.DurEOT), turn: mayWord(s.turn), kind: kop.MayPlayIgnoreColor}
	s.g.nmay = 1
	*s.bld() = build{e: stackEntry{kind: seSpell, obj: o, card: v.RagingGoblin, ctrl: 0}}
	s.castFlagsGen(sc.k, o, castMayPlay)
	if s.bld().bfl&bflAnyColor == 0 {
		t.Fatal("the spent any-colour record did not set bflAnyColor")
	}
	s.sp, s.stack[0] = 1, s.bld().e
	s.finishBuild(sc.k)
	if sc.tapped(isl) != islandFirst || sc.tapped(mtn) == islandFirst {
		t.Fatalf("Island first %v: tapped Island=%v Mountain=%v; want the first source alone", islandFirst, sc.tapped(isl), sc.tapped(mtn))
	}
	if s.pl[0].pool != [kop.NumManaKinds]uint8{} {
		t.Fatalf("pool after the plan %v, want empty", s.pl[0].pool)
	}
}

// TestAnyColorHybrid: the any-colour rider prices a hybrid pip through
// affordsAnyColor and pays it through the hybrid witness, then the rider
// (gorge's expandCostPips anyColorAlts + hybResolveRider).
func TestAnyColorHybrid(t *testing.T) {
	sc := newScenTable(t, castMaypermissionKindTable(), Config{NoElide: true})
	s := sc.s
	mtn := sc.perm(v.Mountain, 0)
	hyb := kop.ManaCost{Hyb: [4]kop.ColorMask{1<<kop.W | 1<<kop.U}}
	var av avail
	s.availFor(sc.k, 0, &av)
	if av.affords(&hyb, pipMaskOf(&hyb), 0) {
		t.Fatal("a Mountain paid a {W/U} pip without the rider")
	}
	if !av.affordsAnyColor(&hyb, 0) {
		t.Fatal("affordsAnyColor rejected a Mountain for a {W/U} pip")
	}
	s.payRider(sc.k, 0, &hyb, 0, NoObj, true)
	if !sc.tapped(mtn) {
		t.Fatal("the hybrid any-colour payment did not tap the Mountain")
	}
	if s.pl[0].pool != [kop.NumManaKinds]uint8{} {
		t.Fatalf("pool after the hybrid any-colour payment %v, want empty", s.pl[0].pool)
	}
}

// TestNestedMayPlayArms: the limit walk scans the flat program slice, so
// a permission nested in an OpIfKicked body arms anyMayLimit and anyMayKind
// (gorge's scanMayPlaysThisTurn walks every instruction).
func TestNestedMayPlayArms(t *testing.T) {
	tbl := castMaypermissionKindTable()
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name:  "Test Nested Permission",
		Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{
			{Op: kop.OpIfKicked, A: kop.Const(1)},
			{Op: kop.OpMayPlay, Obj: kop.RefRemembered, A: kop.Const(kop.DurEOT), B: kop.Const(1 << kop.MayPlayLimitShift), C: int16(kop.ZoneExile)},
			{Op: kop.OpNop},
		}}},
	})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if !sc.k.gen.anyMayLimit {
		t.Fatal("a nested MayPlayLimit did not arm anyMayLimit")
	}
	if !sc.k.gen.anyMayKind {
		t.Fatal("a nested MayPlayLimit did not arm anyMayKind")
	}
}

// TestMayPlayLimitPaths: every real path that moves a land from a
// graveyard or exile onto the battlefield counts the may-play, not only the
// countMayPlayLand hook itself.
func TestMayPlayLimitPaths(t *testing.T) {
	newLand := func(t *testing.T, tbl *kop.Table, o *Obj) (*scen, *State) {
		t.Helper()
		sc := newScenTable(t, tbl, Config{NoElide: true})
		*o = sc.obj(v.Mountain, 1)
		sc.s.place(*o, kop.ZoneGraveyard, false)
		return sc, sc.s
	}
	counted := func(t *testing.T, s *State, o Obj) {
		t.Helper()
		if s.g.hist[1][histMayPlay] != 1 || s.objs[o].zone != kop.ZoneBattlefield {
			t.Fatalf("may-play count %d, zone %v; want one count and the battlefield", s.g.hist[1][histMayPlay], s.objs[o].zone)
		}
	}

	t.Run("playLandNow", func(t *testing.T) {
		var o Obj
		sc, s := newLand(t, castMaypermissionLimitTable(), &o)
		s.playLandNow(sc.k, o, 1)
		counted(t, s, o)
	})
	t.Run("execReturn", func(t *testing.T) {
		var o Obj
		sc, s := newLand(t, castMaypermissionLimitTable(), &o)
		s.res.e = stackEntry{kind: seSpell, obj: o, ctrl: 1}
		in := kop.Instr{Op: kop.OpReturn, Obj: kop.RefSelf, C: int16(kop.ZoneBattlefield)}
		s.execReturn(sc.k, &in, NoSlot)
		counted(t, s, o)
	})
	t.Run("moveFound", func(t *testing.T) {
		var o Obj
		sc, s := newLand(t, castMaypermissionLimitTable(), &o)
		s.g.pickP, s.g.pickZ, s.g.pickF = 1, uint8(kop.ZoneBattlefield), 0
		s.resArm().look[0], s.g.nlook = o, 1
		s.moveFound(sc.k)
		counted(t, s, o)
	})
	t.Run("execChangeZoneAll", func(t *testing.T) {
		tbl := castMaypermissionLimitTable()
		tbl.Filters = append(tbl.Filters, kop.Filter{Zone: kop.ZoneGraveyard})
		var o Obj
		sc, s := newLand(t, tbl, &o)
		s.res.e = stackEntry{kind: seSpell, ctrl: 1}
		in := kop.Instr{Op: kop.OpChangeZoneAll, Obj: kop.RefNone, F: kop.FilterID(len(tbl.Filters) - 1), C: int16(kop.ZoneBattlefield)}
		s.execChangeZoneAll(sc.k, &in, false)
		counted(t, s, o)
	})
	t.Run("enterFromGrantedZone", func(t *testing.T) {
		var o Obj
		sc, s := newLand(t, castMaypermissionLimitTable(), &o)
		from := s.detach(o)
		s.enterFromGrantedZone(sc.k, o, 1, false, from)
		counted(t, s, o)
	})
	t.Run("leaveGen", func(t *testing.T) {
		sc := newScenTable(t, castMaypermissionLimitTable(), Config{NoElide: true})
		s := sc.s
		src := sc.perm(v.Mountain, 0)
		o := sc.obj(v.Mountain, 1)
		s.place(o, kop.ZoneExile, false)
		s.g.links[0] = struct {
			src  Obj
			sver uint8
			obj  Obj
			ts   uint16
		}{src: src, sver: s.objs[src].ver, obj: o}
		s.g.nlinks = 1
		sl := s.objs[src].slot
		s.objs[src].ver++ // the source left: leaveGen runs after the move
		s.leaveGen(sc.k, sl, src)
		counted(t, s, o)
	})
	t.Run("finishResolve", func(t *testing.T) {
		var o Obj
		sc, s := newLand(t, castMaypermissionLimitTable(), &o)
		s.place(o, kop.ZoneExile, false)
		s.sp = 1
		s.res.e = stackEntry{kind: seTrigger}
		s.g.pre = preAsk{obj: o, state: preReturn}
		s.finishResolve(sc.k, false)
		counted(t, s, o)
	})
}

// TestAnyColorWindowColour: an any-colour window source that makes both
// {C} and a colour spends {C} only when the pool still owes a {C} pip;
// while a coloured pip is unpaid it makes the colour, so the source cannot
// strand the pip (gorge's window backtracks over the alternatives).
func TestAnyColorWindowColour(t *testing.T) {
	sc := newScenTable(t, castMaypermissionKindTable(), Config{NoElide: true})
	s := sc.s
	mask := uint8(1<<kop.W | 1<<kop.C)
	cases := []struct {
		name string
		cost kop.ManaCost
		want int
	}{
		{"colour owed", kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.W: 1}}, kop.W},
		{"colourless owed", kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.C: 1}}, kop.C},
		{"both owed", kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.W: 1, kop.C: 1}}, kop.C},
		{"generic only", kop.ManaCost{Generic: 1}, kop.C},
	}
	for _, c := range cases {
		if got := s.windowAnyColour(0, &c.cost, mask, false); got != c.want {
			t.Fatalf("%s: colour %d, want %d", c.name, got, c.want)
		}
	}
	// a pool that already holds the coloured pip makes {C} instead
	s.pl[0].pool[kop.W] = 1
	cost := kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.W: 1}}
	if got := s.windowAnyColour(0, &cost, mask, false); got != kop.C {
		t.Fatalf("colour already in the pool: colour %d, want %d", got, kop.C)
	}
}
