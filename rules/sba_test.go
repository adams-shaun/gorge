package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestZeroLifeEliminatesAPlayer is CR 704.5a: a player at 0 or less life
// loses the game. Three seats so the assertion that the game CONTINUES
// (Over stays false, with two seats still standing) is meaningful -- a
// two-seat version of this test could not tell "the loser was correctly
// eliminated" apart from "the game incorrectly ended for everyone".
func TestZeroLifeEliminatesAPlayer(t *testing.T) {
	e := newSeats(t, 3)
	e.G.Players[1].Life = 0

	e.checkStateBased()

	if !e.G.Players[1].Lost {
		t.Fatal("player at 0 life should be marked Lost")
	}
	if e.G.Players[0].Lost || e.G.Players[2].Lost {
		t.Fatalf("only player 1 should be eliminated: player0.Lost=%v player2.Lost=%v",
			e.G.Players[0].Lost, e.G.Players[2].Lost)
	}
	if e.G.Over {
		t.Fatal("the game must continue with two seats still standing")
	}
}

// TestLethalDamageDestroysACreature: damage marked at least equal to
// toughness is a destroying state-based action (CR 704.5g).
func TestLethalDamageDestroysACreature(t *testing.T) {
	e := layerEngine(t)
	bear := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.emit(events.Event{Kind: events.Damage, Obj: bear, Amount: 2})

	e.checkStateBased()

	if got := e.G.Obj(bear).Zone; got != state.ZGraveyard {
		t.Fatalf("bear zone = %s, want graveyard (2 damage on a 2-toughness creature is lethal)", got)
	}
}

// TestDeathtouchDamageDestroysRegardlessOfAmount: any damage at all from a
// deathtouch source destroys, even on a creature with plenty of toughness
// to spare (CR 704.5g / 702.2c, modelled here the same way combat.go's own
// damageStep marks it -- a Deathtouched counter alongside the damage).
func TestDeathtouchDamageDestroysRegardlessOfAmount(t *testing.T) {
	e := layerEngine(t)
	troll := onBoard(t, e, 0, "Name:Troll\nManaCost:3 G\nTypes:Creature Troll\nPT:4/4\nOracle:x\n")
	e.emit(events.Event{Kind: events.Damage, Obj: troll, Amount: 1})
	e.emit(events.Event{Kind: events.CounterChange, Obj: troll, Counter: "Deathtouched", Amount: 1})

	e.checkStateBased()

	if got := e.G.Obj(troll).Zone; got != state.ZGraveyard {
		t.Fatalf("troll zone = %s, want graveyard (1 deathtouch damage on a 4-toughness creature is still lethal)", got)
	}
}

// TestZeroToughnessDies: a -3/-3 effect on a 2/2 puts it at -1 toughness,
// which is destruction with no damage involved at all (CR 704.5f).
func TestZeroToughnessDies(t *testing.T) {
	e := layerEngine(t)
	bear := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.AddContinuous(ContinuousEffect{Source: bear, Timestamp: 1, Layer: LPT, Sub: SubModify,
		Affects: "Card.Self", Controller: 0, AddPower: -3, AddToughness: -3})
	if got := e.Toughness(bear); got != -1 {
		t.Fatalf("toughness after -3/-3 = %d, want -1", got)
	}

	e.checkStateBased()

	if got := e.G.Obj(bear).Zone; got != state.ZGraveyard {
		t.Fatalf("bear zone = %s, want graveyard (0 or less toughness, no damage needed)", got)
	}
}

// TestIndestructibleSurvivesLethalDamageButNotZeroToughness: Indestructible
// (CR 702.12b) prevents destruction by lethal damage, but CR 704.5f's
// zero-toughness rule is not destruction and is not an exception
// Indestructible covers.
func TestIndestructibleSurvivesLethalDamageButNotZeroToughness(t *testing.T) {
	e := layerEngine(t)
	tough := onBoard(t, e, 0, "Name:Juggernaut\nManaCost:3 R\nTypes:Creature Juggernaut\nPT:2/2\nK:Indestructible\nOracle:x\n")
	e.emit(events.Event{Kind: events.Damage, Obj: tough, Amount: 5})

	shrunk := onBoard(t, e, 0, "Name:Shrinkable\nManaCost:3 R\nTypes:Creature Juggernaut\nPT:2/2\nK:Indestructible\nOracle:x\n")
	e.AddContinuous(ContinuousEffect{Source: shrunk, Timestamp: 1, Layer: LPT, Sub: SubModify,
		Affects: "Card.Self", Controller: 0, AddPower: -5, AddToughness: -5})

	e.checkStateBased()

	if got := e.G.Obj(tough).Zone; got != state.ZBattlefield {
		t.Fatalf("indestructible creature with lethal damage: zone = %s, want battlefield (survives)", got)
	}
	if got := e.G.Obj(shrunk).Zone; got != state.ZGraveyard {
		t.Fatalf("indestructible creature at 0 toughness: zone = %s, want graveyard (Indestructible does not save this)", got)
	}
}

