package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDeathReturnIgnoresReenteredCard: Athreos, God of Passage's "whenever
// another creature you own dies, return it to your hand" names the creature
// that died. When that card comes back to the battlefield and dies again
// before the first trigger resolves, the card in the graveyard is a new
// object (CR 400.7): the first trigger's return finds nothing, and the card
// stays in the graveyard (gorge's TriggeredNewCardLKICopy; dr-mbc-aabb pair
// 148, Kaya's Ghostform returning Bushmeat Poacher).
func TestDeathReturnIgnoresReenteredCard(t *testing.T) {
	deck := []cardset.Entry{{Name: "Athreos, God of Passage", Count: 1}, {Name: "Bushmeat Poacher", Count: 1}, {Name: "Swamp", Count: 8}}
	tbl, da, _, err := synthSet(t, "dr-mbc-aabb").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	ath, poacher := id["Athreos, God of Passage"], id["Bushmeat Poacher"]
	ab := &tbl.Cards[ath].Abilities[0]
	ret := -1
	for i := range ab.Prog {
		if ab.Prog[i].Op == kop.OpReturn && ab.Prog[i].Obj == kop.RefTriggerObj {
			ret = i
		}
	}
	if ab.Kind != kop.AbTriggered || ab.Trig.Event != kop.TrigDies || ret < 0 {
		t.Fatal("precondition: Athreos's first ability is not a dies trigger returning the trigger object")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s, k := sc.s, sc.k
	a := sc.perm(ath, 0)
	p := sc.perm(poacher, 0)

	// the first death queues Athreos's trigger on the Poacher
	s.npend = 0
	s.destroy(k, sc.slot(p))
	if s.npend != 1 || s.pend[0].trigObj != p || s.objs[p].zone != kop.ZoneGraveyard {
		t.Fatalf("precondition: the Poacher's death queued %d triggers (zone %d), want Athreos's one on it", s.npend, s.objs[p].zone)
	}
	pt := s.pend[0] // kept pending below: it holds the death's snapshot (lkiSweep)
	resolve := func() {
		s.res = resCtx{e: stackEntry{kind: seTrigger, obj: a, srcVer: pt.srcVer, card: ath, ab: 0, ctrl: 0,
			trigObj: pt.trigObj, li: pt.li}, active: true, lastTok: NoObj, eachObj: NoObj}
		s.execReturnObj(k, &ab.Prog[ret], NoSlot, p)
	}

	// the control: the card that died is still in the graveyard, so the
	// return finds it (the fork leaves the scenario as it was)
	s2 := s.fork()
	s2.res = resCtx{e: stackEntry{kind: seTrigger, obj: a, srcVer: pt.srcVer, card: ath, ab: 0, ctrl: 0,
		trigObj: pt.trigObj, li: pt.li}, active: true, lastTok: NoObj, eachObj: NoObj}
	s2.execReturnObj(k, &ab.Prog[ret], NoSlot, p)
	if z := s2.objs[p].zone; z != kop.ZoneHand {
		s2.unfork()
		t.Fatalf("precondition: the return left the card that died in zone %d, want the hand", z)
	}
	s2.unfork()

	// the Poacher returns to the battlefield and dies again: a new object
	v0 := s.objs[p].ver
	s.enter(k, p, 0, false, s.detach(p))
	s.npend = 1
	s.destroy(k, sc.slot(p))
	s.npend = 1
	if s.objs[p].zone != kop.ZoneGraveyard || s.objs[p].ver == v0 || s.lk(pt.li).obj != p {
		t.Fatalf("precondition: the Poacher is in zone %d at version %d (was %d), want the graveyard at a later version", s.objs[p].zone, s.objs[p].ver, v0)
	}
	resolve()
	if z := s.objs[p].zone; z != kop.ZoneGraveyard {
		t.Fatalf("the first death's return moved the card that died again to zone %d, want it left in the graveyard (CR 400.7)", z)
	}
}
