package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// ratKingScen is an fdn scenario holding two creatures, Ambush Wolf and
// Axgard Cavalry, with Rat King, Verminister's return built by hand the way
// kgen lowers it (TestChangeZoneAllSameNameLowers pins that lowering): a
// sweep of Card.YouOwn graveyard cards reading PredSameNameT0 onto the
// battlefield tapped.
func ratKingScen(t *testing.T) (*scen, map[string]CardID, kop.Instr) {
	t.Helper()
	set := cardsettest.Set(t, "fdn")
	var deck []cardset.Entry
	for _, n := range []string{"Ambush Wolf", "Axgard Cavalry"} {
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	tbl.Filters = append(slices.Clip(tbl.Filters), kop.Filter{Owner: kop.CtrlYou, Pred: kop.PredSameNameT0, Zone: kop.ZoneGraveyard})
	in := kop.Instr{Op: kop.OpChangeZoneAllEx, C: int16(kop.ZoneBattlefield) | kop.MvTapped<<3, F: kop.FilterID(len(tbl.Filters) - 1)}
	return newScenTable(t, tbl, Config{NoElide: true}), id, in
}

// resolveAt makes the scenario resolve an ability controlled by seat 0
// whose target 0 is o, chosen at version ver; the target is illegal as the
// resolution starts when its version moved (resolveTop's res.bad, by
// targetStillLegal's version check).
func (sc *scen) resolveAt(o Obj, ver uint8) {
	s := sc.s
	s.res = resCtx{active: true, lastTok: NoObj, eachObj: NoObj}
	s.res.e = stackEntry{kind: seAbility, ctrl: 0, nt: 1}
	for i := range s.res.e.tgt {
		s.res.e.tgt[i] = uint8(NoTarget)
	}
	s.res.e.tgt[0], s.res.e.tver[0] = uint8(o), ver
	if s.objs[o].ver != ver {
		s.res.bad |= 1
	}
}

// TestSameNameT0Graveyard: Rat King's return takes the targeted graveyard
// card and every other card in its controller's graveyard with that name
// onto the battlefield tapped; a card with another name stays, and so does
// an opponent's same-named card (Defined$ You). A target that has left the
// graveyard since it was chosen returns nothing.
func TestSameNameT0Graveyard(t *testing.T) {
	t.Run("returns", sameNameT0Returns)
	t.Run("target_gone", sameNameT0TargetGone)
}

// sameNameT0Returns: the target sits between two same-named cards, so the
// card after it matches only if the sweep still reads the target's name
// once the target itself has moved.
func sameNameT0Returns(t *testing.T) {
	sc, id, in := ratKingScen(t)
	s := sc.s
	g := sc.grave(id["Ambush Wolf"], 0, 3)
	other := sc.grave(id["Axgard Cavalry"], 0, 1)[0]
	theirs := sc.grave(id["Ambush Wolf"], 1, 1)[0]
	if s.pl[0].graveN != 4 || s.pl[1].graveN != 1 {
		t.Fatalf("setup: graveyards %d/%d, want 4/1", s.pl[0].graveN, s.pl[1].graveN)
	}
	sc.resolveAt(g[1], s.objs[g[1]].ver)
	if s.res.bad&1 != 0 {
		t.Fatal("setup: the graveyard target is not legal")
	}
	s.execChangeZoneAll(sc.k, &in, false)
	for _, o := range g {
		ob := &s.objs[o]
		if ob.zone != kop.ZoneBattlefield {
			t.Fatalf("Ambush Wolf %d in zone %v, want Battlefield", o, ob.zone)
		}
		if s.perms[ob.slot].flags&pfTapped == 0 {
			t.Errorf("Ambush Wolf %d entered untapped", o)
		}
	}
	if z := s.objs[other].zone; z != kop.ZoneGraveyard {
		t.Errorf("Axgard Cavalry in zone %v, want Graveyard (another name)", z)
	}
	if z := s.objs[theirs].zone; z != kop.ZoneGraveyard {
		t.Errorf("the opponent's Ambush Wolf in zone %v, want Graveyard (your graveyard only)", z)
	}
}

// sameNameT0TargetGone: the target was exiled after it was chosen, so it is
// illegal as the resolution starts; nothing matches, not even the same-named
// cards left behind.
func sameNameT0TargetGone(t *testing.T) {
	sc, id, in := ratKingScen(t)
	s := sc.s
	g := sc.grave(id["Ambush Wolf"], 0, 3)
	ver := s.objs[g[0]].ver
	s.detach(g[0])
	s.place(g[0], kop.ZoneExile, false)
	sc.resolveAt(g[0], ver)
	if s.res.bad&1 == 0 {
		t.Fatal("setup: the exiled target still reads as legal")
	}
	s.execChangeZoneAll(sc.k, &in, false)
	for _, o := range g[1:] {
		if z := s.objs[o].zone; z != kop.ZoneGraveyard {
			t.Errorf("Ambush Wolf %d in zone %v, want Graveyard (the target is gone)", o, z)
		}
	}
	if s.pl[0].graveN != 2 {
		t.Errorf("graveyard holds %d, want 2", s.pl[0].graveN)
	}
}
