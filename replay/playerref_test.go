package replay

import (
	"context"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/seat"
	"github.com/adams-shaun/gorge/view"
)

// raiderSrc is a Haste attacker whose own Attacks trigger reads Defined$
// TriggeredDefendingPlayer (Task 5's form, effects/context.go's playersOf)
// to drain the DEFENDING player specifically. FL-41: before pushTrigger
// encoded a Remembered player entry with state.PlayerRef, that entry
// persisted through the logged TriggerPush event as ObjID 0 (an object
// that never exists), Apply rebuilt it as {Obj: 0}, and playersOf filters
// that entry out -- so Defined$ TriggeredDefendingPlayer resolves to
// nothing and the effect silently no-ops (PlayerOf is never reached).
// Haste means Raider can attack the very turn it resolves, so a two-seat
// game reaches this in well under a full turn.
const raiderSrc = `Name:Raider
ManaCost:R
Types:Creature Goblin
PT:2/2
K:Haste
T:Mode$ Attacks | ValidCard$ Card.Self | Execute$ TrigLose | TriggerDescription$ x
SVar:TrigLose:DB$ LoseLife | Defined$ TriggeredDefendingPlayer | LifeAmount$ 3
Oracle:x
`

// compileCard parses and links a bare Forge script the same way every
// rules-package fixture test does (rules/turn_test.go's own card()) --
// duplicated here rather than exported from rules, since this test lives in
// package replay specifically to reach replay.Replay without an import
// cycle (rules cannot import replay).
func compileCard(t *testing.T, src string) *cards.Card {
	t.Helper()
	c, diags := cards.ParseBytes("t.txt", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("diags: %v", diags)
	}
	c.Link()
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	return c
}

// raiderConfig builds a deterministic 2-seat Config: seat 0 leads with
// Raider ahead of 39 Mountains, seat 1 is a plain 40-Mountain deck with no
// blockers, so Raider's attack always connects and the defending seat is
// unambiguous.
func raiderConfig(t *testing.T) rules.Config {
	t.Helper()
	raider := compileCard(t, raiderSrc)
	mountain := compileCard(t, "Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n")
	deck0 := make([]*cards.Card, 40)
	deck0[0] = raider
	for i := 1; i < 40; i++ {
		deck0[i] = mountain
	}
	deck1 := make([]*cards.Card, 40)
	for i := range deck1 {
		deck1[i] = mountain
	}
	return rules.Config{Seed: 11, Names: []string{"attacker", "defender"},
		Decks: [][]*cards.Card{deck0, deck1}}
}

// TestDeclareAttackersPlayerRefSurvivesReplay is FL-41's own regression
// test: it drives a real game (the deterministic seat.Bot, exactly
// playGame's own wiring, not a rules-package-internal shortcut) through
// Raider's cast and its very first attack, checks the LoseLife landed on
// the DEFENDING player (not the attacker -- the bug this fixes would swap
// them), and then reconstructs the same match from (cfg, log) alone via
// the real replay.Replay -- proving the state.PlayerRef sentinel actually
// round-trips through a log, not just through the live engine that
// produced it.
func TestDeclareAttackersPlayerRefSurvivesReplay(t *testing.T) {
	cfg := raiderConfig(t)
	e := rules.New(cfg)
	e.Advance()
	b := seat.NewBot(cfg.Seed)
	n := 0
	for !e.G.Over && e.Pending() != nil && e.G.Turn < 2 && n < maxIntents {
		d := e.Pending()
		v := view.Project(e.G, e, d.Player, d)
		in, err := b.Decide(context.Background(), v, *d)
		if err != nil {
			t.Fatalf("bot Decide returned an error: %v", err)
		}
		if err := e.Submit(in); err != nil {
			t.Fatalf("Submit intent %d: %v", n, err)
		}
		n++
	}
	if e.G.Turn < 2 {
		t.Fatalf("turn 1 did not complete within the intent budget (stuck at turn %d)", e.G.Turn)
	}
	// 20 starting life; unblocked 2/2 combat damage plus the 3-life
	// TrigLose landing correctly on the defender: 20 - 2 - 3 = 15. If FL-41
	// regressed, the effect silently no-ops (playersOf drops the {Obj: 0}
	// entry): defender stays at 18 (combat damage only) and the attacker
	// stays at 20.
	if e.G.Players[1].Life != 15 {
		t.Fatalf("defending player life = %d, want 15 (20 - 2 combat - 3 TrigLose on the DEFENDER)",
			e.G.Players[1].Life)
	}
	if e.G.Players[0].Life != 20 {
		t.Fatalf("attacking player life = %d, want unchanged 20 -- "+
			"FL-41 regression: TrigLose silently no-oped (playersOf dropped the Obj:0 entry)", e.G.Players[0].Life)
	}

	// Now the round trip the finding actually asked for: reconstruct from
	// (cfg, log) alone, the way a real replay-from-storage would, and prove
	// the reconstructed game agrees -- including the two players' life
	// totals, not just the chain head -- so a PlayerRef that decoded
	// differently on replay than it did live could not hide behind a head
	// that happened to still match.
	re, err := Replay(e.L, cfg)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if re.G.Players[0].Life != e.G.Players[0].Life || re.G.Players[1].Life != e.G.Players[1].Life {
		t.Fatalf("replay diverged: got P0=%d P1=%d, want P0=%d P1=%d",
			re.G.Players[0].Life, re.G.Players[1].Life, e.G.Players[0].Life, e.G.Players[1].Life)
	}
	if re.L.Head() != e.L.Head() {
		t.Fatalf("replay chain head diverged: %s vs %s", re.L.Head(), e.L.Head())
	}
}
