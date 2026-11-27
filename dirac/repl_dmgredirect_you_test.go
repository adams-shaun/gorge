package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// youRedirectState is a state with seat 0's redirect carrier (obj 5, slot 1:
// Vassal's Duty, Sivvi's Valor) the resolving entry and seat 0's creature
// (obj 4, slot 0) the ability's target (the remembered creature, the damaged
// host; the resolving controller, seat 0, is the recipient). The source is
// seat 1's red creature (card 1, slot 2). Both players start at 20 life so a
// landing is visible against the initial value. The table has the redirect
// gate open.
func youRedirectState() (*State, *prep) {
	s := &State{}
	s.g.nseat = MaxSeats
	s.pl[0].life, s.pl[1].life = 20, 20
	g := &genPrep{anyRedir: true, anyRepl: rbPrevCol, replBits: []uint32{0, 0, 0}}
	k := &prep{gen: g, t: &kop.Table{Cards: make([]kop.CardDef, 3)}, cards: make([]pcard, 3)}
	k.t.Cards[1].Colors = kop.ColR
	s.perms[0].card, s.perms[0].ctrl, s.perms[0].obj = 1, 0, 4
	s.objs[4].zone, s.objs[4].slot = kop.ZoneBattlefield, 0
	s.perms[1].card, s.perms[1].ctrl, s.perms[1].obj = 2, 0, 5
	s.objs[5].zone, s.objs[5].slot = kop.ZoneBattlefield, 1
	s.perms[2].card, s.perms[2].ctrl, s.perms[2].obj = 1, 1, 6
	s.objs[6].zone, s.objs[6].slot = kop.ZoneBattlefield, 2
	s.res.e.obj, s.res.e.ctrl, s.res.e.card, s.res.active = 5, 0, 2, true
	s.res.e.tgt[0] = TCode(4) // the ability's target: the remembered creature
	return s, k
}

// youRedirectAdd registers seat 0's redirect record of the given kind.
func youRedirectAdd(s *State, k *prep, kind uint8) {
	in := &kop.Instr{Op: kop.OpExt, A: kop.Const(int(kop.ExtDmgRedirect)), C: int16(kind)}
	in.Obj = kop.RefTarget0
	s.redirAdd(k, in)
}

// TestDmgRedirectYouDealsToController: the You kinds deal the redirected
// damage to the resolving controller -- Shock's 2 under DmgRedirectYouNext
// costs the controller 1 life, leaves 1 on the remembered host and ends the
// record; under DmgRedirectYouAll all 2 cost the controller their life, the
// host is dealt nothing and the record stays (it redirects all damage this
// turn).
func TestDmgRedirectYouDealsToController(t *testing.T) {
	for _, c := range []struct {
		name  string
		kind  uint8
		left  int // damage left to the remembered host
		lost  int // life the controller loses
		spent bool
	}{{"YouNext", kop.DmgRedirectYouNext, 1, 1, true}, {"YouAll", kop.DmgRedirectYouAll, 0, 2, false}} {
		s, k := youRedirectState()
		youRedirectAdd(s, k, c.kind)
		at, h := s.redirFind(TCode(4), 0)
		if at < 0 || h != 1 || s.g.redir[at].tgt != NoObj || s.g.redir[at].ctl != 0 {
			rec := redirRec{}
			if at >= 0 {
				rec = s.g.redir[at]
			}
			t.Fatalf("%s: the record does not match the remembered host: at %d hits %d, rec %+v", c.name, at, h, rec)
		}
		if _, h := s.redirFind(PlayerCode(0), 0); h != 0 {
			t.Fatalf("%s: the controller's own damage matches the record %d times, want 0 (no loop)", c.name, h)
		}
		if s.pl[0].life != 20 || s.perms[0].damage != 0 {
			t.Fatalf("%s: the preconditions do not hold: life %d, host damage %d, want 20 / 0", c.name, s.pl[0].life, s.perms[0].damage)
		}
		// 2 damage (a Shock) from seat 1's creature (slot 2) to the remembered
		// host obj 4: the record's share leaves for the controller
		n := s.replDamage(k, 1, 2, 1, TCode(4), false, 2)
		if n != c.left {
			t.Errorf("%s: damage left to the original recipient = %d, want %d", c.name, n, c.left)
		}
		if s.pl[0].life != int16(20-c.lost) {
			t.Errorf("%s: the controller is at %d life, want %d", c.name, s.pl[0].life, 20-c.lost)
		}
		if s.pl[1].life != 20 {
			t.Errorf("%s: the source's controller is at %d life, want 20 (the redirect goes to the resolving controller)", c.name, s.pl[1].life)
		}
		if c.spent {
			if s.g.redir[0].kind != 0 {
				t.Errorf("%s: the spent record is still live: %+v", c.name, s.g.redir[0])
			}
			// the held event's rest lands as its caller would deal it
			k0 := &prep{gen: &genPrep{}, t: k.t, cards: k.cards}
			s.damageTo(k0, TCode(4), n)
			if s.perms[0].damage != 1 {
				t.Errorf("%s: the host was dealt %d, want 1", c.name, s.perms[0].damage)
			}
			if n := s.replDamage(k, 1, 2, 1, TCode(4), false, 2); n != 2 {
				t.Errorf("%s: a second event after the pool's spend left %d, want 2", c.name, n)
			}
			if s.pl[0].life != int16(20-c.lost) {
				t.Errorf("%s: the controller is at %d life after the pool's spend, want %d", c.name, s.pl[0].life, 20-c.lost)
			}
		} else {
			if s.g.redir[0].kind != c.kind {
				t.Errorf("%s: the record ended: %+v, want kind %d", c.name, s.g.redir[0], c.kind)
			}
			if s.perms[0].damage != 0 {
				t.Errorf("%s: the host was dealt %d, want 0 (all of it left)", c.name, s.perms[0].damage)
			}
		}
	}
}

