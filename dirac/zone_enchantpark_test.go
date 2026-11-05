package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEnchantParkTrimsEnteredCards: a found-card move onto the battlefield that
// enters a non-Aura card and then parks at an Aura's raGenEnchant ask leaves
// only the cards still to move in the look, as every sibling park does. Before,
// the answered re-run began at the card that had already entered and left it
// from a slot already detached ("kernel: place into unsupported zone").
func TestEnchantParkTrimsEnteredCards(t *testing.T) {
	deck := []cardset.Entry{{Name: "Witness Protection", Count: 1}, {Name: "Eager Trufflesnout", Count: 2}, {Name: "Plains", Count: 17}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Mover", Types: kop.TSorcery, Abilities: []kop.Ability{
		{Kind: kop.AbSpell, Prog: []kop.Instr{{Op: kop.OpSearchEx}}}}})

	for _, tail := range []bool{false, true} {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		s := sc.s
		a, b := sc.hand(id["Plains"], 0), sc.hand(id["Witness Protection"], 0)
		look := []Obj{a, b}
		var c Obj = NoObj
		if tail {
			c = sc.hand(id["Plains"], 0)
			look = append(look, c)
		}
		h1, h2 := sc.perm(id["Eager Trufflesnout"], 0), sc.perm(id["Eager Trufflesnout"], 1)
		s.res = resCtx{e: stackEntry{kind: seSpell, obj: NoObj, card: src, ctrl: 0, trigObj: NoObj},
			active: true, lastTok: NoObj, eachObj: NoObj}
		s.res.e.tgt = noTargets
		s.ctl = ctlResolve
		s.g.pickP, s.g.pickZ, s.g.pickF = 0, uint8(kop.ZoneBattlefield), 0
		copy(s.resArm().look[:], look)
		s.g.nlook = uint8(len(look))
		if !s.handAuraArmed(sc.k) || s.objs[b].zone != kop.ZoneHand || s.objs[a].zone != kop.ZoneHand {
			t.Fatalf("tail=%v: want the OpSearchEx armed and the cards in the hand", tail)
		}

		s.moveFoundAns(sc.k, NoSlot)
		if s.res.ask != raGenEnchant {
			t.Fatalf("tail=%v: ask %d, want the enchant ask", tail, s.res.ask)
		}
		if !sc.onBF(a) || s.objs[b].zone != kop.ZoneHand {
			t.Fatalf("tail=%v: after the park A on bf %v, B zone %d; want A entered and B waiting in the hand", tail, sc.onBF(a), s.objs[b].zone)
		}
		if tail && s.objs[c].zone != kop.ZoneHand {
			t.Fatalf("after the park the card behind the Aura is in zone %d, want the hand", s.objs[c].zone)
		}
		if want := uint8(len(look) - 1); s.g.nlook != want || s.resArm().look[0] != b {
			t.Fatalf("tail=%v: nlook %d look[0] %d, want %d starting at the parked Aura %d", tail, s.g.nlook, s.resArm().look[0], want, b)
		}
		aSlot, host := sc.slot(a), sc.slot(h2)
		if sc.slot(h1) == host {
			t.Fatal("the two hosts share a slot")
		}

		s.res.ask = raNone
		s.moveFoundAns(sc.k, host)
		if !sc.onBF(b) || s.perms[sc.slot(b)].attached != host {
			t.Fatalf("tail=%v: Aura on bf %v attached %d, want on the battlefield attached to %d", tail, sc.onBF(b), s.perms[sc.slot(b)].attached, host)
		}
		if !sc.onBF(a) || sc.slot(a) != aSlot {
			t.Fatalf("tail=%v: A on bf %v slot %d, want still in slot %d", tail, sc.onBF(a), sc.slot(a), aSlot)
		}
		if tail && !sc.onBF(c) {
			t.Fatalf("the card behind the Aura is in zone %d, want the battlefield", s.objs[c].zone)
		}
		if s.g.nlook != 0 {
			t.Fatalf("tail=%v: nlook %d after the answer, want 0", tail, s.g.nlook)
		}
	}
}
