package dirac_test

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestStageEquipOnStack: a World on the mbc table with Swiftfoot Boots and two
// creatures on seat 0's battlefield, and the Boots' Equip activation on the
// stack targeting the second creature, stages; once both seats pass the Boots
// are attached to that creature. An entry whose target left the battlefield,
// or one the controller does not control, is refused.
func TestStageEquipOnStack(t *testing.T) {
	cardsettest.Require(t, "mbc")
	set, err := cardset.ByName("mbc")
	if err != nil {
		t.Fatal(err)
	}
	var list []cardset.Entry
	for _, n := range []string{"Swiftfoot Boots", "Fencing Ace", "Knight of the Tusk", "Plains"} {
		list = append(list, cardset.Entry{Name: n, Count: 1})
	}
	tbl, _, _, err := set.Matchup(list, list)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]kop.CardID{}
	for i := range tbl.Cards {
		if tbl.Cards[i].Types&kop.TToken == 0 {
			id[tbl.Cards[i].Name] = kop.CardID(i)
		}
	}
	boots := &tbl.Cards[id["Swiftfoot Boots"]]
	ab := dirac.EquipAbility(boots)
	if ab < 0 {
		t.Fatal("Swiftfoot Boots has no one Equip ability")
	}
	// seat 0's first three cards are the permanents (objects 0, 1, 2); the other
	// 17 cards of each seat are Plains: 7 in hand, the rest in the library
	build := func() dirac.World {
		w := dirac.World{Tbl: tbl, NSeat: 2, Turn: 1, Step: kop.StepMain1, Life: [dirac.MaxSeats]int32{20, 20}}
		for p := uint8(0); p < 2; p++ {
			for i := 0; i < 20; i++ {
				o := dirac.WorldObj{Card: id["Plains"], Owner: p, Ctrl: p, Zone: kop.ZoneLibrary, Pos: uint8(max(i-10, 0))}
				if i < 7 {
					o.Zone, o.Pos = kop.ZoneHand, uint8(i)
				}
				if p == 0 && i < 3 {
					o.Card = [3]kop.CardID{id["Swiftfoot Boots"], id["Fencing Ace"], id["Knight of the Tusk"]}[i]
					o.Zone, o.Pos = kop.ZoneBattlefield, uint8(i)
				}
				w.Objs = append(w.Objs, o)
			}
		}
		// the hand and library places run 0..n-1 per seat
		for p := uint8(0); p < 2; p++ {
			var n [2]uint8
			for i := range w.Objs {
				if o := &w.Objs[i]; o.Owner == p && o.Zone == kop.ZoneLibrary {
					o.Pos, n[0] = n[0], n[0]+1
				} else if ok := o.Owner == p && o.Zone == kop.ZoneHand; ok {
					o.Pos, n[1] = n[1], n[1]+1
				}
			}
		}
		w.Stack = []dirac.WorldStack{{Ability: true, Obj: 0, Ctrl: 0, Ab: uint8(ab), NT: 1, Tgt: [kop.MaxTargets]dirac.TCode{2}}}
		return w
	}
	w := build()
	s, err := dirac.NewFromWorld(&w)
	if err != nil {
		t.Fatalf("the Equip activation does not stage: %v", err)
	}
	var p dirac.Projection
	s.Project(&p)
	if p.NStack != 1 || p.Stack[0].Targets[0] != 2 {
		t.Fatalf("staged stack %d entries, targets %v", p.NStack, p.Stack[0].Targets)
	}
	if p.Objs[0].Attached == 2 {
		t.Fatal("the Boots are attached before the activation resolves")
	}
	// both seats pass; a pass with no other option is taken by the engine itself
	// (elision), so the second may never be asked
	for i := 0; i < 2; i++ {
		s.Project(&p)
		if p.NStack == 0 {
			break
		}
		if d := s.Step(); d.Kind != dirac.DkPriority {
			t.Fatalf("pass %d: decision %v, want priority", i, d.Kind)
		}
		s.Apply(dirac.MakeAction(dirac.AkPass, 0, 0, 0))
	}
	s.Step()
	s.Project(&p)
	if p.NStack != 0 || p.Objs[0].Attached != 2 {
		t.Fatalf("after both pass: %d stack entries, Boots attached to %d, want none and 2", p.NStack, p.Objs[0].Attached)
	}
	for name, mut := range map[string]func(*dirac.World){
		"the target left the battlefield":                         func(w *dirac.World) { w.Objs[2].Zone, w.Objs[2].Pos = kop.ZoneHand, 4 },
		"the target is not on a battlefield":                      func(w *dirac.World) { w.Stack[0].Tgt[0] = 20 },
		"two targets":                                             func(w *dirac.World) { w.Stack[0].NT = 2 },
		"an ability of a creature that is not a self-counter one": func(w *dirac.World) { w.Stack[0].Obj, w.Stack[0].Ab = 1, 0 },
	} {
		w := build()
		mut(&w)
		st, err := dirac.NewFromWorld(&w)
		var u dirac.ErrUnstageable
		if !errors.As(err, &u) || u.Reason != dirac.ReasonStackEntry || st != nil {
			t.Errorf("%s: state %v, error %v; want %q", name, st != nil, err, dirac.ReasonStackEntry)
		}
	}
}
