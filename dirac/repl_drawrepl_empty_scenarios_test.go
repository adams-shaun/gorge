package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The CR 614.11 arms of the empty-library Draw replacement scan
// (drawExcl, dirac/zones.go) over the real draw-repl pool tables: a line
// gorge's emptyDrawReplacement admits must run before the empty-library
// mark, so no mark follows and the state-based pass never loses the player.

// drainLib empties p's library in the scenario state.
func drainLib(sc *scen, p uint8) {
	pl := &sc.s.pl[p]
	for pl.libLo != pl.libHi {
		pl.libLo++
	}
}

func TestReplDrawReplEmptyLibraryScenarios(t *testing.T) {
	t.Run("the counter program replaces the empty draw", func(t *testing.T) {
		sc, id := newSetScenTweak(t, "draw-repl", []cardset.Entry{
			{Name: "Ormos, Archive Keeper", Count: 1}, {Name: "Plains", Count: 8}}, nil)
		ormos := id["Ormos, Archive Keeper"]
		h := sc.perm(ormos, 0)
		if !sc.onBF(h) {
			t.Fatal("precondition: the counter-program host is not on the battlefield")
		}
		drainLib(sc, 0)
		if pl := &sc.s.pl[0]; pl.libLo != pl.libHi {
			t.Fatal("precondition: seat 0's library is not empty")
		}
		sc.s.draw(0)
		if got := sc.s.perms[sc.slot(h)].ctrLow(int(kop.CtrP1P1)); got != 5 {
			t.Fatalf("Ormos holds %d +1/+1 counters, want 5 (the program ran in the draw's place)", got)
		}
		if sc.s.pl[0].mark&pmDrewEmpty != 0 {
			t.Fatal("the replaced empty-library draw marked the player (CR 614.11)")
		}
	})
	for _, kd := range []struct {
		name string
		kind uint8
	}{{"spell", seSpell}, {"ability", seAbility}, {"trigger", seTrigger}} {
		t.Run("the counter body's Self is the permanent inside a resolving "+kd.name, func(t *testing.T) {
			// gorge runs a replacement body with the replacement permanent as
			// its source (Ormos's PutCounter on Self under a resolving Deep
			// Analysis counters Ormos, not the resolving entry). An ability or
			// trigger entry carries its own source's queued version, which
			// differs from Ormos's.
			sc, id := newSetScenTweak(t, "draw-repl", []cardset.Entry{
				{Name: "Ormos, Archive Keeper", Count: 1}, {Name: "Plains", Count: 8}}, nil)
			ormos := id["Ormos, Archive Keeper"]
			h := sc.perm(ormos, 0)
			src := sc.obj(id["Plains"], 0)
			sc.s.objs[src].zone = kop.ZoneStack
			if kd.kind != seSpell {
				sc.s.objs[src].zone = kop.ZoneBattlefield
			}
			sc.s.res = resCtx{active: true, e: stackEntry{
				kind: kd.kind, obj: src, srcVer: sc.s.objs[src].ver + 7, card: id["Plains"], ctrl: 0, trigObj: NoObj}}
			if sc.s.objs[src].ver+7 == sc.s.objs[sc.s.perms[sc.slot(h)].obj].ver {
				t.Fatal("precondition: the entry's srcVer equals Ormos's version")
			}
			drainLib(sc, 0)
			sc.s.draw(0)
			if got := sc.s.perms[sc.slot(h)].ctrLow(int(kop.CtrP1P1)); got != 5 {
				t.Fatalf("Ormos holds %d +1/+1 counters inside a resolution, want 5", got)
			}
			if sc.s.pl[0].mark&pmDrewEmpty != 0 {
				t.Fatal("the replaced empty-library draw marked the player (CR 614.11)")
			}
		})
	}
	t.Run("the bodyless skip replaces the empty draw", func(t *testing.T) {
		sc, id := newSetScenTweak(t, "draw-repl", []cardset.Entry{
			{Name: "Living Conundrum", Count: 1}, {Name: "Plains", Count: 8}}, nil)
		lc := id["Living Conundrum"]
		if !sc.onBF(sc.perm(lc, 0)) {
			t.Fatal("precondition: the skip host is not on the battlefield")
		}
		drainLib(sc, 0)
		hand := sc.s.pl[0].handN
		sc.s.draw(0)
		if sc.s.pl[0].mark&pmDrewEmpty != 0 {
			t.Fatal("the skipped empty-library draw marked the player (CR 614.11)")
		}
		if sc.s.pl[0].handN != hand {
			t.Fatalf("hand grew to %d, want %d (the skip draws nothing)", sc.s.pl[0].handN, hand)
		}
	})
	t.Run("without the line the empty draw marks", func(t *testing.T) {
		// Control: the same draw with the host off the battlefield marks,
		// so the first two arms cannot pass because the mark is unreachable.
		sc, id := newSetScenTweak(t, "draw-repl", []cardset.Entry{
			{Name: "Ormos, Archive Keeper", Count: 1}, {Name: "Plains", Count: 8}}, nil)
		ormos := id["Ormos, Archive Keeper"]
		h := sc.perm(ormos, 0)
		sc.s.leave(sc.k, sc.slot(h), kop.ZoneGraveyard, false)
		drainLib(sc, 0)
		sc.s.draw(0)
		if sc.s.pl[0].mark&pmDrewEmpty == 0 {
			t.Fatal("control: the empty-library draw did not mark with Ormos gone")
		}
	})
}
