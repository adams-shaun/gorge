package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// redirOrderState is a state with seat 1's Gideon's Sacrifice record (all
// damage to seat 1 and its permanents dealt to object 5, its permanent at
// slot 1 instead) and seat 0's red creature at slot 0 (card 1) attacking seat
// 1. scope adds seat 1's PrevScopePlayer record, col a PreventColor record
// for red. The table has the redirect and prevention gates open.
func redirOrderState(scope, col bool) (*State, *prep) {
	s := &State{}
	s.g.nseat = MaxSeats
	g := &genPrep{anyRedir: true, anyRepl: rbPrevCol, replBits: []uint32{0, 0, 0}}
	k := &prep{gen: g, t: &kop.Table{Cards: make([]kop.CardDef, 3)}}
	k.t.Cards[1].Colors = kop.ColR
	s.perms[0].card, s.perms[0].ctrl, s.perms[0].obj, s.perms[0].atk = 1, 0, 4, PlayerTarget0+1
	s.objs[4].zone, s.objs[4].slot = kop.ZoneBattlefield, 0
	s.perms[1].card, s.perms[1].ctrl, s.perms[1].obj = 2, 1, 5
	s.objs[5].zone, s.objs[5].slot = kop.ZoneBattlefield, 1
	s.g.redir[0] = redirRec{tgt: 5, ctl: 1, kind: kop.DmgRedirectScope}
	if scope {
		s.g.prevCol[0].sc, s.g.prevCol[0].ctl = kop.PrevScopePlayer, 1
	}
	if col {
		s.g.prevCol[1].col = kop.ColR
	}
	return s, k
}

// TestDmgRedirectBesidePrevention: a redirect record and a prevention record
// (a scope record, a colour record) both matching one damage event is gorge's
// CR 616.1 order ask, so the kernel stalls StallRedirOrder rather than let
// the prevention answer first; the direct damage sites and the combat pass.
func TestDmgRedirectBesidePrevention(t *testing.T) {
	for _, c := range []struct {
		name       string
		scope, col bool
	}{{"scope", true, false}, {"colour", false, true}} {
		s, k := redirOrderState(c.scope, c.col)
		if !s.prevColHit(k, NoSlot, 1, prevTgt{1, NoSlot}, false) {
			t.Fatalf("%s: the prevention record does not match the event", c.name)
		}
		if _, h := s.redirFind(PlayerTarget0+1, 1); h != 1 {
			t.Fatalf("%s: the redirect record matches %d times, want 1", c.name, h)
		}
		if n := s.replDamage(k, 0, NoSlot, 1, PlayerTarget0+1, false, 3); n != 0 || s.g.stalled != StallRedirOrder {
			t.Errorf("%s noncombat: damage %d stalled %d, want 0 / %d", c.name, n, s.g.stalled, StallRedirOrder)
		}

		s, k = redirOrderState(c.scope, c.col)
		var r dmgRec
		r.dealt[0], r.dealtPl[0], r.toPl[1] = 3, 3, 3
		r.note(0, NoSlot, 3)
		s.replCombatGen(k, &r)
		if s.g.stalled != StallRedirOrder {
			t.Errorf("%s combat: stalled %d (event left %d), want %d", c.name, s.g.stalled, r.ev[0].n, StallRedirOrder)
		}
	}
	// a lone prevention record (no redirect) still prevents unstalled
	s, k := redirOrderState(true, false)
	s.g.redir[0] = redirRec{}
	if n := s.replDamage(k, 0, NoSlot, 1, PlayerTarget0+1, false, 3); n != 0 || s.g.stalled != 0 {
		t.Errorf("a lone scope record: damage %d stalled %d, want 0 / 0", n, s.g.stalled)
	}
}
