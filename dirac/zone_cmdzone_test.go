package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// commanderToZone seats p's commander on the battlefield and moves it to z as
// a destruction would, leaving the ask due (the card moved, so ver is above
// the watermark).
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
		if a.Kind() != AkChoose || a.Source() != uint8(o) || a.Mode() != wantMode {
			t.Fatalf("option %d = kind %d src %d mode %d, want AkChoose src %d mode %d", i, a.Kind(), a.Source(), a.Mode(), o, wantMode)
		}
	}
	if got := sc.s.pl[0].cmdAsk; got != sc.s.objs[o].ver {
		t.Fatalf("ask watermark %d, want the commander's ver %d", got, sc.s.objs[o].ver)
	}
	return d
}

func TestCommanderDiesAskOwner(t *testing.T) {
	sc, o := commanderToZone(t, kop.ZoneGraveyard)
	assertCommanderAsk(t, sc, o)
	sc.s.Apply(MakeAction(AkChoose, uint8(o), 0, 0))
	if z := sc.s.objs[o].zone; z != kop.ZoneCommand {
		t.Fatalf("mode 0 left the commander in zone %v, want the command zone", z)
	}
	if sc.s.pl[0].graveN != 0 {
		t.Fatalf("graveyard still holds %d cards after mode 0", sc.s.pl[0].graveN)
	}
	if d := sc.s.Step(); d.Kind == DkChoose && d.Sub == ChCommanderZone {
		t.Fatal("the commander was asked about again after moving to the command zone")
	}
}

func TestCommanderDiesAskOwnerDecline(t *testing.T) {
	sc, o := commanderToZone(t, kop.ZoneGraveyard)
	assertCommanderAsk(t, sc, o)
	ver := sc.s.objs[o].ver
	sc.s.Apply(MakeAction(AkChoose, uint8(o), 0, 1))
	if z := sc.s.objs[o].zone; z != kop.ZoneGraveyard {
		t.Fatalf("mode 1 moved the commander to zone %v, want the graveyard", z)
	}
	if sc.s.pl[0].graveN != 1 {
		t.Fatalf("graveyard holds %d cards, want the declined commander", sc.s.pl[0].graveN)
	}
	if sc.s.objs[o].ver != ver {
		t.Fatalf("mode 1 changed ver %d -> %d, want it unmoved", ver, sc.s.objs[o].ver)
	}
	if d := sc.s.Step(); d.Kind == DkChoose && d.Sub == ChCommanderZone {
		t.Fatal("a declined commander was asked about again without moving")
	}
}

func TestCommanderExiledAsk(t *testing.T) {
	sc, o := commanderToZone(t, kop.ZoneExile)
	assertCommanderAsk(t, sc, o)
	sc.s.Apply(MakeAction(AkChoose, uint8(o), 0, 0))
	if z := sc.s.objs[o].zone; z != kop.ZoneCommand {
		t.Fatalf("mode 0 left the commander in zone %v, want the command zone", z)
	}
	if sc.s.pl[0].exileN != 0 {
		t.Fatalf("exile still holds %d cards after mode 0", sc.s.pl[0].exileN)
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