// TestSBALoopsUntilStable proves CR 704.3's "checked repeatedly, not once":
// a lord grants another creature +0/+2 toughness for as long as the lord is
// on the battlefield. The lord itself has lethal damage marked, so it dies
// in the FIRST internal pass; only once the lord is actually gone does the
// bear's effective toughness drop back down far enough for its own,
// already-marked damage to become lethal too. Both must be gone after one
// call to checkStateBased -- if the loop only ran the destruction check
// once, the bear would incorrectly survive.
func TestSBALoopsUntilStable(t *testing.T) {
	e := layerEngine(t)
	lord := onBoard(t, e, 0, "Name:Lord\nManaCost:2 W\nTypes:Creature Human\nPT:1/1\nOracle:x\n")
	bear := onBoard(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.AddContinuous(ContinuousEffect{Source: lord, Timestamp: 1, Layer: LPT, Sub: SubModify,
		Affects: "Creature.YouCtrl+Other", Controller: 0, AddToughness: 2})
	if got := e.Toughness(bear); got != 4 {
		t.Fatalf("bear toughness with the lord's boost = %d, want 4", got)
	}

	e.emit(events.Event{Kind: events.Damage, Obj: bear, Amount: 3})
	e.emit(events.Event{Kind: events.Damage, Obj: lord, Amount: 1})

	e.checkStateBased()

	if got := e.G.Obj(lord).Zone; got != state.ZGraveyard {
		t.Fatalf("lord zone = %s, want graveyard (1 damage on a 1-toughness creature is lethal)", got)
	}
	if got := e.G.Obj(bear).Zone; got != state.ZGraveyard {
		t.Fatalf("bear zone = %s, want graveyard: once the lord is gone its toughness drops to 2 and the "+
			"already-marked 3 damage becomes lethal -- this needs a second internal SBA pass", got)
	}
}

// TestGameEndsWithOneSurvivor: four seats, three eliminated -- Over must
// become true and Winner must name the one seat left standing, not just
// "some" seat.
func TestGameEndsWithOneSurvivor(t *testing.T) {
	e := newSeats(t, 4)
	e.G.Players[0].Life = 0
	e.G.Players[1].Life = 0
	e.G.Players[3].Life = 0

	e.checkStateBased()

	if !e.G.Over {
		t.Fatal("game should be over with only one seat left")
	}
	if e.G.Winner != 2 {
		t.Fatalf("winner = %d, want 2 (the only survivor)", e.G.Winner)
	}
	if e.G.Draw {
		t.Fatal("a game with a real winner is not a draw")
	}
}

// TestGameEndsWithZeroSurvivors: simultaneous elimination is a draw (CR
// 104.4a), not a win for whichever seat happens to be numbered 0. This is
// the regression test for stubs.go's old fallback, which unconditionally
// named seat 0 the winner when nobody survived.
func TestGameEndsWithZeroSurvivors(t *testing.T) {
	e := newSeats(t, 2)
	e.G.Players[0].Life = 0
	e.G.Players[1].Life = 0

	e.checkStateBased()

	if !e.G.Over {
		t.Fatal("game should be over with no seats left")
	}
	if !e.G.Draw {
		t.Fatal("a game where every seat is eliminated simultaneously must be recorded as a draw")
	}
}

// TestEliminatedPlayersPermanentsLeaveTheBattlefield: an eliminated player's
// permanents do not linger on a battlefield with nobody left to control
// them (CR 800.4a-shaped cleanup this build approximates as exile, the same
// zone resolveTop already uses for "no equivalent zone exists").
func TestEliminatedPlayersPermanentsLeaveTheBattlefield(t *testing.T) {
	e := newSeats(t, 3)
	a := onBoard(t, e, 1, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	b := onBoard(t, e, 1, "Name:Goat\nManaCost:1 G\nTypes:Creature Goat\nPT:1/1\nOracle:x\n")
	e.G.Players[1].Life = 0

	e.checkStateBased()

	if got := e.G.Zone(state.ZBattlefield, 1); len(got) != 0 {
		t.Fatalf("eliminated player's battlefield = %v, want empty", got)
	}
	if e.G.Obj(a).Zone == state.ZBattlefield || e.G.Obj(b).Zone == state.ZBattlefield {
		t.Fatalf("both permanents should have left the battlefield: a=%s b=%s",
			e.G.Obj(a).Zone, e.G.Obj(b).Zone)
	}
}

// TestNoDecisionIsIssuedAfterGameOver drives elimination through the real
// Submit path (not a direct checkStateBased call): the non-active player is
// brought to 0 life, then the active player passes priority. That pass is
// what runs checkStateBased inside Submit, which must both end the game and
// leave no decision outstanding -- a client must never be asked to answer
// anything once Over is true.
func TestNoDecisionIsIssuedAfterGameOver(t *testing.T) {
	e := newSeats(t, 2)
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("expected a priority decision to start, got %+v", d)
	}
	var victim state.PlayerID
	for p := state.PlayerID(0); p < 2; p++ {
		if p != d.Player {
			victim = p
		}
	}
	e.G.Players[victim].Life = 0

	idx := -1
	for _, o := range d.Options {
		if o.Kind == "pass" {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no pass option: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit pass: %v", err)
	}

	if !e.G.Over {
		t.Fatal("eliminating the only other seat must end the game on this pass")
	}
	if e.Pending() != nil {
		t.Fatalf("a decision is still pending after Over: %+v", e.Pending())
	}
}
