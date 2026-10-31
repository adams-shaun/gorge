package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestConditionTargetedReadsLKIState pins CR 608.2h for the state a departing
// permanent loses as it moves: "if it was tapped" and "if it was attacking" ask
// about the target as it last existed on the battlefield, and each of these
// spells has already moved it when the gate runs.
func TestConditionTargetedReadsLKIState(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	tapped := func(_ *Engine, o *state.Object) { o.Tapped = true }
	attacking := func(_ *Engine, o *state.Object) { o.IsAttacking = true }
	damaged := func(e *Engine, o *state.Object) {
		e.emit(events.Event{Kind: events.Damage, Obj: o.ID, Amount: 1})
	}
	const makesToken, ownerDraws = "token", "draw"
	for _, tc := range []struct {
		name, spell string
		prepare     func(*Engine, *state.Object)
		effect      string
		want        bool
	}{
		{"tapped target makes a Map", "Brackish Blunder", tapped, makesToken, true},
		{"untapped target does not", "Brackish Blunder", nil, makesToken, false},
		{"attacking target makes a Drone", "Desculpting Blast", attacking, makesToken, true},
		{"idle target does not", "Desculpting Blast", nil, makesToken, false},
		{"damaged target makes a Clue", "Sold Out", damaged, makesToken, true},
		{"undamaged target does not", "Sold Out", nil, makesToken, false},
		{"attacking target draws nothing", "Prophesied End", attacking, ownerDraws, false},
		{"idle target draws", "Prophesied End", nil, ownerDraws, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			spellCard, ok := reg.Lookup(tc.spell)
			if !ok {
				t.Fatalf("corpus missing %s", tc.spell)
			}
			e := handEngineTokens(t, spellCard)
			target := e.G.Obj(onBoard(t, e, 1, "Name:Quarry\nTypes:Creature Bear\nPT:4/4\nOracle:x\n"))
			if tc.prepare != nil {
				tc.prepare(e, target)
			}
			for _, m := range []int{state.MW, state.MU, state.MB, state.MR, state.MG} {
				e.G.Players[0].Pool[m] = 3
			}
			hand := len(e.G.Zone(state.ZHand, 1))
			spell := e.G.Zone(state.ZHand, 0)[0]
			castMode(t, e, spell, "")
			submitChoices(t, e, optionForObj(e.Pending(), target.ID))
			passUntilStackEmpty(t, e, 20)
			if target.Zone == state.ZBattlefield {
				t.Fatalf("%s left its target on the battlefield", tc.spell)
			}
			var got bool
			if tc.effect == ownerDraws {
				got = len(e.G.Zone(state.ZHand, 1)) > hand
			} else {
				for _, id := range e.G.Zone(state.ZBattlefield, 0) {
					got = got || e.G.Obj(id).IsToken
				}
			}
			if got != tc.want {
				t.Fatalf("%s: rider happened = %v, want %v", tc.spell, got, tc.want)
			}
		})
	}
}
