package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAltOriginGraveHandElection: the Graveyard,Hand pair's election offers
// the library, the graveyard and the hand; electing the two alternatives
// searches both, applies DifferentNames$ across them (a second card with an
// already-picked name in the other zone is not offered again), and owes no
// library shuffle.
func TestAltOriginGraveHandElection(t *testing.T) {
	var glass, capCard CardID
	sc, prog := czScenWith(t, func(tbl *kop.Table) []kop.Instr {
		glass, capCard = CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
		tbl.Cards = append(tbl.Cards,
			kop.CardDef{Name: "Test Glass", Types: kop.TArtifact},
			kop.CardDef{Name: "Test Cap", Types: kop.TArtifact})
		tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TArtifact})
		return []kop.Instr{{
			Op: kop.OpSearchEx, Obj: kop.RefController, A: kop.Const(2),
			B: kop.Const(kop.SrAltOrigin | kop.SrAltGraveHand<<kop.SrAltZoneShift | kop.SrDifferentNames),
			F: kop.FilterID(len(tbl.Filters) - 1),
			C: int16(kop.ZoneBattlefield),
		}}
	})
	s := sc.s
	// The pool's two names sit across the elected zones: one Glass in the
	// graveyard, another in the hand, one Cap in the graveyard. No library
	// card matches the filter.
	glassG := sc.grave(glass, 0, 1)[0]
	glassH := sc.hand(glass, 0)
	capG := sc.grave(capCard, 0, 1)[0]
	if s.objs[glassG].zone != kop.ZoneGraveyard || s.objs[glassH].zone != kop.ZoneHand || s.objs[capG].zone != kop.ZoneGraveyard {
		t.Fatalf("setup: zones %d/%d/%d, want graveyard, hand, graveyard",
			s.objs[glassG].zone, s.objs[glassH].zone, s.objs[capG].zone)
	}
	pl := &s.pl[0]
	libBefore := slices.Clone(pl.lib[pl.libLo:pl.libHi])

	if parked := s.exec(sc.k, &prog[0]); parked || s.res.ask != raGenKczZones {
		t.Fatalf("exec: parked %v ask %d, want the zone election", parked, s.res.ask)
	}
	zoneDecision := func() Decision {
		var d Decision
		s.resolveOptionsGen(sc.k, &d)
		return d
	}
	choose := func(mode uint8) {
		t.Helper()
		d := zoneDecision()
		for _, a := range d.Options[:d.N] {
			if a.Kind() == AkChoose && a.Mode() == mode {
				s.applyResolveGen(sc.k, a)
				return
			}
		}
		t.Fatalf("zone option %d not offered: %v", mode, d.Options[:d.N])
	}

	d := zoneDecision()
	if d.Sub != ChGenKczZone || d.N != 3 {
		t.Fatalf("election: sub %d n %d, want the three zones", d.Sub, d.N)
	}
	choose(1) // graveyard
	d = zoneDecision()
	if d.N != 3 { // library, hand, Done
		t.Fatalf("after the graveyard: n %d, want library, hand and Done", d.N)
	}
	choose(2) // hand
	d = zoneDecision()
	if d.N != 2 || d.Options[0].Kind() != AkChoose || d.Options[1].Kind() != AkDone {
		t.Fatalf("after both alternatives: %v, want the library and Done", d.Options[:d.N])
	}
	s.applyResolveGen(sc.k, MakeAction(AkDone, 0, 0, 0))

	if s.res.ask != raGenSearch || s.g.pickLeft != 2 {
		t.Fatalf("after the election: ask %d pickLeft %d, want a two-card search", s.res.ask, s.g.pickLeft)
	}
	d = zoneDecision()
	if d.Sub != ChGenSearch || d.N != 4 { // both Glasses, the Cap, Done
		t.Fatalf("first pick: sub %d n %d, want both Glasses, the Cap and Done", d.Sub, d.N)
	}
	var picked bool
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkChoose && Obj(a.Source()) == glassG {
			s.applyResolveGen(sc.k, a)
			picked = true
			break
		}
	}
	if !picked {
		t.Fatal("the graveyard Glass was not offered")
	}
	d = zoneDecision()
	if d.N != 2 { // the Cap and Done: the hand Glass shares the picked name
		t.Fatalf("second pick: n %d, want only the Cap and Done", d.N)
	}
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkChoose {
			if Obj(a.Source()) != capG {
				t.Fatalf("second pick offers %d, want only the Cap", a.Source())
			}
			s.applyResolveGen(sc.k, a)
		}
	}

	for _, o := range []Obj{glassG, capG} {
		if s.objs[o].zone != kop.ZoneBattlefield {
			t.Errorf("picked card %d zone %d, want the battlefield", o, s.objs[o].zone)
		}
	}
	if s.objs[glassH].zone != kop.ZoneHand {
		t.Errorf("the unpicked same-name Glass zone %d, want the hand", s.objs[glassH].zone)
	}
	if !slices.Equal(pl.lib[pl.libLo:pl.libHi], libBefore) {
		t.Errorf("library changed: %v -> %v", libBefore, pl.lib[pl.libLo:pl.libHi])
	}
	if pl.graveN != 0 || pl.handN != 1 {
		t.Errorf("graveyard %d hand %d, want only the unpicked Glass left in hand", pl.graveN, pl.handN)
	}
}
