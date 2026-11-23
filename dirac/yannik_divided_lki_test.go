package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Yannik, Scavenging Sentinel's reflexive divided counters, at the writer:
// the battle-link remember pick (SrBattleLink, RememberChanged$, battlefield
// to exile) accumulates each picked card's last-known power into the
// resolution's remLK before it leaves (zone_hiddenpick.go pickLeaves), and
// the read is ExRemLKIPT over that sum, the capture
// OpReflexiveTrigger's B evaluates as the trigger is put on the stack. The
// card is not in a cardset pool yet (no set list names it), so the test pins
// the writer against a hand-built table of the lowered shape.
const (
	ykYan   CardID = 0 // the Yannik-shape card: the ETB trigger's program
	ykLions CardID = 1 // the exiled creature, 2/2
	ykPlain CardID = 2 // the library
)

func ykTable() *kop.Table {
	return &kop.Table{
		Cards: []kop.CardDef{
			{Name: "Yannik, Scavenging Sentinel", Types: kop.TCreature, Power: 3, Toughness: 3,
				Abilities: []kop.Ability{{
					Prog: []kop.Instr{
						// ChooseMove from the battlefield to exile, one card,
						// remembering (C&7 the destination zone, C>>3 the search
						// flags; B.N bit 9 the SrBattleLink until-leaves link)
						{Op: kop.OpChooseMove, A: kop.Const(1), B: kop.Const(kop.SrBattleLink),
							C: int16(int(kop.ZoneExile) | int(kop.SrRemember)<<3)},
						// the reflexive trigger: B is the ExRemLKIPT capture
						{Op: kop.OpReflexiveTrigger, A: kop.Const(1), B: kop.ValueRef{K: kop.VExpr, N: 1}, C: 1},
					},
				}}},
			{Name: "Savannah Lions", Types: kop.TCreature, Power: 2, Toughness: 2},
			{Name: "Plains"},
		},
		Exprs:   []kop.Expr{{}, {Kind: kop.ExRemLKIPT}},
		Filters: []kop.Filter{{}}, // index 0: ZoneNone, the battle-link pick's zoneless filter
		Conds:   []kop.Cond{},
	}
}

// TestYannikPickRemLKWriter: the pick's writer gate and sum.
func TestYannikPickRemLKWriter(t *testing.T) {
	sc := newScenTableLib(t, ykTable(), ykPlain, Config{NoElide: true})
	s, k := sc.s, sc.k
	if k.gen == nil {
		t.Fatal("the table is not a generated one")
	}
	if !k.gen.anyRemLKIPick {
		t.Fatal("anyRemLKIPick unset with an ExRemLKIPT expr in the table")
	}
	lions := sc.perm(ykLions, 0)
	s.perms[sc.slot(lions)].addCtrN(s, int(kop.CtrP1P1), 1)
	if got := s.power(k, sc.slot(lions)); got != 3 {
		t.Fatalf("setup: the countered Lions' power = %d, want 3", got)
	}
	// the writer's context: the pick is the live instruction of a live
	// resolution and the pick remembers
	s.res.active = true
	s.res.pc = 0
	s.g.pickF = kop.SrRemember
	pw, ok := s.pickRemLKPower(k, lions)
	if !ok || pw != 3 {
		t.Fatalf("pickRemLKPower = %d, %v, want 3, true (the Lions' last-known power)", pw, ok)
	}
	s.pickLeaves(k, lions, kop.ZoneExile)
	if z := s.objs[lions].zone; z != kop.ZoneExile {
		t.Fatalf("the Lions' zone = %d, want exile", z)
	}
	if got := s.remLKIPT(); got != 3 {
		t.Fatalf("the resolution's remLK sum = %d, want 3", got)
	}
	// the read the reflexive capture evaluates
	if got := s.value(k, kop.ValueRef{K: kop.VExpr, N: 1}); got != 3 {
		t.Fatalf("ExRemLKIPT reads %d, want 3", got)
	}
	t.Run("no reader in the table: no accumulation", func(t *testing.T) {
		k.gen.anyRemLKIPick = false
		second := sc.perm(ykLions, 0)
		if _, ok := s.pickRemLKPower(k, second); ok {
			t.Fatal("a battle-link pick accumulated with no ExRemLKIPT expr in the table")
		}
		s.pickLeaves(k, second, kop.ZoneExile)
		if got := s.remLKIPT(); got != 3 {
			t.Fatalf("the gate-off pick still moved the sum: %d, want 3", got)
		}
		k.gen.anyRemLKIPick = true
	})
	t.Run("a token is never written", func(t *testing.T) {
		tok := sc.perm(ykLions, 0)
		s.objs[tok].of |= ofToken
		pw, ok := s.pickRemLKPower(k, tok)
		if ok {
			t.Fatalf("a token's power %d joined the sum", pw)
		}
		s.pickLeaves(k, tok, kop.ZoneExile)
		if got := s.remLKIPT(); got != 3 {
			t.Fatalf("a token moved the sum: %d, want 3", got)
		}
	})
	t.Run("a pick that does not remember", func(t *testing.T) {
		third := sc.perm(ykLions, 0)
		saved := s.g.pickF
		s.g.pickF = 0
		defer func() { s.g.pickF = saved }()
		if _, ok := s.pickRemLKPower(k, third); ok {
			t.Fatal("a pick without SrRemember accumulated")
		}
		s.pickLeaves(k, third, kop.ZoneExile)
		if got := s.remLKIPT(); got != 3 {
			t.Fatalf("a non-remembering pick moved the sum: %d, want 3", got)
		}
	})
	t.Run("a pick that is not the battle link", func(t *testing.T) {
		fourth := sc.perm(ykLions, 0)
		// point the live instruction past the pick: the reflexive trigger is
		// not a battle-link pick, and the leave under it links and writes
		// nothing
		s.res.pc = 1
		defer func() { s.res.pc = 0 }()
		if _, ok := s.pickRemLKPower(k, fourth); ok {
			t.Fatal("a non-pick instruction accumulated")
		}
		s.pickLeaves(k, fourth, kop.ZoneExile)
		if got := s.remLKIPT(); got != 3 {
			t.Fatalf("a pick past its instruction moved the sum: %d, want 3", got)
		}
	})
}
