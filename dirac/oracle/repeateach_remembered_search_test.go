package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRepeatEachRememberedSearchLowers plays Wave of Vitriol in lockstep with
// gorge over the repeateachrememberedsearch fixture pool
// (dirac/cardset/pools/fixtures/repeateachrememberedsearch): "Each player
// sacrifices all artifacts, enchantments, and nonbasic lands they control. For
// each land sacrificed this way, its controller may search their library for a
// basic land card and put it onto the battlefield tapped. Then each player who
// searched their library this way shuffles."
//
// The loop is OpRepeatEach with C RepeatRemembered|RepeatPlain over g.rem: a
// resumable pass per sacrificed card, with RefEach (the bound card) read as a
// player naming its controller. Each pass asks that controller's optional
// confirm and, on a yes, searches their library for a basic land, remembering
// the searcher's seat. The Shuffle Defined$ Player.IsRemembered after the loop
// shuffles exactly those seats, once each.
//
// Preconditions: Wave is cast while both players control a nonbasic land, an
// artifact and an enchantment, so both seats search. Mutation: the search posed
// to the caster instead of the sacrificed card's controller (a different seat
// answers and different libraries change, so the games diverge).
func TestRepeatEachRememberedSearchLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("lockstep", func(t *testing.T) {
		run := runFixtureLockstep(t, "repeateachrememberedsearch", repeatRememberedSearchDecks, repeatRememberedSearchWatch, 200, true, nil)
		t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
		if run.casts["Wave of Vitriol"] < 10 {
			t.Errorf("only %d Wave of Vitriol casts (want 10)", run.casts["Wave of Vitriol"])
		}
	})
	t.Run("mutation/SearchForCaster", func(t *testing.T) {
		n := 0
		run := runFixtureLockstep(t, "repeateachrememberedsearch", repeatRememberedSearchDecks, repeatRememberedSearchWatch, 200, false, func(tbl *kop.Table) {
			mutateCardIfPresent(t, tbl, "Wave of Vitriol", func(_ *kop.Ability, in *kop.Instr) {
				if in.Op == kop.OpSearchEx && in.Obj == kop.RefEach {
					in.Obj = kop.RefController
					n++
				}
			})
		})
		if n == 0 {
			t.Fatal("no Search for RefEach in the table (the remembered-searcher loop is not lowered)")
		}
		run.requireDivergence(t, "the basic-land search posed to the caster")
	})
}

var repeatRememberedSearchWatch = map[string]bool{"Wave of Vitriol": true}

var repeatRememberedSearchDecks = []fixtureDecks{
	{
		{
			// green: Wave, the nonbasic land, artifact, enchantment and basics
			{Name: "Wave of Vitriol", Count: 5}, {Name: "Radiant Fountain", Count: 4},
			{Name: "Bonesplitter", Count: 4}, {Name: "Spirit Link", Count: 4},
			{Name: "Grizzly Bears", Count: 8}, {Name: "Llanowar Elves", Count: 5},
			{Name: "Forest", Count: 10},
		},
		{
			// the mirror with a red threat and white mana for Spirit Link
			{Name: "Wave of Vitriol", Count: 5}, {Name: "Radiant Fountain", Count: 4},
			{Name: "Bonesplitter", Count: 4}, {Name: "Spirit Link", Count: 4},
			{Name: "Hill Giant", Count: 7}, {Name: "Grizzly Bears", Count: 4},
			{Name: "Llanowar Elves", Count: 2}, {Name: "Forest", Count: 6}, {Name: "Plains", Count: 4},
		},
	},
}
