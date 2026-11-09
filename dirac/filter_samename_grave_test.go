package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// ratKingScen is a std-tmt scenario holding Rat King, Verminister and two
// other creatures; it returns the scenario, the card ids by name and Rat
// King's lowered graveyard sweep (target 0 and every card with its name).
func ratKingScen(t *testing.T) (*scen, map[string]CardID, kop.Instr) {
	t.Helper()
	set := cardsettest.Synth(t, "std-tmt")
	names := []string{"Rat King, Verminister", "Foot Elite", "Punk Frogs"}
	var deck []cardset.Entry
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s is not lowered in the std-tmt table", n)
		}
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
	sc := newScenTable(t, tbl, Config{NoElide: true})
	for _, a := range tbl.Cards[id["Rat King, Verminister"]].Abilities {
		for _, in := range a.Prog {
			if (in.Op == kop.OpChangeZoneAll || in.Op == kop.OpChangeZoneAllEx) && tbl.Filters[in.F].Pred == kop.PredSameNameT0 {
				return sc, id, in
			}
		}
	}
	t.Fatal("Rat King, Verminister has no PredSameNameT0 sweep")
	return nil, nil, kop.Instr{}
}

// resolveAt makes the scenario resolve Rat King's ability, controlled by
// seat 0, whose target 0 is o, chosen at version ver; the target's legality
// is taken as resolveTop takes it when the resolution starts.
func (sc *scen) resolveAt(t *testing.T, rk CardID, o Obj, ver uint8) {
	t.Helper()
	s := sc.s
	src := sc.perm(rk, 0)
	s.res = resCtx{active: true, lastTok: NoObj, eachObj: NoObj}
	s.res.e = stackEntry{kind: seAbility, obj: src, card: rk, ab: 0, ctrl: 0, nt: 1, srcVer: s.objs[src].ver}
	for i := range s.res.e.tgt {
		s.res.e.tgt[i] = NoTarget
	}
	s.res.e.tgt[0], s.res.e.tver[0] = uint8(o), ver
	if !s.targetStillLegal(sc.k, &s.res.e, 0) {
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
	g := sc.grave(id["Foot Elite"], 0, 3)
	other := sc.grave(id["Punk Frogs"], 0, 1)[0]
	theirs := sc.grave(id["Foot Elite"], 1, 1)[0]
	if s.pl[0].graveN != 4 || s.pl[1].graveN != 1 {
		t.Fatalf("setup: graveyards %d/%d, want 4/1", s.pl[0].graveN, s.pl[1].graveN)
	}
	sc.resolveAt(t, id["Rat King, Verminister"], g[1], s.objs[g[1]].ver)
	if s.res.bad&1 != 0 {
		t.Fatal("setup: the graveyard target is not legal")
	}
	s.execChangeZoneAll(sc.k, &in, false)
	for _, o := range g {
		ob := &s.objs[o]
		if ob.zone != kop.ZoneBattlefield {
			t.Fatalf("Foot Elite %d in zone %v, want Battlefield", o, ob.zone)
		}
		if s.perms[ob.slot].flags&pfTapped == 0 {
			t.Errorf("Foot Elite %d entered untapped", o)
		}
	}
	if z := s.objs[other].zone; z != kop.ZoneGraveyard {
		t.Errorf("Punk Frogs in zone %v, want Graveyard (another name)", z)
	}
	if z := s.objs[theirs].zone; z != kop.ZoneGraveyard {
		t.Errorf("the opponent's Foot Elite in zone %v, want Graveyard (your graveyard only)", z)
	}
}

// sameNameT0TargetGone: the target was exiled after it was chosen, so it is
// illegal as the resolution starts; nothing matches, not even the same-named
// cards left behind.
func sameNameT0TargetGone(t *testing.T) {
	sc, id, in := ratKingScen(t)
	s := sc.s
	g := sc.grave(id["Foot Elite"], 0, 3)
	ver := s.objs[g[0]].ver
	s.detach(g[0])
	s.place(g[0], kop.ZoneExile, false)
	sc.resolveAt(t, id["Rat King, Verminister"], g[0], ver)
	if s.res.bad&1 == 0 {
		t.Fatal("setup: the exiled target still reads as legal")
	}
	s.execChangeZoneAll(sc.k, &in, false)
	for _, o := range g[1:] {
		if z := s.objs[o].zone; z != kop.ZoneGraveyard {
			t.Errorf("Foot Elite %d in zone %v, want Graveyard (the target is gone)", o, z)
		}
	}
	if s.pl[0].graveN != 2 {
		t.Errorf("graveyard holds %d, want 2", s.pl[0].graveN)
	}
}
