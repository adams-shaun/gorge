package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPlayCrossHandTriggerPlayerShapes pins the two Play spellings over the
// trigger's damaged player's hand: Mindleech Mass's Valid$ Card.ControlledBy
// TriggeredTarget and Silent-Blade Oni's Valid$ Card.nonLand+OwnedBy
// TriggeredTarget, both ValidZone$ Hand. Each lowers to a CastFree over
// RefTriggerPlayer's hand with no target slot; the Oni keeps its nonLand
// filter, the control predicate dropped because the ref picks the seat.
func TestPlayCrossHandTriggerPlayerShapes(t *testing.T) {
	names := []string{"Mindleech Mass", "Silent-Blade Oni"}
	all, bs, _, err := buildPool(lowerNamedPool(t, names))
	if err != nil {
		t.Fatal(err)
	}
	wantFilter := map[string]bool{"Mindleech Mass": false, "Silent-Blade Oni": true}
	seen := map[string]bool{}
	for i, c := range all {
		want, ok := wantFilter[c.Name]
		if !ok {
			continue
		}
		seen[c.Name] = true
		for _, r := range lowerRefusals(c) {
			t.Errorf("%s refused: %v", c.Name, r)
		}
		var cast *kop.Instr
		for j := range bs[i].def.Abilities {
			prog := bs[i].def.Abilities[j].Prog
			for k := range prog {
				if prog[k].Op == kop.OpCastFree {
					cast = &prog[k]
				}
			}
		}
		if cast == nil {
			t.Errorf("%s built no CastFree", c.Name)
			continue
		}
		if cast.Obj != kop.RefTriggerPlayer || int(cast.C&7) != int(kop.ZoneHand) {
			t.Errorf("%s CastFree Obj=%v zone=%d, want RefTriggerPlayer over the hand", c.Name, cast.Obj, cast.C&7)
		}
		if got := cast.F != 0; got != want {
			t.Errorf("%s CastFree filter set = %v (F=%d), want %v", c.Name, got, cast.F, want)
		}
	}
	for _, n := range names {
		if !seen[n] {
			t.Errorf("%s not in the lowered pool", n)
		}
	}
}
