//go:build dirac_mp

package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// Commander at more than two seats (CR 903): every seat's commander starts
// in the command zone, the CR 903.9a command-zone ask reaches seats 2 and 3
// (and a partner's slot), commander damage is tallied per opponent's
// commander, and a seat that leaves takes its commander with it.

// newCommanderScenN deals n 20-card decks of Forests with the commander at
// index 0 (and a partner at 1 for the seats in partners), clears the
// opening hands, and stands at seat 0's first main phase.
func newCommanderScenN(t *testing.T, n uint8, partners uint8) (*scen, CardID) {
	t.Helper()
	tbl, card := commanderTable(0)
	var decks [MaxSeats][]CardID
	cmds := noCommanders()
	for p := uint8(0); p < n; p++ {
		d := make([]CardID, 20)
		for i := range d {
			d[i] = v.Forest
		}
		d[0], d[1] = card, card
		decks[p] = d
		cmds[p] = [2]int16{0, -1}
		if partners&(1<<p) != 0 {
			cmds[p][1] = 1
		}
	}
	s := new(State)
	s.InitN(tbl, decks, n, 1, Config{NoElide: true, Commander: true, Commanders: cmds})
	for p := uint8(0); p < n; p++ {
		pl := &s.pl[p]
		for pl.handN > 0 {
			o := pl.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pl.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	return &scen{s: s, k: s.k()}, card
}

// commanderOut puts seat p's commander in slot j onto the battlefield and
// then into zone z, as a destruction or an exile would, so its ask is due.
func commanderOut(t *testing.T, sc *scen, p uint8, j int, z kop.Zone) Obj {
	t.Helper()
	s := sc.s
	o := s.cmdObj(p, j)
	if o == NoObj || s.objs[o].zone != kop.ZoneCommand {
		t.Fatalf("seat %d slot %d: commander %d not in the command zone", p, j, o)
	}
	s.detach(o)
	sl := s.enter(sc.k, o, p, false, kop.ZoneCommand)
	s.leave(sc.k, sl, z, false)
	if s.objs[o].zone != z {
		t.Fatalf("seat %d slot %d: commander in zone %v, want %v", p, j, s.objs[o].zone, z)
	}
	return o
}

// TestNSeatCommanderZoneAsk: at four seats every seat's commander starts in
// the command zone; commanders of seats 3 (its partner), 2 and 1 arrive in a
// graveyard or exile in that order and each owner is asked in arrival order
// (gorge's cmdZoneDue list), the answer moving the commander or leaving it.
func TestNSeatCommanderZoneAsk(t *testing.T) {
	sc, _ := newCommanderScenN(t, 4, 1<<3)
	s := sc.s
	for p := uint8(0); p < 4; p++ {
		if o := s.cmdObj(p, 0); o == NoObj || s.objs[o].zone != kop.ZoneCommand || s.objs[o].owner() != p {
			t.Fatalf("seat %d: commander %d not seated", p, o)
		}
		if got := s.pl[p].life; got != 40 {
			t.Fatalf("seat %d: life %d, want 40", p, got)
		}
	}
	if s.cmdObj(3, 1) == NoObj {
		t.Fatal("seat 3's partner not seated")
	}
	want := []struct {
		p    uint8
		j    int
		z    kop.Zone
		mode uint8
	}{{3, 1, kop.ZoneGraveyard, 0}, {2, 0, kop.ZoneExile, 1}, {1, 0, kop.ZoneGraveyard, 0}}
	var os []Obj
	for _, w := range want {
		os = append(os, commanderOut(t, sc, w.p, w.j, w.z))
	}
	for i, w := range want {
		d := s.Step()
		if d.Kind != DkChoose || d.Sub != ChCommanderZone || d.Player != w.p || d.N != 2 || d.Options[0].Source() != uint8(os[i]) {
			t.Fatalf("ask %d: kind %d sub %d player %d n %d src %d, want seat %d's commander %d", i, d.Kind, d.Sub, d.Player, d.N, d.Options[0].Source(), w.p, os[i])
		}
		s.Apply(d.Options[w.mode])
		wantZ := kop.ZoneCommand
		if w.mode != 0 {
			wantZ = w.z
		}
		if z := s.objs[os[i]].zone; z != wantZ {
			t.Fatalf("ask %d: mode %d left the commander in zone %v, want %v", i, w.mode, z, wantZ)
		}
	}
	if d := s.Step(); d.Kind == DkChoose && d.Sub == ChCommanderZone {
		t.Fatalf("a fourth ask, to seat %d", d.Player)
	}
}

// TestNSeatCommanderDamageLoss: commander damage is tallied per commander
// (CR 903.10a): 20 from each of two opponents' commanders is no loss, 21
// from one is, and the game goes on among the seats left (CR 800.4a). A
// seat that leaves with its commander on the battlefield takes it out of
// the game, and no command-zone ask follows for it.
func TestNSeatCommanderDamageLoss(t *testing.T) {
	sc, _ := newCommanderScenN(t, 4, 0)
	s := sc.s
	var sl [MaxSeats]Slot
	for p := uint8(1); p < 4; p++ {
		o := s.cmdObj(p, 0)
		s.detach(o)
		sl[p] = s.enter(sc.k, o, p, false, kop.ZoneCommand)
	}
	s.noteCmdDamage(sl[2], 0, 20)
	s.noteCmdDamage(sl[3], 0, 20)
	s.noteCmdDamage(sl[3], 1, 21)
	if got := [4]uint8{s.pl[0].cmdDmg[2], s.pl[0].cmdDmg[3], s.pl[1].cmdDmg[3], s.pl[1].cmdDmg[1]}; got != [4]uint8{20, 20, 21, 0} {
		t.Fatalf("tallies %v, want [20 20 21 0]", got)
	}
	s.Step()
	if s.pl[0].lost {
		t.Fatal("seat 0 lost to 20 damage from each of two commanders")
	}
	if !s.pl[1].lost || s.over {
		t.Fatalf("seat 1 lost %v, game over %v: want seat 1 out and the game going on", s.pl[1].lost, s.over)
	}
	o3 := s.cmdObj(3, 0)
	s.pl[3].life = 0
	d := s.Step()
	if !s.pl[3].lost || s.over {
		t.Fatalf("seat 3 lost %v, game over %v", s.pl[3].lost, s.over)
	}
	if z := s.objs[o3].zone; z != kop.ZoneNone {
		t.Fatalf("seat 3's commander in zone %v after it left, want none (ceased)", z)
	}
	if d.Kind == DkChoose && d.Sub == ChCommanderZone {
		t.Fatalf("a command-zone ask to seat %d after seat 3 left", d.Player)
	}
}
