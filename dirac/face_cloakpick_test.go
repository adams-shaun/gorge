package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// kcloakScen is a State over the cloaksources pool's table at seat 0's main
// phase 1 of turn 2, with Vannifar's card id in ids (newScenTableLib empties
// both hands into their libraries).
func kcloakScen(t *testing.T, deck []cardset.Entry) (*scen, map[string]kop.CardID) {
	t.Helper()
	set := cardsettest.Set(t, "cloaksources")
	for _, e := range deck {
		if _, ok := set.Lookup(e.Name); !ok {
			t.Fatalf("%s not in the cloaksources pool", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]kop.CardID{}
	for _, c := range da {
		ids[tbl.Cards[c].Name] = c
	}
	var plains kop.CardID
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Plains" {
			plains = kop.CardID(i)
		}
	}
	if plains == 0 {
		names := make([]string, 0, len(tbl.Cards))
		for i := range tbl.Cards {
			names = append(names, tbl.Cards[i].Name)
		}
		t.Fatalf("no Plains in the cloaksources table: %q", names)
	}
	sc := newScenTableLib(t, tbl, plains, Config{NoElide: true})
	return sc, ids
}

// kcloakTrigger drives the game from Vannifar's BeginCombat trigger through
// its resolution: the Charm mode ask picks the cloak mode (the Charm's first
// listed mode), the cloak pick ask picks per pick, and the next priority
// round ends the drive. It reports whether the Charm mode ask was served, and
// fails on any other choice.
func kcloakTrigger(t *testing.T, sc *scen, pick func(d *Decision) (Action, bool)) (modeAsk bool) {
	t.Helper()
	for i := 0; i < 200; i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
			if modeAsk && sc.s.StackLen() == 0 {
				return modeAsk // the trigger resolved; the next round is ordinary play
			}
			continue
		}
		if d.Kind == DkModes {
			modeAsk = true
			sc.s.Apply(d.Options[0]) // the Charm's first listed mode: the cloak
			continue
		}
		if a, ok := pick(&d); ok {
			sc.s.Apply(a)
			return modeAsk // the answer runs the rest of the resolution synchronously
		}
		t.Fatalf("unhandled decision kind %d sub %d player %d opts %v", d.Kind, d.Sub, d.Player, d.Options[:d.N])
	}
	t.Fatal("Vannifar's trigger never resolved")
	return false
}

// TestCloakPickChosenCard: Vannifar's combat trigger takes the cloak mode,
// asks its controller which hand card to cloak (gorge's face_down_choice
// pick) and cloaks it from where it sits: face down, marked cloaked, under
// the cloaking player's control, owned by its owner (CR 701.58a, 110.2a).
func TestCloakPickChosenCard(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Vannifar, Evolved Enigma", Count: 2},
		{Name: "Grizzly Bears", Count: 4},
		{Name: "Ornithopter", Count: 4},
		{Name: "Plains", Count: 6},
	}
	sc, ids := kcloakScen(t, deck)
	s := sc.s
	van := sc.perm(ids["Vannifar, Evolved Enigma"], 0)
	bear := sc.hand(ids["Grizzly Bears"], 0)
	thr := sc.perm(ids["Ornithopter"], 0)
	sc.passUntil(t, kop.StepAttackers)
	modeAsk := kcloakTrigger(t, sc, func(d *Decision) (Action, bool) {
		if d.Sub != ChGenFaceDownPick {
			return Action(0), false
		}
		if d.N != 1 || d.Options[0].Kind() != AkChoose || d.Options[0].Source() != TCode(uint8(bear)) {
			t.Fatalf("pick ask over %d options, want exactly the one hand card %d: %v", d.N, bear, d.Options[:d.N])
		}
		return d.Options[0], true
	})
	if !modeAsk {
		t.Fatal("the Charm never asked its modes")
	}
	if s.objs[bear].zone != kop.ZoneBattlefield {
		t.Fatalf("the picked card is in zone %d, want the battlefield", s.objs[bear].zone)
	}
	sl := s.objs[bear].slot
	if !s.faceDown(sc.k, sl) || !s.cloaked(sl) {
		t.Fatal("the picked card did not enter face down and cloaked")
	}
	if s.pl[0].handN != 0 {
		t.Fatalf("hand has %d cards, want 0", s.pl[0].handN)
	}
	if s.perms[sl].ctrl != 0 {
		t.Fatalf("cloaked under player %d, want the cloaking player 0", s.perms[sl].ctrl)
	}
	if s.objs[bear].acts>>7 != 0 {
		t.Fatal("owner changed: the cloaked card must keep its owner")
	}
	if kw := s.kw(sc.k, sl); kw&kop.KwWard == 0 {
		t.Fatal("the cloaked permanent lacks ward {2}")
	}
	if !sc.onBF(van) || !sc.onBF(thr) {
		t.Fatal("Vannifar or the ornithopter left the battlefield")
	}
}

// TestCloakPickEmptyHand: with an empty hand the trigger still offers the
// cloak mode, but the resolution asks nothing and cloaks nothing (gorge
// chooseFromZoneForFaceDown's empty pool returns no ids).
func TestCloakPickEmptyHand(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Vannifar, Evolved Enigma", Count: 2},
		{Name: "Grizzly Bears", Count: 4},
		{Name: "Ornithopter", Count: 4},
		{Name: "Plains", Count: 6},
	}
	sc, ids := kcloakScen(t, deck)
	s := sc.s
	van := sc.perm(ids["Vannifar, Evolved Enigma"], 0)
	sc.passUntil(t, kop.StepAttackers)
	asked := false
	modeAsk := kcloakTrigger(t, sc, func(d *Decision) (Action, bool) {
		if d.Sub == ChGenFaceDownPick {
			asked = true
			return d.Options[0], true
		}
		return Action(0), false
	})
	if !modeAsk {
		t.Fatal("the Charm never asked its modes")
	}
	if asked {
		t.Fatal("an empty hand was asked for a cloak pick")
	}
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if s.faceDown(sc.k, sl) {
			t.Fatalf("slot %d entered face down with an empty hand", sl)
		}
	}
	if !sc.onBF(van) {
		t.Fatal("Vannifar left the battlefield")
	}
}
