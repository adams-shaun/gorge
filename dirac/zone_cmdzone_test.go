package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// commanderToZoneSeat seats p's commander on the battlefield and moves it to z
// as a destruction would, leaving the ask due (the card moved, so ver is
// above the watermark).
func commanderToZoneSeat(t *testing.T, p uint8, z kop.Zone) (*scen, Obj) {
	t.Helper()
	tbl, card := commanderTable(0)
	sc := newCommanderScen(t, tbl, card, true)
	s := sc.s
	o := s.pl[p].cmd
	if o == NoObj || s.objs[o].zone != kop.ZoneCommand {
		t.Fatalf("commander not seated in the command zone: obj %d zone %v", o, s.objs[o].zone)
	}
	s.detach(o)
	sl := s.enter(sc.k, o, p, false, kop.ZoneCommand)
	s.perms[sl].flags &^= pfSick
	s.leave(sc.k, sl, z, false)
	if s.objs[o].zone != z {
		t.Fatalf("commander in zone %v, want %v", s.objs[o].zone, z)
	}
	if s.objs[o].ver == s.pl[p].cmdAsk {
		t.Fatalf("commander arrived with ver %d still at the ask watermark", s.objs[o].ver)
	}
	return sc, o
}

func commanderToZone(t *testing.T, z kop.Zone) (*scen, Obj) {
	t.Helper()
	return commanderToZoneSeat(t, 0, z)
}

// assertCommanderAsk: the next decision is the owner's two-option CR 903.9a
// ask, in command_zone then leave order, naming the commander.
func assertCommanderAsk(t *testing.T, sc *scen, o Obj) Decision {
	t.Helper()
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChCommanderZone {
		t.Fatalf("decision kind=%d sub=%d, want ChCommanderZone", d.Kind, d.Sub)
	}
	if d.Player != 0 {
		t.Fatalf("asked player %d, want the commander's owner 0", d.Player)
	}
	if d.N != 2 {
		t.Fatalf("%d options, want exactly 2", d.N)
	}
	for i, wantMode := range [2]uint8{0, 1} {
		a := d.Options[i]
		if a.Kind() != AkChoose || a.Source() != TCode(uint8(o)) || a.Mode() != wantMode {
			t.Fatalf("option %d = kind %d src %d mode %d, want AkChoose src %d mode %d", i, a.Kind(), a.Source(), a.Mode(), o, wantMode)
		}
	}
	if got := sc.s.pl[0].cmdAsk; got != sc.s.objs[o].ver {
		t.Fatalf("ask watermark %d, want the commander's ver %d", got, sc.s.objs[o].ver)
	}
	return d
}

// assertCommanderMode answers the ask and checks the branch it takes; after
// either answer the commander sits still, so no second ask may follow.
func assertCommanderMode(t *testing.T, sc *scen, o Obj, mode uint8) {
	t.Helper()
	s := sc.s
	from := s.objs[o].zone
	ver := s.objs[o].ver
	s.Apply(MakeAction(AkChoose, TCode(o), 0, mode))
	switch mode {
	case 0:
		if z := s.objs[o].zone; z != kop.ZoneCommand {
			t.Fatalf("mode 0 left the commander in zone %v, want the command zone", z)
		}
		if from == kop.ZoneGraveyard && s.pl[0].graveN != 0 {
			t.Fatalf("graveyard still holds %d cards after mode 0", s.pl[0].graveN)
		}
		if from == kop.ZoneExile && s.pl[0].exileN != 0 {
			t.Fatalf("exile still holds %d cards after mode 0", s.pl[0].exileN)
		}
		if s.objs[o].ver == ver {
			t.Fatalf("mode 0 did not bump ver %d for the command-zone arrival", ver)
		}
	case 1:
		if z := s.objs[o].zone; z != from {
			t.Fatalf("mode 1 moved the commander to zone %v, want %v", z, from)
		}
		if s.objs[o].ver != ver {
			t.Fatalf("mode 1 changed ver %d -> %d, want it unmoved", ver, s.objs[o].ver)
		}
	}
	if d := s.Step(); d.Kind == DkChoose && d.Sub == ChCommanderZone {
		t.Fatal("the commander was asked about again without moving")
	}
}

