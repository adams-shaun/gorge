package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// kcapsTable copies vanilla's table and appends extra cards, returning the
// ids of the appended cards. The copy shares the original's other tables.
func kcapsTable(t *testing.T, extra ...kop.CardDef) (*kop.Table, []CardID) {
	t.Helper()
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	first := len(tbl.Cards)
	tbl.Cards = append(tbl.Cards, extra...)
	ids := make([]CardID, len(extra))
	for i := range ids {
		ids[i] = CardID(first + i)
	}
	return &tbl, ids
}

// TestLookWindowFitsTwentyFour: a dig of 24 (genState.look's new width)
// records the whole window; a 25th card stalls loud with StallLook instead
// of truncating silently (interp_ask.go startDig).
func TestLookWindowFitsTwentyFour(t *testing.T) {
	build := func(t *testing.T, n int) *scen {
		t.Helper()
		tbl, ids := kcapsTable(t, kop.CardDef{Name: "Test Dig", Types: kop.TSorcery,
			Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{{
				Op: kop.OpDigEx, Obj: kop.RefController, A: kop.Const(n), B: kop.Const(-1), C: int16(kop.ZoneHand)}}}}})
		sc := newScenTable(t, tbl, Config{NoElide: true})
		for i := 0; i < 25; i++ {
			o := sc.obj(v.Plains, 0)
			sc.s.objs[o].zone = kop.ZoneLibrary
			sc.s.pl[0].libBottom(o)
		}
		sc.s.res = resCtx{e: stackEntry{kind: seSpell, card: ids[0], ab: 0, ctrl: 0}, pc: 0, active: true}
		sc.s.startDig(sc.k, &sc.s.prog(sc.k)[0])
		return sc
	}
	sc := build(t, 24)
	if got := sc.s.g.nlook; got != 24 || sc.s.Stalled() {
		t.Fatalf("24-card dig: window %d, stalled %v", got, sc.s.Stalled())
	}
	sc = build(t, 25)
	if got := sc.s.g.nlook; got != 24 {
		t.Fatalf("25-card dig: window %d, want 24", got)
	}
	if got := sc.s.StallReason(); got != StallLook {
		t.Fatalf("25-card dig: stall %q, want look", StallName(got))
	}
}

// TestMayPlayPermissionsFitSixteen: 16 play-from-exile permissions fit
// (two resolutions of 12 then 4 remembered cards); a 17th stalls with
// StallMayPlay (value_validagg.go mayPlayRemembered).
func TestMayPlayPermissionsFitSixteen(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	exiled := make([]Obj, 17)
	for i := range exiled {
		exiled[i] = sc.obj(v.GrizzlyBears, 0)
		sc.s.place(exiled[i], kop.ZoneExile, false)
	}
	remember := func(objs []Obj) {
		copy(sc.s.g.rem[:], objs)
		sc.s.g.nrem = uint8(len(objs))
		in := kop.Instr{Op: kop.OpMayPlay, C: int16(kop.ZoneExile), B: kop.Const(0)}
		sc.s.mayPlayRemembered(sc.k, &in, 0)
	}
	remember(exiled[:12])
	if n := sc.s.g.nmay; n != 12 || sc.s.Stalled() {
		t.Fatalf("12 permissions: %d, stalled %v", n, sc.s.Stalled())
	}
	remember(exiled[12:16])
	if n := sc.s.g.nmay; n != 16 || sc.s.Stalled() {
		t.Fatalf("16 permissions: %d, stalled %v", n, sc.s.Stalled())
	}
	remember(exiled[16:])
	if got := sc.s.StallReason(); got != StallMayPlay {
		t.Fatalf("17th permission: stall %q, want mayplay", StallName(got))
	}
}

// TestTapPowerPicksFitTwentyFour: a CostTapPower cost can record 24 picks
// (all 24 candidates); with a 25th candidate still unpicked the record is
// full and the pick stalls with StallTapPick (kw_crewsaddle.go
// tapPowerOptions).
func TestTapPowerPicksFitTwentyFour(t *testing.T) {
	build := func(t *testing.T, n int) *scen {
		t.Helper()
		tbl, ids := kcapsTable(t, kop.CardDef{Name: "Test Crew", Types: kop.TArtifact,
			Abilities: []kop.Ability{{Kind: kop.AbActivated, Cost: kop.Cost{Flags: kop.CostTapPower, N: 25}}}})
		sc := newScenTable(t, tbl, Config{NoElide: true})
		src := sc.perm(ids[0], 0)
		sc.s.perms[sc.slot(src)].flags |= pfTapped
		bears := make([]Obj, n)
		for i := range bears {
			bears[i] = sc.perm(v.GrizzlyBears, 0)
		}
		s := sc.s
		s.bld().e = stackEntry{kind: seAbility, card: ids[0], ab: 0, ctrl: 0, obj: src, srcVer: s.objs[src].ver}
		for i := 0; i < 24 && i < n; i++ {
			s.bld().picks[i], s.bld().pickKs[i] = bears[i], pkTapX
		}
		s.bld().pickN = 24
		s.ctl = ctlBuild // a live build: picked() reads arm B's picks only then
		var d Decision
		s.tapPowerOptions(sc.k, &d)
		return sc
	}
	if sc := build(t, 24); sc.s.Stalled() {
		t.Fatalf("24 candidates: stalled %q", StallName(sc.s.StallReason()))
	}
	sc := build(t, 25)
	if got := sc.s.StallReason(); got != StallTapPick {
		t.Fatalf("25 candidates: stall %q, want tappick", StallName(got))
	}
}

// TestLayerWalkCollectsTwentyFour: the layer walk collects 24 override
// statics on one slot; a 25th stalls with StallBoard (static_layerwalk.go
// lfxCollect, maxLfx doubled by kcaps2x).
func TestLayerWalkCollectsTwentyFour(t *testing.T) {
	build := func(t *testing.T, n int) *scen {
		t.Helper()
		tbl := *v.Table
		tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
		tbl.Exprs = make([]kop.Expr, 1) // force the generated path (Exprs[0] is unused)
		tbl.Overrides = append([]kop.Override(nil), v.Table.Overrides...)
		tbl.Overrides = append(tbl.Overrides, kop.Override{HasPT: true, SetP: 3, SetT: 3})
		ov := uint16(len(tbl.Overrides))
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Lord", Types: kop.TCreature, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatOverride, Override: ov}}}})
		lord := CardID(len(tbl.Cards) - 1)
		sc := newScenTable(t, &tbl, Config{NoElide: true})
		target := sc.perm(v.GrizzlyBears, 0)
		for i := 0; i < n; i++ {
			sc.perm(lord, 0)
		}
		sc.s.g.stalled = 0
		var fx [maxLfx]lfx
		if got := sc.s.lfxCollect(sc.k, sc.slot(target), &fx); got != 24 {
			t.Fatalf("%d sources: collected %d, want 24", n, got)
		}
		return sc
	}
	if sc := build(t, 24); sc.s.Stalled() {
		t.Fatalf("24 sources: stalled %q", StallName(sc.s.StallReason()))
	}
	sc := build(t, 25)
	if got := sc.s.StallReason(); got != StallBoard {
		t.Fatalf("25 sources: stall %q, want board", StallName(got))
	}
}
