package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestThisTurnCastExpr: ExThisTurnCast reads the caster's spell count
// from the history bank (N=0) or the instant/sorcery tally (N=1), and a
// table carrying the N=0 form gates the history bank on.
func TestThisTurnCastExpr(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 8}}
	set := cardsettest.Set(t, "blb")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	cp := *tbl
	cp.Exprs = append(append([]kop.Expr(nil), tbl.Exprs...), kop.Expr{Kind: kop.ExThisTurnCast, Ref: kop.RefController})
	var lib [20]CardID
	for i := range lib {
		lib[i] = da[0]
	}
	s := new(State)
	s.Init(&cp, lib[:], lib[:], 1, Config{NoElide: true})
	k := s.k()
	if k.gen == nil || !k.gen.anyHist {
		t.Fatal("a Count$ThisTurnCast table does not gate the history bank on")
	}
	s.g.hist[0][histSpells], s.g.hist[1][histSpells], s.g.isCast[0] = 2, 3, 1
	for _, tc := range []struct {
		x    kop.Expr
		want int
	}{
		{kop.Expr{Kind: kop.ExThisTurnCast, Ref: kop.RefController}, 2},
		{kop.Expr{Kind: kop.ExThisTurnCast, Ref: kop.RefEachOpponent}, 3},
		{kop.Expr{Kind: kop.ExThisTurnCast, Ref: kop.RefController, N: 1}, 1},
	} {
		if got := s.thisTurnCast(k, &tc.x, evalCtx{ctrl: 0, src: NoObj, srcSl: NoSlot}); got != tc.want {
			t.Errorf("%+v: %d, want %d", tc.x, got, tc.want)
		}
	}
}

// kcastYouCtrl is the YouCtrl filter kcastScen appends to the pool table.
var kcastYouCtrl kop.FilterID

// kcastScen is a scenario over the generated thisturncast pool table, with
// a bare YouCtrl filter appended (kcastYouCtrl).
func kcastScen(t *testing.T) (*scen, func(string) CardID) {
	t.Helper()
	set := cardsettest.Set(t, "thisturncast")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(append([]kop.Filter(nil), tbl.Filters...), kop.Filter{Ctrl: kop.CtrlYou})
	kcastYouCtrl = kop.FilterID(len(tbl.Filters) - 1)
	sc := newScenTable(t, tbl, Config{})
	if !sc.k.gen.castRingOn {
		t.Fatal("the thisturncast table does not turn the cast ring on")
	}
	id := func(name string) CardID {
		i, ok := set.Lookup(name)
		if !ok {
			t.Fatalf("%s not in the pool", name)
		}
		return CardID(set.PoolID(i))
	}
	return sc, id
}

// kcastBuild starts building a cast of o (card c) by p and pushes it (CR
// 601.2a), as buildStep's bsPush does. The build stays live (ctlBuild) so a
// following manaStep/finishBuild reads it as the real bsMana/bsPay do.
func (sc *scen) kcastBuild(o Obj, c CardID, p uint8) {
	*sc.s.bld() = build{e: stackEntry{kind: seSpell, obj: o, card: c, ctrl: p, srcVer: sc.s.objs[o].ver}}
	sc.s.bld().e.tgt = noTargets
	sc.s.pushBuild(sc.k)
	sc.s.ctl = ctlBuild
}

