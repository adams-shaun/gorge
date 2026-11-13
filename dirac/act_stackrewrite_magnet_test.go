package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestKsrMagnetAsksWhichTarget: Spellskite's controller picks which of Arc
// Lightning's three targets it takes (CR 115.7b); the ask offers exactly the
// three targets, and the pick's place alone becomes Spellskite.
func TestKsrMagnetAsksWhichTarget(t *testing.T) {
	set := cardsettest.Set(t, "retarget")
	var deck []cardset.Entry
	for _, n := range []string{"Arc Lightning", "Spellskite", "Grizzly Bears", "Hill Giant", "Mountain"} {
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
	for pick := 0; pick < 3; pick++ {
		sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true})
		for i := 0; i < 3; i++ {
			sc.perm(id["Mountain"], 0)
		}
		skite := sc.perm(id["Spellskite"], 1)
		bears := sc.perm(id["Grizzly Bears"], 1)
		giant := sc.perm(id["Hill Giant"], 1)
		foe := PlayerTarget0 + 1
		targets := []uint8{uint8(bears), uint8(giant), foe}
		sc.castCard(t, sc.hand(id["Arc Lightning"], 0))
		left := slices.Clone(targets)
		activated := false
		var ask Decision
		for i := 0; i < 60 && ask.N == 0; i++ {
			d := sc.s.Step()
			a := d.Options[0]
			switch {
			case d.Kind == DkChoose && d.Sub == ChKsrTarget:
				ask = d
				continue
			case d.Kind == DkTarget && len(left) > 0:
				for j := 0; j < int(d.N); j++ {
					if o := d.Options[j]; o.Kind() == AkTarget && slices.Contains(left, o.Target()) {
						a = o
						left = slices.DeleteFunc(left, func(c uint8) bool { return c == o.Target() })
						break
					}
				}
			case d.Kind == DkTarget && !activated:
				a = MakeAction(AkDone, 0, 0, 0)
			case d.Kind == DkPriority && d.Player == 1 && !activated:
				a, activated = MakeAction(AkActivate, uint8(skite), 0, 0), true
			}
			sc.s.Apply(a)
		}
		if ask.N == 0 {
			t.Fatal("Spellskite resolved without asking which of three targets to change")
		}
		var offered []uint8
		for j := 0; j < int(ask.N); j++ {
			if a := ask.Options[j]; a.Kind() != AkChoose || a.Target() != 0 || a.Mode() != 0 {
				t.Fatalf("option %v is not a plain target pick", a)
			} else {
				offered = append(offered, a.Source())
			}
		}
		slices.Sort(offered)
		wantOffered := slices.Clone(targets)
		slices.Sort(wantOffered)
		if ask.Player != 1 || !slices.Equal(offered, wantOffered) {
			t.Fatalf("ask player %d offers %v, want seat 1 and exactly the places %v", ask.Player, offered, wantOffered)
		}
		sc.s.Apply(MakeAction(AkChoose, targets[pick], 0, 0))
		var got [MaxOptions]uint8
		n := sc.s.copySlotTargets(&sc.s.stack[0], 0, &got)
		want := slices.Clone(targets)
		want[pick] = uint8(skite)
		if !slices.Equal(got[:n], want) {
			t.Fatalf("picked %d: Arc Lightning's targets %v, want %v", pick, got[:n], want)
		}
	}
}
