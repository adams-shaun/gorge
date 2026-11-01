package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSpeedControlChangeStartsEngines is CR 702.179a's control-change half:
// a seat with no speed that takes a Start your engines! permanent starts at
// 1. It is the case a battlefield-entry-only scan cannot see, so the oracle
// lockstep's mutation test cannot use it: Involuntary Employment's Treasure
// enters after the gain, and that entry re-runs the global scan (gorge
// checkSpeedStart scans every seat), masking the removed hook.
func TestSpeedControlChangeStartsEngines(t *testing.T) {
	set := cardsettest.Set(t, "speed")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	pi, ok := set.Lookup("Pride of the Road")
	if !ok {
		t.Fatal("Pride of the Road not in the speed pool")
	}
	pride := set.PoolID(pi)
	li, ok := set.Lookup("Plains")
	if !ok {
		t.Fatal("Plains not in the speed pool")
	}
	land := set.PoolID(li)
	deck := make([]CardID, 40)
	for i := range deck {
		deck[i] = land
	}
	s := NewConfig(tbl, deck, deck, 1, Config{})
	k := s.k()

	add := func(ctrl uint8) Slot {
		o := Obj(s.nobj)
		s.nobj++
		s.objs[o] = object{card: pride, chosen: 255, zone: kop.ZoneHand, slot: NoSlot}
		s.objs[o].setOwner(ctrl)
		return s.enter(k, o, ctrl, false, kop.ZoneNone)
	}

	sl := add(0)
	if got := s.Speed(0); got != 1 {
		t.Fatalf("entry under seat 0: speed %d, want 1", got)
	}
	s.g.speed = 0
	s.ctrlMove(sl, 1)
	if got := s.Speed(1); got != 1 {
		t.Fatalf("control change to seat 1: speed %d, want 1", got)
	}

	s.g.speed = 0
	DebugIgnoreControlStart = true
	s.ctrlMove(sl, 0)
	s.ctrlMove(sl, 1)
	DebugIgnoreControlStart = false
	if got := s.Speed(1); got != 0 {
		t.Fatalf("control change with the hook off: speed %d, want 0", got)
	}

	// A later entry re-runs the scan over every seat, so the hook's effect is
	// redundant when something else enters afterwards (Involuntary
	// Employment's Treasure).
	add(0)
	if got := s.Speed(1); got != 1 {
		t.Fatalf("later entry with the hook off: speed %d, want 1", got)
	}
}