func TestCommanderDiesAskOwner(t *testing.T) {
	for _, mode := range [2]uint8{0, 1} {
		t.Run([2]string{"command zone", "leave"}[mode], func(t *testing.T) {
			sc, o := commanderToZone(t, kop.ZoneGraveyard)
			assertCommanderAsk(t, sc, o)
			assertCommanderMode(t, sc, o, mode)
		})
	}
}

func TestCommanderExiledAsk(t *testing.T) {
	for _, mode := range [2]uint8{0, 1} {
		t.Run([2]string{"command zone", "leave"}[mode], func(t *testing.T) {
			sc, o := commanderToZone(t, kop.ZoneExile)
			assertCommanderAsk(t, sc, o)
			assertCommanderMode(t, sc, o, mode)
		})
	}
}

// TestCommanderZoneArrivalOrder: with both commanders due at once, gorge asks
// the one that arrived first, not seat 0 first.
func TestCommanderZoneArrivalOrder(t *testing.T) {
	tbl, card := commanderTable(0)
	sc := newCommanderScen(t, tbl, card, true)
	s := sc.s
	die := func(p uint8) {
		o := s.pl[p].cmd
		s.detach(o)
		sl := s.enter(sc.k, o, p, false, kop.ZoneCommand)
		s.perms[sl].flags &^= pfSick
		s.leave(sc.k, sl, kop.ZoneGraveyard, false)
	}
	die(1) // seat 1 arrives first
	die(0)
	d := s.Step()
	if d.Kind != DkChoose || d.Sub != ChCommanderZone || d.Player != 1 {
		t.Fatalf("decision kind=%d sub=%d player=%d, want seat 1's commander-zone ask", d.Kind, d.Sub, d.Player)
	}
}

// TestCommanderZoneAskBeforeLegend: one resolution can leave a commander in
// the graveyard and a legend duplicate on the battlefield, so both asks are
// owed at the same pass start. Gorge's commanderZoneSBA runs before the pass
// parks its legend batch, so the commander ask comes first; declining it then
// leaves the legend ask for the re-run.
func TestCommanderZoneAskBeforeLegend(t *testing.T) {
	tbl, card := commanderTable(0)
	tbl.Cards[card].Types |= kop.TLegendary
	sc := newCommanderScen(t, tbl, card, true)
	s := sc.s
	o := s.pl[0].cmd
	s.detach(o)
	sl := s.enter(sc.k, o, 0, false, kop.ZoneCommand)
	s.perms[sl].flags &^= pfSick
	s.leave(sc.k, sl, kop.ZoneGraveyard, false)
	if z := s.objs[o].zone; z != kop.ZoneGraveyard {
		t.Fatalf("setup: commander in zone %v, want the graveyard", z)
	}
	if s.objs[o].ver == s.pl[0].cmdAsk {
		t.Fatalf("setup: commander ver %d still at the ask watermark", s.objs[o].ver)
	}
	sc.perm(card, 0)
	sc.perm(card, 0)
	if !s.legendOwed(sc.k) {
		t.Fatal("setup: the legend duplicate does not owe a keep ask")
	}
	d := s.Step()
	if d.Kind != DkChoose || d.Sub != ChCommanderZone || d.Player != 0 {
		t.Fatalf("first decision kind=%d sub=%d player=%d, want the commander-zone ask before the legend ask", d.Kind, d.Sub, d.Player)
	}
	s.Apply(MakeAction(AkChoose, TCode(o), 0, 1)) // leave it: the legend ask is still owed
	if d := s.Step(); d.Kind != DkChoose || d.Sub != ChLegend {
		t.Fatalf("after declining the commander, decision kind=%d sub=%d, want the legend keep ask", d.Kind, d.Sub)
	}
}
