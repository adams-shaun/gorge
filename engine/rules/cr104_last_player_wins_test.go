package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCR104_2aLastPlayerWinsDespiteCantWin: CR 104.2a, "A player still in the
// game wins the game if that player's opponents have all left the game. This
// happens immediately and overrides all effects that would preclude that
// player from winning the game." Seat 1 controls the real Abyssal Persecutor
// ("You can't win the game and your opponents can't lose the game"); seat 0
// concedes (CR 104.3a, which no effect stops). Seat 1 is the last player in
// the game and wins at once.
func TestCR104_2aLastPlayerWinsDespiteCantWin(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	persecutor := searchCorpusCard(t, reg, "Abyssal Persecutor")
	if d := persecutor.Link(); len(d) != 0 {
		t.Fatalf("link Abyssal Persecutor: %v", d)
	}
	cfg := seatZeroStart(Config{Seed: 8310, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{mountainDeck(t, 40), append([]*cards.Card{persecutor}, mountainDeck(t, 39)...)},
		Tokens: reg.Tokens, NameUniverse: reg.Universe()})
	e := New(cfg)
	e.Advance()
	pid := moveNamedToBattlefield(t, e, 1, "Abyssal Persecutor")
	o := e.G.Obj(pid)
	hasGameWin := false
	for i := range o.Face().Repls {
		if o.Face().Repls[i].Event == "GameWin" {
			hasGameWin = true
		}
	}
	if o.Zone != state.ZBattlefield || o.Controller != 1 || !hasGameWin {
		t.Fatalf("CR 104.2a Abyssal Persecutor seq %d: precondition: not on seat 1's battlefield with its Event$ GameWin line: %+v", len(e.L.Events), o)
	}
	if !e.gameWinPrevented(1) {
		t.Fatalf("CR 104.2a Abyssal Persecutor seq %d: precondition: the GameWin CantHappen replacement does not apply to seat 1", len(e.L.Events))
	}
	e.pending = nil
	e.priorityRound()
	opt := highestPriorityOptionWithKind(t, e, "concede")
	submitChoices(t, e, opt.Index)
	if !e.G.Players[0].Lost {
		t.Fatalf("CR 104.2a Abyssal Persecutor seq %d: seat 0 conceded but is not Lost", len(e.L.Events))
	}
	if !e.G.Over || e.G.Winner != 1 || e.G.Draw {
		t.Fatalf("CR 104.2a Abyssal Persecutor seq %d: over=%v winner=%d draw=%v; want seat 1, the last player in the game, to win despite \"you can't win\"",
			len(e.L.Events), e.G.Over, e.G.Winner, e.G.Draw)
	}
	replayCheck(t, e, cfg)
}
