package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRepeatEachRememberedAskLowers plays the asking remembered-set RepeatEach
// in lockstep with gorge over the repeatrememberedask fixture pool
// (dirac/cardset/pools/fixtures/repeatrememberedask): Stench of Evil, "destroy
// all Plains; for each land destroyed this way it deals 1 damage to that
// land's controller unless they pay {2}". The loop is OpRepeatEach with C
// RepeatRemembered|RepeatPlain|RepeatDmgBatch: a resumable frame over g.rem
// whose body is [UnlessPaid, DealDamage] for RefEach, the destroyed Plains'
// controller, and whose damage batch stays open across every pass's ask.
//
// Preconditions: Stench is cast. Mutation: the payment asked of the caster
// instead of the Plains' controller (a different seat answers, so the games
// diverge).
func TestRepeatEachRememberedAskLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("lockstep", func(t *testing.T) {
		run := runFixtureLockstep(t, "repeatrememberedask", repeatRememberedAskDecks, repeatRememberedAskWatch, 200, true, nil)
		t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
		if run.casts["Stench of Evil"] < 10 {
			t.Errorf("only %d Stench of Evil casts (want 10)", run.casts["Stench of Evil"])
		}
	})
	t.Run("mutation/PayerIsCaster", func(t *testing.T) {
		n := 0
		run := runFixtureLockstep(t, "repeatrememberedask", repeatRememberedAskDecks, repeatRememberedAskWatch, 200, false, func(tbl *kop.Table) {
			mutateCardIfPresent(t, tbl, "Stench of Evil", func(_ *kop.Ability, in *kop.Instr) {
				if in.Op == kop.OpUnlessPaid && in.Obj == kop.RefEach {
					in.Obj = kop.RefController
					n++
				}
			})
		})
		if n == 0 {
			t.Fatal("no UnlessPaid for RefEach in the table (the asking remembered loop is not lowered)")
		}
		run.requireDivergence(t, "the Plains' controller's payment asked of the caster")
	})
}

var repeatRememberedAskWatch = map[string]bool{"Stench of Evil": true}

var repeatRememberedAskDecks = []fixtureDecks{
	{
		{
			{Name: "Stench of Evil", Count: 6}, {Name: "Plains", Count: 6}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 4},
			{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		},
		{
			{Name: "Plains", Count: 12}, {Name: "Mountain", Count: 6}, {Name: "Swamp", Count: 4}, {Name: "Forest", Count: 2},
			{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 6}, {Name: "Llanowar Elves", Count: 4},
		},
	},
	{
		{
			{Name: "Stench of Evil", Count: 4}, {Name: "Plains", Count: 10}, {Name: "Swamp", Count: 8},
			{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 6}, {Name: "Llanowar Elves", Count: 4},
		},
		{
			{Name: "Stench of Evil", Count: 4}, {Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 4},
			{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 6}, {Name: "Llanowar Elves", Count: 4},
		},
	},
}
