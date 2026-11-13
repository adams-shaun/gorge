package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// charmMinCountTable is vanilla's Test Charm lowered as a charm with no
// MinCharmNum$ over a two-arm count (the optional-cost shape): the offer
// gate's minimum is the smaller arm 1 (ModesPick 2, ModesFew 1) and the mode
// step's minimum is the evaluated count, here the constant V.
func charmMinCountTable(count int) *kop.Table {
	t := charmUpToTable(count)
	a := t.Cards[v.TestCharm].Abilities[0]
	a.ModesPick, a.ModesFew = 2, 1
	t.Cards[v.TestCharm].Abilities = []kop.Ability{a}
	t.Cards[v.TestCharm].ModesMinCount = true
	return t
}

// TestCharmMinCountFollowsTheCount: with only the player mode targetable
// (no creature in play) the plain cast (count 1) is offered and asks for the
// one mode, while the count-2 cast needs both modes and is reversed at the
// mode step (gorge's minimum is the evaluated count).
func TestCharmMinCountFollowsTheCount(t *testing.T) {
	sc := newScenTable(t, charmMinCountTable(1), Config{NoElide: true})
	sc.lands(0, v.Mountain, 1)
	fc := sc.hand(v.TestCharm, 0)
	sc.do(t, AkCast, TCode(fc), scAny) // offered: one feasible mode meets the gate's minimum 1
	d := sc.s.Step()
	if d.Kind != DkModes {
		t.Fatalf("count 1: want the mode ask, got kind %d", d.Kind)
	}
	if m, done := modeOpts(&d); m != 2 || done {
		t.Fatalf("count 1 first pose %+v: want only the player mode and no Done", d.Options[:d.N])
	}

	sc = newScenTable(t, charmMinCountTable(2), Config{NoElide: true})
	sc.lands(0, v.Mountain, 1)
	fc = sc.hand(v.TestCharm, 0)
	sc.do(t, AkCast, TCode(fc), scAny)
	if d := sc.s.Step(); d.Kind == DkModes {
		t.Fatalf("count 2 with one feasible mode: a mode ask was posed: %+v", d.Options[:d.N])
	}
	if sc.s.objs[fc].zone != kop.ZoneHand {
		t.Fatalf("count 2 with one feasible mode: the spell is in zone %d, want the cast reversed to the hand", sc.s.objs[fc].zone)
	}
}
