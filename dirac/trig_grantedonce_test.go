package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestOncePerTurnGrantedLineLockstep: a granted once-each-turn line and the
// host's own printed once-each-turn line at the same ability index of their
// own cards each hold their own turn. The used record is keyed by the line's
// index in its card; a granted line's index is its granter's, so before the
// record carried the granting permanent too, queueing one spent the other's
// turn (gorge keys a limit by {Source, Idx, Grantor}).
func TestOncePerTurnGrantedLineLockstep(t *testing.T) {
	gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
	tapOnce := kop.Trigger{Event: kop.TrigTaps, Filter: 0, Flags: kop.TfOncePerTurn}
	tbl := &kop.Table{
		Filters: []kop.Filter{{Types: kop.TCreature, Owner: kop.CtrlYou}, {Types: kop.TCreature, Flags: kop.FAttachedBySrc}},
		Exprs:   []kop.Expr{{Kind: kop.ExConst, N: 1}},
		Cards: []kop.CardDef{
			{Name: "Test Plains", Types: kop.TLand},
			// ability 0 is the host's own once-each-turn line
			{Name: "Test Own Once Bear", Types: kop.TCreature, Power: 2, Toughness: 2,
				Abilities: []kop.Ability{{Kind: kop.AbTriggered, Trig: tapOnce, Prog: gain}}},
			// ability 0 is the line it grants (the same index), ability 1 the grant
			{Name: "Test Granting Banner", Types: kop.TArtifact, Abilities: []kop.Ability{
				{Kind: kop.AbTriggered, AFlags: kop.AfLatent, Trig: tapOnce, Prog: gain},
				{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Filter: 1, Grant: 1}}}},
		},
	}
	sc := newScenTableLib(t, tbl, 0, Config{NoElide: true})
	s, k := sc.s, sc.k
	bear := sc.perm(1, 0)
	banner := sc.perm(2, 0)
	s.perms[s.objs[banner].slot].attached = s.objs[bear].slot

	tap := func() {
		t.Helper()
		s.res.e.trigObj = bear
		if !s.exec(k, &kop.Instr{Op: kop.OpTap, Obj: kop.RefTriggerObj}) {
			t.Fatal("the tap suspended")
		}
	}
	untap := func() {
		t.Helper()
		s.untapEffect(k, s.objs[bear].slot)
		if s.perms[s.objs[bear].slot].flags&pfTapped != 0 {
			t.Fatal("precondition: the permanent did not untap")
		}
	}

	s.npend = 0
	tap()
	if n := s.npend; n != 2 {
		t.Fatalf("first tap: %d triggers queued, want 2 (the own line and the granted line at the same index)", n)
	}
	var own, granted int
	for i := 0; i < int(s.npend); i++ {
		if s.pend[i].card == 1 {
			own++
		}
		if s.pend[i].card == 2 {
			granted++
		}
	}
	if own != 1 || granted != 1 {
		t.Fatalf("first tap queued own %d granted %d, want 1 each", own, granted)
	}

	// a dropped queueing (CR 733.1) gives each line's own turn back
	s.dropPendingLimits(k, 0)
	if s.npend != 0 || s.g.nused != 0 {
		t.Fatalf("after the drop: %d pending, %d used records, want none", s.npend, s.g.nused)
	}
	untap()
	tap()
	if n := s.npend; n != 2 {
		t.Fatalf("tap after the drop: %d triggers queued, want 2", n)
	}

	// each line has used its turn
	untap()
	s.npend = 0
	tap()
	if n := s.npend; n != 0 {
		t.Fatalf("second tap this turn: %d triggers queued, want 0", n)
	}
}