// TestThisturncastCostExclusion: Focus the Mind ("costs {2} less if you've
// cast another spell this turn", Count$ThisTurnCast_Card.YouCtrl GE1).
// gorge composes a pushed spell's cost with its own push hidden
// (withCostCompositionEvent), so the first Focus costs its full {4}{U} both
// at the offer (before the push) and at payment (after it); the second sees
// the first and costs {2}{U}. Outside the composition the in-flight push
// counts.
func TestThisturncastCostExclusion(t *testing.T) {
	sc, id := kcastScen(t)
	focus := id("Focus the Mind")
	full := sc.k.t.Cards[focus].Cost
	if full.Generic != 4 {
		t.Fatalf("Focus the Mind printed generic %d, want 4", full.Generic)
	}
	f1, f2 := sc.hand(focus, 0), sc.hand(focus, 0)
	if g := sc.s.adjustCost(sc.k, 0, focus, NoObj, full).Generic; g != 4 {
		t.Fatalf("first Focus at the offer: generic %d, want 4", g)
	}
	sc.kcastBuild(f1, focus, 0)
	if g := sc.s.buildCost(sc.k).Generic; g != 4 {
		t.Fatalf("first Focus composed after its push: generic %d, want 4 (its own push hidden)", g)
	}
	x := &kop.Expr{Kind: kop.ExThisTurnCastSpec, N: kop.CastSpecYou}
	if n := sc.s.thisTurnCastSpec(sc.k, x, evalCtx{src: NoObj, srcSl: NoSlot, ctrl: 0}); n != 1 {
		t.Fatalf("outside the composition the in-flight push counts: got %d, want 1", n)
	}
	if g := sc.s.adjustCost(sc.k, 0, focus, NoObj, full).Generic; g != 2 {
		t.Fatalf("second Focus at the offer: generic %d, want 2", g)
	}
	sc.kcastBuild(f2, focus, 0)
	if g := sc.s.buildCost(sc.k).Generic; g != 2 {
		t.Fatalf("second Focus composed after its push: generic %d, want 2", g)
	}
	if sc.s.bld().bfl&bflCostComp != 0 {
		t.Fatal("bflCostComp outlived buildCost")
	}
}

// TestThisturncastAbortCounts pins gorge's CR 733.1 reversal: abortCast
// (rules/cast_commit.go) moves the pushed spell back to the zone it came
// from and emits SpellCastReversed, which only undoes the
// SpellsCastThisTurn turn watcher; the PutOnStack stays in the append-only
// log and spellsCastThisTurnMatching (rules/host_cast_ledger.go) never
// reads SpellCastReversed. So the reversed cast still counts, matched
// against the card back in its owner's hand, and unpushSpell leaves the
// ring entry in place.
func TestThisturncastAbortCounts(t *testing.T) {
	sc, id := kcastScen(t)
	scour := id("Thought Scour")
	o := sc.hand(scour, 0)
	sc.kcastBuild(o, scour, 0)
	sc.s.unpushSpell(sc.k)
	if sc.s.objs[o].zone != kop.ZoneHand {
		t.Fatalf("reversed spell in zone %d, want the hand", sc.s.objs[o].zone)
	}
	x := &kop.Expr{Kind: kop.ExThisTurnCastSpec, N: kop.CastSpecYou}
	ctx := evalCtx{src: NoObj, srcSl: NoSlot, ctrl: 0}
	if n := sc.s.thisTurnCastSpec(sc.k, x, ctx); n != 1 {
		t.Fatalf("a reversed cast: got %d, want 1 (gorge keeps its PutOnStack)", n)
	}
	hand := &kop.Expr{Kind: kop.ExThisTurnCastSpec, N: kop.CastSpecYou | kop.CastSpecHand}
	if n := sc.s.thisTurnCastSpec(sc.k, hand, ctx); n != 1 {
		t.Fatalf("a reversed cast from the hand: got %d, want 1", n)
	}
	// Cast again: the same object is listed twice, as gorge's log holds
	// two pushes.
	sc.kcastBuild(o, scour, 0)
	if n := sc.s.thisTurnCastSpec(sc.k, x, ctx); n != 2 {
		t.Fatalf("recast after a reversal: got %d, want 2", n)
	}
}

