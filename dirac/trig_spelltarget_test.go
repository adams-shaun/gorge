package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestRefTriggerSpellTargetReadsTheSpellOnTheStack: the ref names the cast
// spell's single distinct object target while the spell is on the stack
// (gorge reads the targets off the spell object), and nobody once the spell
// left it (a counterspell cast in response to the trigger: gorge clears the
// targets), or when the spell's target is a player, several objects or none.
func TestRefTriggerSpellTargetReadsTheSpellOnTheStack(t *testing.T) {
	sc := newScen(t, Config{})
	s := sc.s
	victim := sc.perm(v.TyphoidRats, 1)
	other := sc.perm(v.TyphoidRats, 1)
	if victim == other {
		t.Fatal("precondition: two distinct permanents")
	}
	cases := []struct {
		name string
		tgt  []TCode
		want Obj
	}{
		{"one object target", []TCode{TCode(victim)}, victim},
		{"the same object twice", []TCode{TCode(victim), TCode(victim)}, victim},
		{"two distinct targets", []TCode{TCode(victim), TCode(other)}, NoObj},
		{"a player target", []TCode{TCode(PlayerTarget0)}, NoObj},
		{"no target", nil, NoObj},
	}
	for _, tc := range cases {
		spell := sc.obj(v.TyphoidRats, 0)
		s.objs[spell].zone = kop.ZoneStack
		e := stackEntry{kind: seSpell, obj: spell, ctrl: 0, trigObj: NoObj}
		for i, c := range tc.tgt {
			e.tgt[i] = c
		}
		e.nt = uint8(len(tc.tgt))
		s.sp = 0
		s.push(e)
		s.res.e = stackEntry{kind: seTrigger, obj: victim, trigObj: spell}
		if got := s.objRefObj(kop.RefTriggerSpellTarget); got != tc.want {
			t.Errorf("%s: RefTriggerSpellTarget = %d, want %d", tc.name, got, tc.want)
		}
		if tc.want != NoObj {
			s.sp = 0 // the spell was countered: its entry is gone
			s.objs[spell].zone = kop.ZoneGraveyard
			if got := s.objRefObj(kop.RefTriggerSpellTarget); got != NoObj {
				t.Errorf("%s: countered spell: RefTriggerSpellTarget = %d, want none", tc.name, got)
			}
		}
	}
}

// TestRefTriggerSpellTargetFollowsTheBattlefield: the named object yields a
// slot only while it is on the battlefield (a creature that left before the
// trigger resolved is not gained).
func TestRefTriggerSpellTargetFollowsTheBattlefield(t *testing.T) {
	sc := newScen(t, Config{})
	s, k := sc.s, sc.k
	victim := sc.perm(v.TyphoidRats, 1)
	spell := sc.obj(v.TyphoidRats, 0)
	s.objs[spell].zone = kop.ZoneStack
	e := stackEntry{kind: seSpell, obj: spell, trigObj: NoObj}
	e.tgt[0], e.nt = TCode(victim), 1
	s.push(e)
	s.res.e = stackEntry{kind: seTrigger, obj: victim, trigObj: spell}
	if sl := s.objRef(k, kop.RefTriggerSpellTarget); sl != sc.slot(victim) {
		t.Fatalf("on the battlefield: slot %d, want %d", sl, sc.slot(victim))
	}
	s.objs[victim].zone = kop.ZoneGraveyard
	if sl := s.objRef(k, kop.RefTriggerSpellTarget); sl != NoSlot {
		t.Errorf("in the graveyard: slot %d, want none", sl)
	}
}
