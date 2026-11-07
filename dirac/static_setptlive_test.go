package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// livePTTable is vanilla's table plus an attached override static whose base
// P/T is its controller's life total (LivePT: the value is re-read at every
// layer-7b check) and a literal attached setter, both Equipment-shaped.
func livePTTable(t *testing.T) (tbl *kop.Table, live, literal CardID) {
	t.Helper()
	base := v.Table
	tbl = new(kop.Table)
	*tbl = *base
	tbl.Cards = append([]kop.CardDef(nil), base.Cards...)
	tbl.Filters = append([]kop.Filter(nil), base.Filters...)
	tbl.Overrides = append([]kop.Override(nil), base.Overrides...)
	tbl.Exprs = append([]kop.Expr(nil), base.Exprs...)

	f := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Flags: kop.FAttachedBySrc})
	life := int16(len(tbl.Exprs))
	tbl.Exprs = append(tbl.Exprs, kop.Expr{Kind: kop.ExLifeOf, Ref: kop.RefController})
	val := kop.ValueRef{K: kop.VExpr, N: life}

	liveOv := len(tbl.Overrides)
	tbl.Overrides = append(tbl.Overrides, kop.Override{HasPT: true, LivePT: true, PA: val, TA: val})
	literalOv := len(tbl.Overrides)
	tbl.Overrides = append(tbl.Overrides, kop.Override{HasPT: true, SetP: 0, SetT: 1})

	card := func(name string, ov int) CardID {
		id := CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: name, Types: kop.TArtifact,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{
				Kind: kop.StatOverride, Filter: f, Override: uint16(ov + 1)}}}})
		return id
	}
	return tbl, card("Live Equipment", liveOv), card("Literal Equipment", literalOv)
}

// TestStaticOverrideLivePT: an attached static override with LivePT reads its
// PA/TA with the STATIC source as the context source and its controller, not
// the affected permanent (an Equipment's "your life total" is its
// controller's); the fixed attached-source reader and the layer walk agree,
// the value follows the life total, and unequipping returns the printed P/T.
func TestStaticOverrideLivePT(t *testing.T) {
	tbl, liveEq, _ := livePTTable(t)
	if !tbl.Overrides[0].LivePT {
		t.Fatal("test setup: the override is not LivePT")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	creature := sc.perm(v.SavannahLions, 0) // printed 2/1
	sl := sc.slot(creature)
	if p, tt := sc.s.basePTGen(sc.k, sl); p != 2 || tt != 1 {
		t.Fatalf("unequipped base %d/%d, want the printed 2/1", p, tt)
	}
	sc.s.pl[0].life = 7
	eq := sc.perm(liveEq, 0)
	sc.s.attachTo(sc.k, sc.slot(eq), sl)
	sc.s.refreshTy(sc.k)
	if sc.s.perms[sl].ovr.walk&walkOn != 0 {
		t.Fatal("test setup: one static is already a walk slot, not the fixed reader")
	}
	if p, tt := sc.s.basePTGen(sc.k, sl); p != 7 || tt != 7 {
		t.Fatalf("equipped base %d/%d, want 7/7 from the controller's life total", p, tt)
	}
	sc.s.pl[0].life = 3
	if p, tt := sc.s.basePTGen(sc.k, sl); p != 3 || tt != 3 {
		t.Fatalf("after the life total changed, base %d/%d, want 3/3", p, tt)
	}
	sc.s.perms[sc.slot(eq)].attached = NoSlot
	sc.s.refreshTy(sc.k)
	if p, tt := sc.s.basePTGen(sc.k, sl); p != 2 || tt != 1 {
		t.Fatalf("after unequipping, base %d/%d, want the printed 2/1", p, tt)
	}

	// The Equipment's controller is the anchor: an opponent's copy reads the
	// opponent's life total (9), not the creature controller's (3).
	sc.s.pl[1].life = 9
	opp := sc.perm(liveEq, 1)
	sc.s.attachTo(sc.k, sc.slot(opp), sl)
	sc.s.refreshTy(sc.k)
	if c := sc.s.perms[sc.slot(opp)].ctrl; c != 1 {
		t.Fatalf("test setup: the Equipment's controller is %d, want 1", c)
	}
	if p, tt := sc.s.basePTGen(sc.k, sl); p != 9 || tt != 9 {
		t.Fatalf("opponent-equipped base %d/%d, want 9/9 from the opponent's life total", p, tt)
	}

	t.Run("beside a literal setter the layer walk reads the live source", func(t *testing.T) {
		tbl, liveEq, literalEq := livePTTable(t)
		sc := newScenTable(t, tbl, Config{NoElide: true})
		creature := sc.perm(v.SavannahLions, 0)
		sl := sc.slot(creature)
		// The live Equipment is the opponent's, and the two life totals
		// differ: the walk must read the source's controller (9), not the
		// affected creature's (6).
		sc.s.pl[0].life, sc.s.pl[1].life = 6, 9
		lit := sc.perm(literalEq, 0)
		sc.s.attachTo(sc.k, sc.slot(lit), sl)
		sc.s.refreshTy(sc.k)
		live := sc.perm(liveEq, 1)
		sc.s.attachTo(sc.k, sc.slot(live), sl)
		sc.s.refreshTy(sc.k)
		if sc.s.perms[sl].ovr.walk&walkOn == 0 {
			t.Fatal("test setup: two override statics do not make the slot walk")
		}
		if c := sc.s.perms[sc.slot(live)].ctrl; c != 1 || sc.s.perms[sl].ctrl != 0 {
			t.Fatalf("test setup: live Equipment controller %d, creature controller %d; want 1 and 0", c, sc.s.perms[sl].ctrl)
		}
		if p, tt := sc.s.basePTGen(sc.k, sl); p != 9 || tt != 9 {
			t.Fatalf("walked base %d/%d, want 9/9 from the Equipment controller's life total", p, tt)
		}
		sc.s.pl[1].life = 2
		if p, tt := sc.s.basePTGen(sc.k, sl); p != 2 || tt != 2 {
			t.Fatalf("walked base after the life total changed %d/%d, want 2/2", p, tt)
		}
	})

	t.Run("a host that is not a creature keeps its printed P/T", func(t *testing.T) {
		tbl, liveEq, _ := livePTTable(t)
		sc := newScenTable(t, tbl, Config{NoElide: true})
		creature := sc.perm(v.SavannahLions, 0)
		sl := sc.slot(creature)
		sc.s.pl[0].life = 7
		eq := sc.perm(liveEq, 0)
		sc.s.attachTo(sc.k, sc.slot(eq), sl)
		sc.s.refreshTy(sc.k)
		if p, tt := sc.s.basePTGen(sc.k, sl); p != 7 || tt != 7 {
			t.Fatalf("test setup: equipped base %d/%d, want 7/7", p, tt)
		}
		// Narrow the static's filter to a type the host lacks (as a host that
		// stopped being a creature would): the fixed reader must apply it.
		tbl.Filters[len(tbl.Filters)-1].Types = kop.TLand
		if sc.s.typesGen(sc.k, sl)&kop.TLand != 0 {
			t.Fatal("test setup: the host is a land")
		}
		if p, tt := sc.s.basePTGen(sc.k, sl); p != 2 || tt != 1 {
			t.Fatalf("base %d/%d on a host the filter does not match, want the printed 2/1", p, tt)
		}
	})
}
