package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDeferredRiderQueuesAfterCastTriggers: Pyromancer's Goggles' {R} pays a
// Shock while a Monastery Swiftspear that entered after the Goggles is out.
// onCast's listener walk meets the Goggles first, but gorge queues the
// TriggersWhenSpent$ rider apart, after the ordinary cast triggers and before
// cascade (cast_commit.go fireManaSpentTriggers), so the pending queue holds
// the prowess trigger, then the rider. (The lockstep cannot see this: both
// sides pose trigger-order options sorted canonically.)
func TestDeferredRiderQueuesAfterCastTriggers(t *testing.T) {
	deck := []cardset.Entry{{Name: "Pyromancer's Goggles", Count: 1}, {Name: "Monastery Swiftspear", Count: 1},
		{Name: "Shock", Count: 1}, {Name: "Mountain", Count: 20}}
	tbl, da, _, err := cardsettest.Set(t, "goggles").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true})
	gog, swift, shock := id["Pyromancer's Goggles"], id["Monastery Swiftspear"], id["Shock"]
	if !sc.k.gen.anyDeferred || !sc.k.gen.has(gog, gfDeferred) {
		t.Fatalf("anyDeferred %v, Goggles gfDeferred %v: want both", sc.k.gen.anyDeferred, sc.k.gen.has(gog, gfDeferred))
	}
	g := sc.perm(gog, 0)
	sw := sc.perm(swift, 0)
	var ord [MaxPerm]Slot
	if n := sc.s.listenerOrder(slotMask{}.with(sc.slot(g)).with(sc.slot(sw)), &ord); n != 2 || ord[0] != sc.slot(g) {
		t.Fatalf("listener order %v: want the Goggles first", ord[:n])
	}
	o := sc.obj(shock, 0)
	sc.kcastBuild(o, shock, 0)
	sc.s.bld().obsSl = sc.slot(g) + 1 // the payment spent the Goggles' {R}
	e := sc.s.bld().e
	sc.s.onCast(sc.k, &e)
	if sc.s.npend != 2 {
		t.Fatalf("%d pending: want the prowess trigger and the rider", sc.s.npend)
	}
	p0, p1 := sc.s.pend[0], sc.s.pend[1]
	rider := &sc.k.t.Cards[gog].Abilities[p1.ab]
	if p0.card != swift || p1.card != gog || rider.Trig.Flags2&kop.Tf2ManaFromSelf == 0 {
		t.Fatalf("pending cards %d then %d (ab %d): want Swiftspear %d, then the Goggles %d rider", p0.card, p1.card, p1.ab, swift, gog)
	}
}
