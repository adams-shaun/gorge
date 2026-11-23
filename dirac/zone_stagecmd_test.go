package dirac_test

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// cmdPodBase is a four-seat Commander world at seat 0's main phase of turn 1,
// 40 life, 20 cards a seat: card 0 of each seat is its commander (Legendary
// Bear), in zones[p], the rest Plains, seven to a hand (the commander counting
// when it is there) and the others the library from the top.
func cmdPodBase(zones [4]kop.Zone) dirac.World {
	w := dirac.World{Tbl: vanilla.Table, NSeat: 4, Turn: 1, Step: kop.StepMain1, Commander: true}
	for p := uint8(0); p < 4; p++ {
		w.Life[p] = 40
		w.Cmd[p] = dirac.Obj(20 * int(p))
		var n = map[kop.Zone]uint8{}
		place := func(c dirac.CardID, z kop.Zone) {
			w.Objs = append(w.Objs, dirac.WorldObj{Card: c, Owner: p, Ctrl: p, Zone: z, Pos: n[z]})
			n[z]++
		}
		if zones[p] != kop.ZoneLibrary {
			place(vanilla.LegendaryBear, zones[p])
		} else {
			place(vanilla.LegendaryBear, kop.ZoneLibrary) // the commander on top, order kept for the bottom case
		}
		for i := 1; i < 20; i++ {
			if n[kop.ZoneHand] < 7 {
				place(vanilla.Plains, kop.ZoneHand)
			} else {
				place(vanilla.Plains, kop.ZoneLibrary)
			}
		}
	}
	return w
}

// TestStageCommanderPosition: a commander stages in the command zone, the
// graveyard, the battlefield, the hand and the library, with its cast count
// and the commander damage each player has taken, and its game is a
// Commander game at the staged life.
func TestStageCommanderPosition(t *testing.T) {
	t.Parallel()
	zones := [4]kop.Zone{kop.ZoneCommand, kop.ZoneGraveyard, kop.ZoneBattlefield, kop.ZoneHand}
	w := cmdPodBase(zones)
	w.Objs[40].Sick = true
	w.CmdCasts = [dirac.MaxSeats]uint8{2, 1, 0, 3}
	w.CmdDamage[1][0], w.CmdDamage[3][2], w.CmdDamage[0][3] = 7, 5, 21
	s, err := dirac.NewFromWorld(&w)
	if err != nil {
		t.Fatal(err)
	}
	var p dirac.Projection
	s.Project(&p)
	if !s.FeatCommanderGame() {
		t.Error("the staged game is not a Commander game")
	}
	if p.CmdCast != [dirac.MaxSeats]uint8{2, 1, 0, 3} {
		t.Errorf("cast counts %v", p.CmdCast)
	}
	if p.CmdDmg[1][0] != 7 || p.CmdDmg[3][2] != 5 || p.CmdDmg[0][3] != 21 || p.CmdDmg[2][1] != 0 {
		t.Errorf("commander damage %v", p.CmdDmg)
	}
	for q, z := range zones {
		if got := p.Objs[20*q].Zone; got != z {
			t.Errorf("seat %d commander in zone %d, want %d", q, got, z)
		}
	}
	if p.Life[0] != 40 {
		t.Errorf("life %d", p.Life[0])
	}
	// the graveyard commander was offered the command zone and declined: no ask is pending
	var d dirac.Decision
	s.StepInto(&d)
	if d.Kind != dirac.DkPriority {
		t.Errorf("the staged position poses %v, want a priority decision", d.Kind)
	}
	w = cmdPodBase([4]kop.Zone{kop.ZoneLibrary, kop.ZoneCommand, kop.ZoneCommand, kop.ZoneCommand})
	if _, err := dirac.NewFromWorld(&w); err != nil {
		t.Errorf("a commander on its library's top does not stage: %v", err)
	}
}

// TestStageCommanderRefusals: each Commander field the staging cannot
// rebuild returns the commander reason.
func TestStageCommanderRefusals(t *testing.T) {
	t.Parallel()
	w := cmdPodBase([4]kop.Zone{kop.ZoneHand, kop.ZoneHand, kop.ZoneHand, kop.ZoneHand})
	if _, err := dirac.NewFromWorld(&w); err != nil {
		t.Fatalf("the base world does not stage: %v", err)
	}
	cases := map[string]func(*dirac.World){
		"two cards in a command zone": func(w *dirac.World) {
			w.Objs[0].Zone, w.Objs[0].Pos, w.Objs[1].Zone, w.Objs[1].Pos = kop.ZoneCommand, 0, kop.ZoneCommand, 0
		},
		"a command zone card with Commander off": func(w *dirac.World) {
			w.Commander, w.Cmd = false, [dirac.MaxSeats]dirac.Obj{}
			w.Objs[1].Zone, w.Objs[1].Pos = kop.ZoneCommand, 0
		},
		"a commander the seat does not own":  func(w *dirac.World) { w.Cmd[1] = 0 },
		"a commander that is not legendary":  func(w *dirac.World) { w.Cmd[0] = 1 },
		"commander damage from no commander": func(w *dirac.World) { w.Cmd[2], w.CmdDamage[0][2] = dirac.NoObj, 1 },
		"a cast count with Commander off": func(w *dirac.World) {
			w.Commander, w.Cmd, w.CmdCasts[1] = false, [dirac.MaxSeats]dirac.Obj{}, 1
		},
	}
	for name, mut := range cases {
		w := cmdPodBase([4]kop.Zone{kop.ZoneHand, kop.ZoneHand, kop.ZoneHand, kop.ZoneHand})
		mut(&w)
		s, err := dirac.NewFromWorld(&w)
		var u dirac.ErrUnstageable
		if !errors.As(err, &u) || u.Reason != dirac.ReasonCommander || s != nil {
			t.Errorf("%s: got state %v, error %v", name, s != nil, err)
		}
	}
	// a legendary non-creature is not staged
	w = cmdPodBase([4]kop.Zone{kop.ZoneHand, kop.ZoneHand, kop.ZoneHand, kop.ZoneHand})
	w.Objs[0].Card = vanilla.EmberWalker
	if _, err := dirac.NewFromWorld(&w); !errors.As(err, new(dirac.ErrUnstageable)) {
		t.Errorf("a legendary planeswalker commander staged: %v", err)
	}
}