// TestThisturncastRing pins the ring's reads: You-scoping by caster, the
// !CastSaSource exclusion of every push of the source, hand provenance per
// push, and the current controller (the owner once the object is off the
// battlefield and stack).
func TestThisturncastRing(t *testing.T) {
	sc, id := kcastScen(t)
	scour := id("Thought Scour")
	a := sc.hand(scour, 0)
	sc.kcastBuild(a, scour, 0) // p0's cast from hand
	b := sc.obj(scour, 1)      // p1's card, cast by p0 from exile (not from hand)
	sc.s.place(b, kop.ZoneExile, false)
	sc.kcastBuild(b, scour, 0)
	c := sc.hand(scour, 1)
	sc.kcastBuild(c, scour, 1) // p1's own cast
	ctx0 := evalCtx{src: NoObj, srcSl: NoSlot, ctrl: 0}
	you := &kop.Expr{Kind: kop.ExThisTurnCastSpec, N: kop.CastSpecYou}
	if n := sc.s.thisTurnCastSpec(sc.k, you, ctx0); n != 2 {
		t.Fatalf("p0's casts: got %d, want 2", n)
	}
	all := &kop.Expr{Kind: kop.ExThisTurnCastSpec}
	if n := sc.s.thisTurnCastSpec(sc.k, all, ctx0); n != 3 {
		t.Fatalf("every cast: got %d, want 3", n)
	}
	hand := &kop.Expr{Kind: kop.ExThisTurnCastSpec, N: kop.CastSpecYou | kop.CastSpecHand}
	if n := sc.s.thisTurnCastSpec(sc.k, hand, ctx0); n != 1 {
		t.Fatalf("p0's casts from hand: got %d, want 1", n)
	}
	notHand := &kop.Expr{Kind: kop.ExThisTurnCastSpec, N: kop.CastSpecYou | kop.CastSpecNotHand}
	if n := sc.s.thisTurnCastSpec(sc.k, notHand, ctx0); n != 1 {
		t.Fatalf("p0's casts from anywhere but the hand: got %d, want 1", n)
	}
	excl := &kop.Expr{Kind: kop.ExThisTurnCastSpec, N: kop.CastSpecYou | kop.CastSpecExclude}
	if n := sc.s.thisTurnCastSpec(sc.k, excl, evalCtx{src: a, srcSl: NoSlot, ctrl: 0}); n != 1 {
		t.Fatalf("p0's casts other than the source: got %d, want 1", n)
	}
	// YouCtrl reads the object now: b is on the stack under p0; once it
	// leaves the stack it is its owner's (p1's), and p0's YouCtrl count
	// drops though p0 cast it.
	yc := kcastYouCtrl
	ycx := &kop.Expr{Kind: kop.ExThisTurnCastSpec, N: kop.CastSpecYou, F: yc}
	if n := sc.s.thisTurnCastSpec(sc.k, ycx, ctx0); n != 2 {
		t.Fatalf("Card.YouCtrl with both of p0's spells on the stack: got %d, want 2", n)
	}
	sc.s.sp-- // b (pushed second, under c) leaves: pop c and b, put b in its owner's graveyard
	sc.s.sp--
	sc.s.objs[b].zone = kop.ZoneNone
	sc.s.place(b, kop.ZoneGraveyard, false)
	if n := sc.s.thisTurnCastSpec(sc.k, ycx, ctx0); n != 1 {
		t.Fatalf("Card.YouCtrl after p1's card left the stack: got %d, want 1", n)
	}
}

// A table with no cast ring still hashes its history bank.
func TestViewHashWithoutCastRing(t *testing.T) {
	var s State
	s.g.on = true
	s.g.hist[1][histSpells] = 1
	s.genViewHash(0)
	if len(s.castPushes(0))+len(s.castPushes(1)) != 0 {
		t.Fatal("pushes in a table with no ring")
	}
}

