package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// tapsDeferFilterBoard puts a tapped Retrofitter Foundry on p0's battlefield
// beside an untapped Viridescent Bog and, with woodlot, an untapped Hickory
// Woodlot holding its depletion counters; no other mana source.
func tapsDeferFilterBoard(t *testing.T, woodlot bool) (*scen, Obj, Obj, Obj) {
	t.Helper()
	set := cardsettest.Set(t, "tapsdeferfilter")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for i := range tbl.Cards {
		id[tbl.Cards[i].Name] = CardID(i)
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	wl := Obj(NoObj)
	if woodlot {
		wl = sc.perm(id["Hickory Woodlot"], 0)
		sl := sc.slot(wl)
		a, ok := sc.s.riderAbilityOf(sc.k, sl)
		if !ok || sc.s.perms[sl].ctrN(sc.s, int(a.ManaRiderCtr)) == 0 {
			t.Fatal("precondition: Hickory Woodlot is not a depletion rider holding counters")
		}
		sc.s.perms[sl].flags &^= pfTapped // it entered tapped; it untaps on its controller's turn
	}
	bog := sc.perm(id["Viridescent Bog"], 0)
	sc.s.perms[sc.slot(bog)].flags &^= pfTapped
	fd := sc.perm(id["Retrofitter Foundry"], 0)
	sc.s.perms[sc.slot(fd)].flags |= pfTapped
	return sc, wl, bog, fd
}

// foundryOffered steps to p0's priority decision and returns the Foundry's
// activation when the play is on it (tapped, its only ability is {3}: Untap).
func foundryOffered(t *testing.T, sc *scen, fd Obj) (Decision, Action, bool) {
	t.Helper()
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && Obj(a.Source()) == fd {
			return d, a, true
		}
	}
	return d, 0, false
}

// TestTapsDeferGapFundsFilterFee: Hickory Woodlot's {G}{G} (a depletion
// rider, a Taps-deferred census gap) pays one {G} into Viridescent Bog's {1}
// fee, and the Bog's {B}{G} with the other {G} pays Retrofitter Foundry's {3}
// untap, so gorge's one prefix search finds it and the activation is offered
// (mana_filtertapsdefer.go; CR 605.3a, 601.2g-h). Played, both lands are
// tapped and the Woodlot lost a counter. The Bog alone pays nothing ({1}
// needs mana it cannot make), so without the Woodlot it is not offered.
func TestTapsDeferGapFundsFilterFee(t *testing.T) {
	sc, wl, bog, fd := tapsDeferFilterBoard(t, true)
	wsl := sc.slot(wl)
	if !sc.s.scriptTapsDefer(sc.k, 0).has(wsl) {
		t.Fatal("Hickory Woodlot is not a live Taps-deferred gap")
	}
	if !sc.s.scriptFilt(sc.k, 0).has(sc.slot(bog)) {
		t.Fatal("Viridescent Bog is not a census filter gap")
	}
	a, _ := sc.s.riderAbilityOf(sc.k, wsl)
	ctr0 := sc.s.perms[wsl].ctrN(sc.s, int(a.ManaRiderCtr))
	d, act, ok := foundryOffered(t, sc, fd)
	if !ok {
		t.Fatalf("Retrofitter Foundry's {3} untap not offered: %v", d.Options[:d.N])
	}
	sc.s.Apply(act)
	sc.s.Step()
	if !sc.tapped(wl) || !sc.tapped(bog) {
		t.Errorf("Woodlot tapped %v, Bog tapped %v after the activation, want both", sc.tapped(wl), sc.tapped(bog))
	}
	if n := sc.s.perms[wsl].ctrN(sc.s, int(a.ManaRiderCtr)); n != ctr0-1 {
		t.Errorf("Woodlot holds %d depletion counters, want %d", n, ctr0-1)
	}

	sc, _, _, fd = tapsDeferFilterBoard(t, false)
	if d, _, ok := foundryOffered(t, sc, fd); ok {
		t.Fatalf("Retrofitter Foundry's {3} untap offered with the Bog alone: %v", d.Options[:d.N])
	}
}
