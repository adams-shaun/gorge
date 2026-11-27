package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// hostRedirectState is a state with seat 1's permanent (obj 5, slot 1) the
// resolving entry and seat 0's creature (obj 4, slot 0) the ability's target
// (Daughter of Autumn: the remembered creature is the damaged host, the
// resolving permanent the recipient). The source is seat 0's other creature
// (card 1, slot 2). The table has the redirect gate open.
func hostRedirectState() (*State, *prep) {
	s := &State{}
	s.g.nseat = MaxSeats
	g := &genPrep{anyRedir: true, anyRepl: rbPrevCol, replBits: []uint32{0, 0, 0}}
	k := &prep{gen: g, t: &kop.Table{Cards: make([]kop.CardDef, 3)}, cards: make([]pcard, 3)}
	k.t.Cards[1].Colors = kop.ColR
	s.perms[0].card, s.perms[0].ctrl, s.perms[0].obj = 1, 0, 4
	s.objs[4].zone, s.objs[4].slot = kop.ZoneBattlefield, 0
	s.perms[1].card, s.perms[1].ctrl, s.perms[1].obj = 2, 1, 5
	s.objs[5].zone, s.objs[5].slot = kop.ZoneBattlefield, 1
	s.perms[2].card, s.perms[2].ctrl, s.perms[2].obj = 1, 0, 6
	s.objs[6].zone, s.objs[6].slot = kop.ZoneBattlefield, 2
	s.res.e.obj, s.res.e.ctrl, s.res.e.card, s.res.active = 5, 1, 2, true
	s.res.e.tgt[0] = TCode(4) // the ability's target: the damaged host
	return s, k
}

// hostRedirectAdd registers seat 1's DmgRedirectHost record.
func hostRedirectAdd(s *State, k *prep) {
	in := &kop.Instr{Op: kop.OpExt, A: kop.Const(int(kop.ExtDmgRedirect)), C: kop.DmgRedirectHost}
	in.W = kop.RefTarget0
	s.redirAdd(k, in)
}

// TestDmgRedirectHostDealsToHost: DmgRedirectHost is DmgRedirectNext with the
// pair swapped -- the next 1 damage that would be dealt to the ability's
// target is dealt to the resolving host permanent instead, as a new event,
// the held event keeps the rest, and the pool's spend ends the record.
func TestDmgRedirectHostDealsToHost(t *testing.T) {
	s, k := hostRedirectState()
	hostRedirectAdd(s, k)
	if at, h := s.redirFind(TCode(4), 0); h != 1 || s.g.redir[at].tgt != 5 {
		t.Fatalf("the record does not match the damaged host: at %d hits %d, want 0 / 1 to obj 5", at, h)
	}
	if _, h := s.redirFind(TCode(5), 1); h != 0 {
		t.Fatalf("the recipient's own damage matches the record %d times, want 0 (no loop)", h)
	}
	// 3 damage from seat 0's creature (slot 2) to the remembered host obj 4:
	// 1 is redirected to the resolving permanent obj 5, 2 stay
	if n := s.replDamage(k, 0, 2, 1, TCode(4), false, 3); n != 2 {
		t.Errorf("damage left to the original recipient = %d, want 2", n)
	}
	if s.perms[1].damage != 1 {
		t.Errorf("the host permanent took %d, want 1", s.perms[1].damage)
	}
	if s.g.redir[0].kind != 0 {
		t.Errorf("the spent record is still live: %+v", s.g.redir[0])
	}
	if n := s.replDamage(k, 0, 2, 1, TCode(4), false, 3); n != 3 {
		t.Errorf("a second event after the pool's spend left %d, want 3", n)
	}
	if s.perms[1].damage != 1 {
		t.Errorf("the host permanent took %d after the spend, want 1", s.perms[1].damage)
	}
}

// TestDmgRedirectHostDrops: the record ends when either named permanent
// leaves: the recipient (Duration$ UntilHostLeavesPlayOrEOT) and the damaged
// host (the remembered creature's line can no longer match).
func TestDmgRedirectHostDrops(t *testing.T) {
	for _, o := range []Obj{4, 5} {
		s, k := hostRedirectState()
		hostRedirectAdd(s, k)
		if _, h := s.redirFind(TCode(4), 0); h != 1 {
			t.Fatalf("obj %d: the record does not match the host", o)
		}
		s.redirDrop(o)
		if _, h := s.redirFind(TCode(4), 0); h != 0 {
			t.Errorf("obj %d leaving left %d live records, want 0", o, h)
		}
	}
}

// TestDmgRedirectHostRegistration: the registration needs both permanents on
// the battlefield: a host or recipient that is gone registers nothing (gorge
// has no live referent for the line, or the host left under Duration$
// UntilHostLeavesPlayOrEOT).
func TestDmgRedirectHostRegistration(t *testing.T) {
	for _, c := range []struct {
		name string
		gone Obj
		zone kop.Zone
		want int
	}{{"recipient gone", 5, kop.ZoneGraveyard, 0}, {"host gone", 4, kop.ZoneExile, 0}, {"live", 0, 0, 1}} {
		s, k := hostRedirectState()
		if c.gone != 0 {
			s.objs[c.gone].zone = c.zone
		}
		hostRedirectAdd(s, k)
		// the damaged host's own event: only the live registration matches it
		if _, h := s.redirFind(TCode(4), 0); h != c.want {
			t.Errorf("%s: live records %d, want %d", c.name, h, c.want)
		}
	}
}