// TestThisturncastRingSaturates: a push past castRingN by one player in one turn
// stalls with StallCastRing and records nothing (never truncates); the
// other player's ring is untouched.
func TestThisturncastRingSaturates(t *testing.T) {
	sc, id := kcastScen(t)
	scour := id("Thought Scour")
	o := sc.obj(scour, 0)
	for i := 0; i < castRingN; i++ {
		sc.s.noteCast(o, 0, kop.ZoneHand)
	}
	h := sc.s.g.hist[0]
	if h[histCastN] != castRingN || sc.s.g.stalled != 0 {
		t.Fatalf("after %d pushes: count %d stalled %d", castRingN, h[histCastN], sc.s.g.stalled)
	}
	sc.s.noteCast(o, 0, kop.ZoneExile)
	if sc.s.g.hist[0] != h {
		t.Fatalf("the overflowing push changed the ring: %v -> %v", h, sc.s.g.hist[0])
	}
	if sc.s.g.stalled != StallCastRing {
		t.Fatalf("overflow stall %d, want StallCastRing %d", sc.s.g.stalled, StallCastRing)
	}
	if sc.s.g.hist[1] != [3]uint8{} {
		t.Fatalf("p1's history changed: %v", sc.s.g.hist[1])
	}
}

// TestCastWithFlashCond: a CastWithFlash static's condition (its
// CheckSVar$/IsPresent$, Static.Cond2) gates the permission, as gorge's
// castWithFlash reads the static's conditions; castStatic used to grant it
// unconditionally.
func TestCastWithFlashCond(t *testing.T) {
	for _, tc := range []struct {
		b    int
		want bool
	}{{b: 2, want: false}, {b: 1, want: true}} {
		tbl := *v.Table
		tbl.Exprs = make([]kop.Expr, 1) // force the generated path
		tbl.Conds = append([]kop.Cond(nil), v.Table.Conds...)
		if len(tbl.Conds) == 0 {
			tbl.Conds = append(tbl.Conds, kop.Cond{}) // CondID 0 is "always"
		}
		tbl.Conds = append(tbl.Conds, kop.Cond{Kind: kop.CondValue, Op: kop.CmpGE, A: kop.Const(1), B: kop.Const(tc.b)})
		cond := kop.CondID(len(tbl.Conds) - 1)
		tbl.Cards = append(append([]kop.CardDef(nil), v.Table.Cards...), kop.CardDef{Types: kop.TArtifact,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatCastWithFlash, Cond2: cond}}}})
		orrery := CardID(len(tbl.Cards) - 1)
		sc := newScenTable(t, &tbl, Config{})
		sc.perm(orrery, 0)
		if got := sc.s.castStatic(sc.k, kop.StatCastWithFlash, 0, v.GrizzlyBears); got != tc.want {
			t.Errorf("condition 1 >= %d: flash %v, want %v", tc.b, got, tc.want)
		}
	}
}

