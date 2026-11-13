package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestEnchantedBySecondLeg is the kernel half of the Commander curses'
// second leg: the Aura's Tf4DefEnch gate is source-relative, and the leg's
// SkipUnless guard gives the second token to the DECLARING player only when
// that player is an opponent of the Aura's controller. The program is the
// one kgen lowers for Curse of Opulence/Verbosity/Vitality/Bounty, written
// here directly so the kernel semantics are pinned without a corpus table.
func TestEnchantedBySecondLeg(t *testing.T) {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Conds = append([]kop.Cond(nil), v.Table.Conds...)
	cond := kop.CondID(len(tbl.Conds))
	tbl.Conds = append(tbl.Conds, kop.Cond{Kind: kop.CondYourTurn, Neg: true})
	gold := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Gold", Types: kop.TArtifact})
	curse := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Curse", Types: kop.TEnchantment | kop.TAura, Enchant: kop.TargetSpec{Kind: kop.TgtPlayer},
		Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigAttackersDeclared, Flags2: kop.Tf2DefPlayer, Flags4: kop.Tf4DefEnch},
			Prog: []kop.Instr{
				{Op: kop.OpSkipUnless, C: int16(cond), A: kop.Const(1)},
				{Op: kop.OpCreateToken, Obj: kop.RefTriggerPlayer, A: kop.Const(1), C: int16(gold)},
			}}}})

	run := func(ctrl uint8) (tokens uint8) {
		sc := newScenTable(t, &tbl, Config{})
		curseObj := sc.perm(curse, ctrl)
		cs := sc.slot(curseObj)
		sc.s.setAtchSeat(sc.k, cs, 2) // enchants seat 1 (the seat the other player attacks)
		bear := sc.perm(v.GrizzlyBears, 0)
		bs := sc.slot(bear)
		sc.s.perms[bs].atk = uint8(PlayerTarget0 + 1)
		if sc.s.atchSeat(sc.k, cs) != 2 || sc.slot(bear) == cs {
			t.Fatalf("precondition: curse must enchant seat 1 (atchSeat=%d)", sc.s.atchSeat(sc.k, cs))
		}
		l := trigListener{obj: curseObj, sl: cs, ctrl: ctrl, card: curse, ab: 0}
		sc.s.fireDefPlayer(sc.k, l, &tbl.Cards[curse].Abilities[0], []Slot{bs})
		if sc.s.npend != 1 {
			t.Fatalf("controller %d: firing the trigger queued %d pending, want 1", ctrl, sc.s.npend)
		}
		for i := 0; i < 40; i++ {
			d := sc.s.Step()
			if sc.countOnBF(gold) > 0 || sc.s.npend == 0 && sc.s.sp == 0 && !sc.s.res.active {
				break // the trigger resolved, or its guard skipped it
			}
			if d.Kind == DkNone {
				t.Fatalf("controller %d: game ended before the trigger resolved", ctrl)
			}
			applied := false
			for _, a := range d.Options[:d.N] {
				if a.Kind() == AkPass {
					sc.s.Apply(a)
					applied = true
					break
				}
			}
			if !applied {
				t.Fatalf("controller %d: unexpected decision %d/%d player %d opts %v", ctrl, d.Kind, d.Sub, d.Player, d.Options[:d.N])
			}
		}
		for o := Obj(0); o < Obj(sc.s.nobj); o++ {
			ob := &sc.s.objs[o]
			if ob.zone == kop.ZoneBattlefield && ob.card == gold {
				if ob.owner() != 0 {
					t.Fatalf("controller %d: the second-leg token went to seat %d, want the trigger player 0", ctrl, ob.owner())
				}
				tokens++
			}
		}
		return tokens
	}

	// The ordinary "curse on myself" case: seat 1's curse enchants seat 1,
	// and the active (declaring) seat 0 attacks it. Seat 0 is an opponent of
	// the controller, so it receives the second token.
	if got := run(1); got != 1 {
		t.Fatalf("opponent attacking the enchanted player got %d second-leg tokens, want 1", got)
	}
	// The controller itself attacks the player it enchanted: Player.Opponent
	// excludes it, so the guard skips the leg.
	if got := run(0); got != 0 {
		t.Fatalf("the controller attacking its own enchanted player got %d second-leg tokens, want 0", got)
	}
}

// countOnBF counts the battlefield objects of card c.
func (sc *scen) countOnBF(c CardID) int {
	n := 0
	for o := Obj(0); o < Obj(sc.s.nobj); o++ {
		if ob := &sc.s.objs[o]; ob.zone == kop.ZoneBattlefield && ob.card == c {
			n++
		}
	}
	return n
}