// TestDmgRedirectYouDrops: the record ends when the remembered host leaves
// (the Effect's ExileOnMoved$/ForgetOnMoved$ Battlefield) and matches nothing
// once its controller has lost (gorge leaves the event standing).
func TestDmgRedirectYouDrops(t *testing.T) {
	for _, kind := range []uint8{kop.DmgRedirectYouNext, kop.DmgRedirectYouAll} {
		s, k := youRedirectState()
		youRedirectAdd(s, k, kind)
		if _, h := s.redirFind(TCode(4), 0); h != 1 {
			t.Fatalf("kind %d: the record does not match the remembered host", kind)
		}
		s.redirDrop(4) // the remembered creature leaves the battlefield
		if _, h := s.redirFind(TCode(4), 0); h != 0 {
			t.Errorf("kind %d: the host leaving left %d live records, want 0", kind, h)
		}

		s, k = youRedirectState()
		youRedirectAdd(s, k, kind)
		s.pl[0].lost = true
		if _, h := s.redirFind(TCode(4), 0); h != 0 {
			t.Errorf("kind %d: the lost controller matched %d records, want 0", kind, h)
		}
		if n := s.replDamage(k, 1, 2, 1, TCode(4), false, 2); n != 2 {
			t.Errorf("kind %d: damage to the host of a lost controller left %d, want 2 (the event stands)", kind, n)
		}
		if s.pl[0].life != 20 {
			t.Errorf("kind %d: the lost controller is at %d life, want 20", kind, s.pl[0].life)
		}
	}
}

// TestDmgRedirectYouCombatStalls: a combat event to the remembered host
// stalls StallRedirOrder -- a player recipient's new event has no dmgEv field
// (only noncombat damage is modelled for the You kinds).
func TestDmgRedirectYouCombatStalls(t *testing.T) {
	for _, kind := range []uint8{kop.DmgRedirectYouNext, kop.DmgRedirectYouAll} {
		s, k := youRedirectState()
		youRedirectAdd(s, k, kind)
		var r dmgRec
		ev := dmgEv{src: 2, dst: 0, n: 3}
		if at := s.redirMatch(k, 1, 2, 1, TCode(4), 0, 0, true); at < 0 {
			t.Fatalf("kind %d: redirMatch found no record (at %d): the stall would prove nothing", kind, at)
		}
		if s.g.stalled != 0 {
			t.Fatalf("kind %d: redirMatch stalled %d before the combat arm ran", kind, s.g.stalled)
		}
		if s.redirCombat(k, &r, &ev, false, TCode(4), 0) {
			t.Errorf("kind %d: the combat event lost damage, want the stall", kind)
		}
		if s.g.stalled != StallRedirOrder {
			t.Errorf("kind %d: stalled %d, want %d", kind, s.g.stalled, StallRedirOrder)
		}
		if ev.n != 3 {
			t.Errorf("kind %d: the event left with %d damage, want 3", kind, ev.n)
		}
	}
}

// TestDmgRedirectYouMarksSource: a permanent source whose noncombat damage a
// You kind redirects to the controller is still marked as having dealt
// damage this turn (damageTo's markDealt; gorge emits the Damage event with
// that source). Under YouAll the original event's n drops to 0, so nothing
// else marks it.
func TestDmgRedirectYouMarksSource(t *testing.T) {
	for _, kind := range []uint8{kop.DmgRedirectYouNext, kop.DmgRedirectYouAll} {
		s, k := youRedirectState()
		k.gen.anyPermMark = true
		k.gen.ix[mechDealtTurn], k.gen.ix[mechDealtAny] = 0, 1
		s.cold.ms = make([]slotMask, 2)
		s.occ.set(2)
		youRedirectAdd(s, k, kind)
		if s.mSl(k, mechDealtTurn, 2) || s.mSl(k, mechDealtAny, 2) {
			t.Fatalf("kind %d: the source is marked before the damage", kind)
		}
		// a creature source (slot 2, seat 1) deals 2 to the remembered host
		s.replDamage(k, 1, 2, 1, TCode(4), false, 2)
		if s.pl[0].life == 20 {
			t.Fatalf("kind %d: the damage was not redirected to the controller", kind)
		}
		if !s.mSl(k, mechDealtTurn, 2) || !s.mSl(k, mechDealtAny, 2) {
			t.Errorf("kind %d: the redirecting source is not marked as having dealt damage", kind)
		}
	}
}