// TestThisTurnCastOrigin pins the two origin reads gorge's ledger makes:
// wasCastFromExile per push from the push's origin zone (castOriginAdmits
// on the ledger's cast view, any caster), and the bare wasCastFromGraveyard
// as the CastFlags predicate on the object NOW (matchesSpec's
// state.ObjectWasCastFromGraveyard: the CastFlags or,, the
// latest cast zone): every push of an object on the stack cast from a
// graveyard counts, none once it has left the stack, and while buildCost
// composes the in-flight cast's reductions its own push is hidden but its
// zone already answers for the object's earlier pushes.
func TestThisTurnCastOrigin(t *testing.T) {
	sc, id := kcastScen(t)
	scour := id("Thought Scour")
	a := sc.hand(scour, 0)
	sc.kcastBuild(a, scour, 0) // p0 from hand
	b := sc.obj(scour, 0)
	sc.s.place(b, kop.ZoneExile, false)
	sc.kcastBuild(b, scour, 0) // p0 from exile
	c := sc.obj(scour, 1)
	sc.s.place(c, kop.ZoneExile, false)
	sc.kcastBuild(c, scour, 1) // p1 from exile
	g := sc.hand(scour, 0)
	sc.kcastBuild(g, scour, 0) // p0 from hand, then countered into the graveyard
	sc.s.sp--
	sc.s.objs[g].zone = kop.ZoneNone
	sc.s.place(g, kop.ZoneGraveyard, false)
	*sc.s.bld() = build{e: stackEntry{kind: seSpell, obj: g, card: scour, ctrl: 0, flags: sfFlashback, srcVer: sc.s.objs[g].ver}}
	sc.s.bld().e.tgt = noTargets
	sc.s.pushBuild(sc.k) // and flashed back
	var got []uint8
	for _, r := range sc.s.castPushes(0) {
		got = append(got, r.prov)
	}
	if want := []uint8{castFromHand, castFromExile, castFromHand, castFromGrave}; !slices.Equal(got, want) {
		t.Fatalf("p0 origins %v, want %v", got, want)
	}
	ctx0 := evalCtx{src: NoObj, srcSl: NoSlot, ctrl: 0}
	cnt := func(n int) int {
		return sc.s.thisTurnCastSpec(sc.k, &kop.Expr{Kind: kop.ExThisTurnCastSpec, N: int16(n)}, ctx0)
	}
	if n := cnt(kop.CastSpecYou | kop.CastSpecExile); n != 1 {
		t.Fatalf("p0's casts from exile: got %d, want 1", n)
	}
	if n := cnt(kop.CastSpecExile); n != 2 {
		t.Fatalf("every cast from exile: got %d, want 2", n)
	}
	if n := cnt(kop.CastSpecYou | kop.CastSpecGrave); n != 1 {
		t.Fatalf("wasCastFromGraveyard with g on the stack flashed back: got %d, want 1 (per push)", n)
	}
	sc.s.bld().bfl |= bflCostComp
	if n := cnt(kop.CastSpecYou | kop.CastSpecGrave); n != 0 {
		t.Fatalf("wasCastFromGraveyard while the flashback cast composes its cost: got %d, want 0 (the in-flight push is hidden)", n)
	}
	sc.s.bld().bfl &^= bflCostComp
	sc.s.sp-- // the flashback spell resolves into exile: gorge clears CastFlags on the move
	sc.s.objs[g].zone = kop.ZoneNone
	sc.s.place(g, kop.ZoneExile, false)
	if n := cnt(kop.CastSpecYou | kop.CastSpecGrave); n != 1 {
		t.Fatalf("wasCastFromGraveyard after g left the stack: got %d, want 1 (the cast happened, CR 400.7)", n)
	}
	if n := cnt(kop.CastSpecYou | kop.CastSpecExile); n != 1 {
		t.Fatalf("p0's casts from exile after g moved to exile: got %d, want 1 (per push, not now)", n)
	}
	sc.s.newTurnGen()
	if len(sc.s.castPushes(0)) != 0 || sc.s.cold.castRing[0] != (castPush{}) {
		t.Fatalf("the cast ring survived the turn: %v", sc.s.cold.castRing)
	}
}

// TestCastSpecAsCastCountsARecast: an OnlyFirstSpell$ spec drops no push of the
// object being priced (gorge onlyFirstSpellUsed skips only the push under
// composition, which the build hides separately): an object cast, bounced and
// priced again the same turn finds its earlier cast in the ledger.
func TestCastSpecAsCastCountsARecast(t *testing.T) {
	sc, id := kcastScen(t)
	f := sc.hand(id("Focus the Mind"), 0)
	s := sc.s
	s.noteCast(f, 0, kop.ZoneHand)
	x := &kop.Expr{Kind: kop.ExThisTurnCastSpec, N: kop.CastSpecYou | kop.CastSpecAsCast}
	ctx := evalCtx{src: NoObj, srcSl: NoSlot, ctrl: 0}
	s.bld().e.obj = f
	s.bld().e.kind = seSpell
	s.bld().bfl |= bflCostComp
	defer func() { s.bld().bfl &^= bflCostComp }()
	if n := s.thisTurnCastSpec(sc.k, x, ctx); n != 1 {
		t.Fatalf("pricing a recast misses its earlier cast: %d", n)
	}
}
