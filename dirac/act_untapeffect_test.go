package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEffectUntapFiresUntapsTrigger: an Untap effect (OpUntap's core,
// untapSlot; gorge TryUntap's Untap event) on a tapped creature that the
// other player's Fishing Pole equips queues the Pole's "whenever equipped
// creature becomes untapped" trigger, controlled by the Pole's controller
// (CR 701.21c, 603.2); a stun counter taking the untap's place queues none
// (CR 122.1d).
func TestEffectUntapFiresUntapsTrigger(t *testing.T) {
	mk := func(t *testing.T) (*scen, Obj, Obj) {
		t.Helper()
		deck := []cardset.Entry{{Name: "Fishing Pole", Count: 1}, {Name: "Vanguard Seraph", Count: 1}, {Name: "Forest", Count: 20}}
		tbl, _, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
		if err != nil {
			t.Fatal(err)
		}
		sc := newScenTable(t, tbl, Config{NoElide: true})
		pole, bear := sc.idByName(t, "Fishing Pole"), sc.idByName(t, "Vanguard Seraph")
		hasUntaps := false
		for _, a := range sc.k.t.Cards[pole].Abilities {
			hasUntaps = hasUntaps || (a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigUntaps)
		}
		if !hasUntaps {
			t.Fatal("Fishing Pole compiled without an Untaps trigger")
		}
		b := sc.perm(bear, 0)
		e := sc.perm(pole, 1)
		sc.s.perms[sc.slot(e)].attached = sc.slot(b)
		sc.s.perms[sc.slot(b)].flags |= pfTapped
		if !sc.s.subs[kop.TrigUntaps].any() || sc.s.npend != 0 || !sc.tapped(b) {
			t.Fatalf("setup: subscribed %v pending %d tapped %v", sc.s.subs[kop.TrigUntaps].any(), sc.s.npend, sc.tapped(b))
		}
		return sc, b, e
	}

	t.Run("untap", func(t *testing.T) {
		sc, b, e := mk(t)
		sc.s.untapSlot(sc.k, sc.slot(b))
		if sc.tapped(b) {
			t.Fatal("the bear should untap")
		}
		if sc.s.npend != 1 {
			t.Fatalf("%d pending triggers, want the Pole's one", sc.s.npend)
		}
		if p := sc.s.pend[0]; p.obj != e || p.ctrl != 1 {
			t.Fatalf("pending trigger obj %d ctrl %d, want the Pole %d controlled by p1", p.obj, p.ctrl, e)
		}
	})
	t.Run("stun", func(t *testing.T) {
		sc, b, _ := mk(t)
		p := &sc.s.perms[sc.slot(b)]
		p.addCtrN(sc.s, int(kop.CtrStun), 1)
		sc.s.untapSlot(sc.k, sc.slot(b))
		if !sc.tapped(b) || p.ctrLow(int(kop.CtrStun)) != 0 {
			t.Fatalf("tapped %v stun %d, want tapped with the counter removed", sc.tapped(b), p.ctrLow(int(kop.CtrStun)))
		}
		if sc.s.npend != 0 {
			t.Fatalf("%d pending triggers, want none (CR 122.1d)", sc.s.npend)
		}
	})
}
