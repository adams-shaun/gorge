package dirac_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// stageBase is a legal world: 20 cards a seat (10 Plains, 10 Savannah Lions),
// the first 7 of each seat's library order in hand, turn 1, seat 0's main phase.
func stageBase() dirac.World {
	w := dirac.World{Tbl: vanilla.Table, NSeat: 2, Turn: 1, Step: kop.StepMain1, Active: 0, Prio: 0, Life: [dirac.MaxSeats]int32{20, 20}}
	for p := uint8(0); p < 2; p++ {
		for i := 0; i < 20; i++ {
			c := vanilla.Plains
			if i%2 == 1 {
				c = vanilla.SavannahLions
			}
			o := dirac.WorldObj{Card: c, Owner: p, Ctrl: p, Zone: kop.ZoneLibrary, Pos: uint8(max(i-7, 0))}
			if i < 7 {
				o.Zone, o.Pos = kop.ZoneHand, uint8(i)
			}
			w.Objs = append(w.Objs, o)
		}
	}
	return w
}

// TestUnstageableReasons: the base world stages, and each way out of the
// minimal scope returns its own typed reason, every reason NewFromWorld has
// among them.
func TestUnstageableReasons(t *testing.T) {
	t.Parallel()
	if _, err := dirac.NewFromWorld(&[]dirac.World{stageBase()}[0]); err != nil {
		t.Fatalf("the base world does not stage: %v", err)
	}
	cases := map[string]func(*dirac.World){
		dirac.ReasonTable:   func(w *dirac.World) { w.Tbl = nil },
		dirac.ReasonSeats:   func(w *dirac.World) { w.NSeat = 5 },
		dirac.ReasonLayout:  func(w *dirac.World) { w.Objs[0], w.Objs[39] = w.Objs[39], w.Objs[0] },
		dirac.ReasonCard:    func(w *dirac.World) { w.Objs[3].Card = vanilla.LightningBolt },
		dirac.ReasonZone:    func(w *dirac.World) { w.Objs[3].Zone = kop.ZoneExile },
		dirac.ReasonControl: func(w *dirac.World) { w.Objs[3].Zone, w.Objs[3].Ctrl, w.Objs[3].Pos = kop.ZoneBattlefield, 1, 0 },
		dirac.ReasonPlace:   func(w *dirac.World) { w.Objs[3].Pos = 9 },
		dirac.ReasonStep:    func(w *dirac.World) { w.Step = kop.StepDamage },
		dirac.ReasonCombat: func(w *dirac.World) {
			w.Objs[3].Zone, w.Objs[3].Pos, w.Objs[3].Blocked = kop.ZoneBattlefield, 0, true // blocked and not attacking
		},
		dirac.ReasonPriority: func(w *dirac.World) { w.Prio = 2 },
		dirac.ReasonTurn:     func(w *dirac.World) { w.Turn = 0 },
		dirac.ReasonLife:     func(w *dirac.World) { w.Life[0] = 1 << 20 },
		dirac.ReasonToken:    func(w *dirac.World) { w.Objs[3].Token = true },
		dirac.ReasonAttach:   func(w *dirac.World) { w.Objs[3].Attached, w.Objs[3].Host = true, 4 },
		dirac.ReasonCommander: func(w *dirac.World) {
			// two objects in one command zone
			w.Commander, w.Cmd[0] = true, 3
			w.Objs[3].Zone, w.Objs[3].Pos, w.Objs[4].Zone, w.Objs[4].Pos = kop.ZoneCommand, 0, kop.ZoneCommand, 0
		},
		dirac.ReasonStackEntry: func(w *dirac.World) {
			w.Stack = []dirac.WorldStack{{Obj: 3, Ctrl: w.Objs[3].Owner}} // a spell whose card is in a hand
		},
		dirac.ReasonBattlefield: func(w *dirac.World) {
			w.Objs = nil
			for p := uint8(0); p < 2; p++ {
				for i := 0; i < dirac.MaxDeck; i++ {
					w.Objs = append(w.Objs, dirac.WorldObj{Card: vanilla.Plains, Owner: p, Ctrl: p, Zone: kop.ZoneBattlefield, Pos: uint8(i)})
				}
			}
		},
	}
	for _, r := range dirac.UnstageableReasons {
		mut, ok := cases[r]
		if !ok {
			t.Errorf("reason %q has no case", r)
			continue
		}
		w := stageBase()
		mut(&w)
		s, err := dirac.NewFromWorld(&w)
		var u dirac.ErrUnstageable
		if !errors.As(err, &u) || u.Reason != r || s != nil {
			t.Errorf("reason %q: got state %v, error %v", r, s != nil, err)
		}
	}
	for r := range cases {
		if !slices.Contains(dirac.UnstageableReasons, r) {
			t.Errorf("case %q is not in UnstageableReasons", r)
		}
	}
}

// TestStageStackPriorityRule: a non-empty stack may only be staged when its top
// entry belongs to the active seat, whoever holds priority. The other seat's
// top entry cannot tell a round that started at its controller (0 passes) from
// one that started at the active seat after a resolution (also 0 passes).
func TestStageStackPriorityRule(t *testing.T) {
	ab := dirac.SelfCounterAbility(&vanilla.Table.Cards[vanilla.HungryGhoul])
	if ab < 0 {
		t.Fatalf("Hungry Ghoul has no self-counter ability: %d", ab)
	}
	for _, tc := range []struct {
		name   string
		ctrl   uint8
		prio   uint8
		refuse bool
	}{
		{"seat-1-prio-0", 1, 0, true},
		{"seat-1-prio-1", 1, 1, true},
		{"seat-0-prio-0", 0, 0, false},
		{"seat-0-prio-1", 0, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := stageBase()
			ghouls := stageBattlefieldGhouls(&w)
			w.Active = 0
			w.Prio = tc.prio
			w.Stack = []dirac.WorldStack{{Ability: true, Obj: ghouls[tc.ctrl], Ctrl: tc.ctrl, Ab: uint8(ab)}}
			s, err := dirac.NewFromWorld(&w)
			var u dirac.ErrUnstageable
			if tc.refuse {
				if !errors.As(err, &u) || u.Reason != dirac.ReasonPriority || s != nil {
					t.Fatalf("got state %v, error %v; want %q", s != nil, err, dirac.ReasonPriority)
				}
				return
			}
			if err != nil || s == nil {
				t.Fatalf("did not stage: state %v, error %v", s != nil, err)
			}
		})
	}
}

// stageBattlefieldGhouls puts a Hungry Ghoul and a Savannah Lions on each
// seat's battlefield and returns each ghoul's object index. The two highest
// library cards become the permanents, leaving the library places dense.
func stageBattlefieldGhouls(w *dirac.World) [2]dirac.Obj {
	var ghouls [2]dirac.Obj
	for p := uint8(0); p < 2; p++ {
		base := int(p) * 20
		w.Objs[base+19] = dirac.WorldObj{Card: vanilla.HungryGhoul, Owner: p, Ctrl: p, Zone: kop.ZoneBattlefield, Pos: 0}
		w.Objs[base+18] = dirac.WorldObj{Card: vanilla.SavannahLions, Owner: p, Ctrl: p, Zone: kop.ZoneBattlefield, Pos: 1}
		ghouls[p] = dirac.Obj(base + 19)
	}
	return ghouls
}
