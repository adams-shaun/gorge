package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMayPlayRememberedBothZones: Ark of Hunger's lowered grant carries one
// OpMayPlay per zone, so a remembered card in the graveyard and one in exile
// each get exactly one permission, and a card that changes zone loses it (its
// record's version goes stale).
func TestMayPlayRememberedBothZones(t *testing.T) {
	set := cardsettest.Synth(t, "std-sos")
	deck := []cardset.Entry{{Name: "Ark of Hunger", Count: 1}, {Name: "Plains", Count: 39}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatalf("Ark of Hunger is not lowered: %v", err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	ark, ok := id["Ark of Hunger"]
	if !ok {
		t.Fatal("Ark of Hunger is not in the SOS table")
	}
	var plays []kop.Instr
	for _, a := range tbl.Cards[ark].Abilities {
		for _, in := range a.Prog {
			if in.Op == kop.OpMayPlay {
				plays = append(plays, in)
			}
		}
	}
	if len(plays) != 2 || plays[0].C != int16(kop.ZoneGraveyard) || plays[1].C != int16(kop.ZoneExile) {
		t.Fatalf("Ark of Hunger lowered to %v, want one MayPlay per zone", plays)
	}
	// The two remembered cards stand in for more than Ark's Mill 1; clear the
	// one-card ExileOnMoved flag so both zone instructions run in this test.
	for i := range plays {
		plays[i].B = kop.ValueRef{}
	}

	pl := id["Plains"]
	var lib [20]CardID
	for i := range lib {
		lib[i] = pl
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	for p := uint8(0); p < 2; p++ {
		pls := &s.pl[p]
		for pls.handN > 0 {
			o := pls.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pls.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	k := s.k()

	gy, ex := s.pl[0].lib[0], s.pl[0].lib[1]
	for _, m := range []struct {
		o Obj
		z kop.Zone
	}{{gy, kop.ZoneGraveyard}, {ex, kop.ZoneExile}} {
		s.detach(m.o)
		s.place(m.o, m.z, false)
	}
	s.g.rem[0], s.g.rem[1], s.g.nrem = gy, ex, 2
	s.mayPlayRemembered(k, &plays[0], 0)
	s.mayPlayRemembered(k, &plays[1], 0)

	valid := func(o Obj) int {
		n := 0
		for i := 0; i < int(s.g.nmay); i++ {
			if r := s.g.may[i]; r.obj == o && r.ver == s.objs[o].ver {
				n++
			}
		}
		return n
	}
	if s.g.nmay != 2 || s.g.may[0].obj != gy || s.g.may[1].obj != ex {
		t.Fatalf("permissions %d (%v), want the graveyard card then the exiled one", s.g.nmay, s.g.may[:s.g.nmay])
	}
	if a, b := valid(gy), valid(ex); a != 1 || b != 1 {
		t.Fatalf("valid permissions %d/%d, want one for each remembered card", a, b)
	}

	// The graveyard card moves on: its record's version no longer matches.
	s.detach(gy)
	s.place(gy, kop.ZoneExile, false)
	if n := valid(gy); n != 0 {
		t.Fatalf("the moved card kept %d permissions", n)
	}
	if n := valid(ex); n != 1 {
		t.Fatalf("the unmoved card has %d permissions, want one", n)
	}
}
