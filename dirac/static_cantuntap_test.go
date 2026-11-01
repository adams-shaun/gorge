package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestAnyUntapFunnel: ReplAnyUntap (gorge cards.ReplUntap with no
// ValidStepTurnToController$, "can't become untapped"; Frozen in Ice) through
// the one State.untapBlocked funnel: one match keeps a tapped permanent
// tapped at an effect site; none untaps it; a stun counter is removed first
// (CR 122.1d); two matches keep it tapped at an effect site, with no ask (none is
// possible there) and ask the CR 616.1 order in the untap step.
func TestAnyUntapFunnel(t *testing.T) {
	mk := func(t *testing.T) (*scen, CardID, CardID) {
		t.Helper()
		tbl := *vanilla.Table
		tbl.Cards = append([]kop.CardDef(nil), vanilla.Table.Cards...)
		tbl.Filters = append([]kop.Filter(nil), vanilla.Table.Filters...)
		tbl.Exprs = make([]kop.Expr, 1)
		tbl.Filters = append(tbl.Filters, kop.Filter{Flags: kop.FOther})
		other := kop.FilterID(len(tbl.Filters) - 1)
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Freezer", Types: kop.TCreature, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplAnyUntap, Filter: other}}}})
		sc := newScenTable(t, &tbl, Config{NoElide: true})
		if !sc.k.gen.untapRepl || !sc.k.gen.anyUntapBlock {
			t.Fatal("prep did not see the replacement")
		}
		return sc, CardID(len(tbl.Cards) - 1), vanilla.GrizzlyBears
	}
	tap := func(sc *scen, o Obj) Slot {
		sl := sc.slot(o)
		sc.s.perms[sl].flags |= pfTapped
		return sl
	}

	t.Run("block", func(t *testing.T) {
		sc, fz, bear := mk(t)
		sc.perm(fz, 0)
		tg := sc.perm(bear, 0)
		sc.s.untapSlot(sc.k, tap(sc, tg))
		if !sc.tapped(tg) || sc.s.Stalled() {
			t.Fatalf("tapped %v stall %q, want tapped and none", sc.tapped(tg), StallName(sc.s.StallReason()))
		}
	})
	t.Run("control", func(t *testing.T) {
		sc, _, bear := mk(t)
		tg := sc.perm(bear, 0)
		sc.s.untapSlot(sc.k, tap(sc, tg))
		if sc.tapped(tg) {
			t.Fatal("no source: the permanent should untap")
		}
	})
	t.Run("stun first", func(t *testing.T) {
		sc, fz, bear := mk(t)
		sc.perm(fz, 0)
		tg := sc.perm(bear, 0)
		sl := tap(sc, tg)
		sc.s.perms[sl].addCtrN(sc.s, int(kop.CtrStun), 1)
		sc.s.untapSlot(sc.k, sl)
		if !sc.tapped(tg) || sc.s.perms[sl].ctrLow(int(kop.CtrStun)) != 0 || sc.s.Stalled() {
			t.Fatalf("tapped %v stun %d stall %q, want tapped, stun removed, no stall",
				sc.tapped(tg), sc.s.perms[sl].ctrLow(int(kop.CtrStun)), StallName(sc.s.StallReason()))
		}
	})
	t.Run("two matches keep it tapped at an effect site", func(t *testing.T) {
		sc, fz, bear := mk(t)
		sc.perm(fz, 0)
		sc.perm(fz, 1)
		tg := sc.perm(bear, 0)
		sc.s.untapSlot(sc.k, tap(sc, tg))
		if sc.s.Stalled() {
			t.Fatalf("stall %q, want none: nothing to order (CR 502.3, 614.1)", StallName(sc.s.StallReason()))
		}
		if !sc.tapped(tg) {
			t.Fatal("the permanent untapped on a stalled funnel")
		}
	})
	t.Run("untap step asks nothing with two matches", func(t *testing.T) {
		sc, fz, bear := mk(t)
		sc.perm(fz, 0)
		sc.perm(fz, 0)
		tg := sc.perm(bear, 0)
		tap(sc, tg)
		sc.s.turn, sc.s.active, sc.s.step, sc.s.ctl = 3, 0, kop.StepUntap, ctlStepBegin
		for i := 0; i < 10 && sc.s.step == kop.StepUntap; i++ {
			if d := sc.s.Step(); d.Kind == DkChoose {
				t.Fatalf("an ask: sub %v", d.Sub)
			}
		}
		if !sc.tapped(tg) || sc.s.Stalled() {
			t.Fatalf("tapped %v stall %q, want tapped and none", sc.tapped(tg), StallName(sc.s.StallReason()))
		}
	})
	t.Run("control", func(t *testing.T) {
		sc, _, bear := mk(t)
		tg := sc.perm(bear, 0)
		sc.s.untapSlot(sc.k, tap(sc, tg))
		if sc.tapped(tg) {
			t.Fatal("no source: the permanent should untap")
		}
	})
	t.Run("stun first", func(t *testing.T) {
		sc, fz, bear := mk(t)
		sc.perm(fz, 0)
		tg := sc.perm(bear, 0)
		sl := tap(sc, tg)
		sc.s.perms[sl].addCtrN(sc.s, int(kop.CtrStun), 1)
		sc.s.untapSlot(sc.k, sl)
		if !sc.tapped(tg) || sc.s.perms[sl].ctrLow(int(kop.CtrStun)) != 0 || sc.s.Stalled() {
			t.Fatalf("tapped %v stun %d stall %q, want tapped, stun removed, no stall",
				sc.tapped(tg), sc.s.perms[sl].ctrLow(int(kop.CtrStun)), StallName(sc.s.StallReason()))
		}
	})
	t.Run("two matches keep it tapped at an effect site", func(t *testing.T) {
		sc, fz, bear := mk(t)
		sc.perm(fz, 0)
		sc.perm(fz, 1)
		tg := sc.perm(bear, 0)
		sc.s.untapSlot(sc.k, tap(sc, tg))
		if sc.s.Stalled() {
			t.Fatalf("stall %q, want none: nothing to order (CR 502.3, 614.1)", StallName(sc.s.StallReason()))
		}
		if !sc.tapped(tg) {
			t.Fatal("the permanent untapped on a stalled funnel")
		}
	})
	t.Run("untap step asks nothing with two matches", func(t *testing.T) {
		sc, fz, bear := mk(t)
		sc.perm(fz, 0)
		sc.perm(fz, 0)
		tg := sc.perm(bear, 0)
		tap(sc, tg)
		sc.s.turn, sc.s.active, sc.s.step, sc.s.ctl = 3, 0, kop.StepUntap, ctlStepBegin
		for i := 0; i < 10 && sc.s.step == kop.StepUntap; i++ {
			if d := sc.s.Step(); d.Kind == DkChoose {
				t.Fatalf("an ask: sub %v", d.Sub)
			}
		}
		if !sc.tapped(tg) || sc.s.Stalled() {
			t.Fatalf("tapped %v stall %q, want tapped and none", sc.tapped(tg), StallName(sc.s.StallReason()))
		}
	})
}
