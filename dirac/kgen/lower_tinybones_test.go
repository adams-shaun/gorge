package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// tinybonesSrc is the activated ability alone: Tinybones, Trinket Thief's
// "each opponent with no cards in hand loses 10 life". The card's other
// trigger is not the subject here, so the probe carries only the A: line.
const tinybonesSrc = `Name:Tinybones Probe
ManaCost:1 B
Types:Legendary Creature Skeleton Rogue
PT:1/2
A:AB$ LoseLife | LifeAmount$ 10 | Defined$ Player.Opponent+HasCardsInHand_Card_EQ0 | Cost$ 4 B B
Oracle:{4}{B}{B}: Each opponent with no cards in hand loses 10 life.
`

// TestTinybonesHasCardsInHandLowers: DefinedLoseLife over
// Player.Opponent+HasCardsInHand_Card_EQ0 lowers to an each-opponent loop
// whose predicate is kop.PlayerPredHandEmpty, body one OpLoseLife of 10 on
// the loop's seat (gorge definedSpec: the controller skipped for base
// Opponent, each kept seat losing the life).
func TestTinybonesHasCardsInHandLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, diags := cards.ParseBytes("tinybones_probe.txt", []byte(tinybonesSrc))
	if card == nil {
		t.Fatalf("script did not parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
	lc := lowerCard(p, card.Faces[0].Name, card, false)
	for _, rf := range lc.Refusals {
		t.Errorf("refused: %s", rf.Reason)
	}
	if len(lc.Refusals) != 0 {
		t.FailNow()
	}
	if len(lc.Abilities) != 1 {
		t.Fatalf("lowered abilities = %d, want 1", len(lc.Abilities))
	}
	_, all, _, err := buildPool(&lowered{cards: []*lCard{lc}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(all) != 1 || len(all[0].def.Abilities) != 1 {
		t.Fatalf("built %d cards, want 1 with one ability", len(all))
	}
	prog := all[0].def.Abilities[0].Prog
	var loop, lose *kop.Instr
	for i := range prog {
		switch prog[i].Op {
		case kop.OpRepeatEachPlayer:
			loop = &prog[i]
		case kop.OpLoseLife:
			lose = &prog[i]
		}
	}
	if loop == nil || lose == nil {
		t.Fatalf("built prog has no each-player loop over a LoseLife: %+v", prog)
	}
	if loop.Obj != kop.RefEachOpponent {
		t.Errorf("loop seats = %v, want RefEachOpponent (the controller is skipped)", loop.Obj)
	}
	if loop.B.N != kop.PlayerPredHandEmpty {
		t.Errorf("loop predicate = %d, want PlayerPredHandEmpty (%d)", loop.B.N, kop.PlayerPredHandEmpty)
	}
	if lose.Obj != kop.RefEach || lose.A.N != 10 {
		t.Errorf("LoseLife Obj %v A %v, want the loop's seat and 10", lose.Obj, lose.A)
	}
}
