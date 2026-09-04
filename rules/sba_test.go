package rules

import (
	"fmt"
	"strings"
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

// TestDestroyLethalDamageDoesNotAmplifyWhenAReplacementKeepsThePermanent is
// the fix-round-1 regression test for the blocking finding: a MoveZone
// replacement that keeps a lethally-damaged permanent on the battlefield
// (a regeneration shield, "sacrifice a Clue instead", or -- as reproduced
// here -- a straight life-gain substitute) must not make checkStateBased's
// fixed-point loop believe something changed on every one of its
// maxSBAPasses passes. Before the fix, destroyLethalDamage reported
// "changed" from the MoveZone it emitted, not from whether the shield
// actually left; two checkStateBased calls per Submit (one directly, one
// via Advance -> step) times 32 wasted passes each meant 64 replacement
// firings and +64 life for a single "pass" intent. After the fix, each
// checkStateBased call attempts the destruction exactly once, sees the
// shield is still on the battlefield, and stops: 2 firings, +2 life --
// which is also exactly what one checkStateBased call per Submit produced
// before this task existed at all (BASE dec046a had no pass loop to
// exhaust in the first place).
func TestDestroyLethalDamageDoesNotAmplifyWhenAReplacementKeepsThePermanent(t *testing.T) {
	e := newSeats(t, 2)
	shield := onBoard(t, e, 0, `Name:Shield
ManaCost:1 G
Types:Creature Wall
PT:0/4
R:Event$ Moved | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | ReplaceWith$ RepLife
SVar:RepLife:DB$ GainLife | Defined$ You | LifeAmount$ 1
Oracle:x
`)
	e.emit(events.Event{Kind: events.Damage, Obj: shield, Amount: 4})

	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "pass" {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no pass option: %+v", d.Options)
	}
	beforeLife := e.G.Players[0].Life
	beforeEvents := len(e.L.Events)

	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit pass: %v", err)
	}

	if got := e.G.Obj(shield).Zone; got != state.ZBattlefield {
		t.Fatalf("shield zone = %s, want battlefield: the replacement keeps it there", got)
	}
	if gained := e.G.Players[0].Life - beforeLife; gained != 2 {
		t.Fatalf("life gained after one Submit = %d, want exactly 2 (one destruction attempt per "+
			"checkStateBased call, not the full 32-pass budget spent 64 times over)", gained)
	}
	if added := len(e.L.Events) - beforeEvents; added != 6 {
		t.Fatalf("log grew by %d events after one Submit, want exactly 6, not a 32x-amplified count", added)
	}
}

// TestRemovePermanentsDoesNotAmplifyWhenAReplacementKeepsThePermanent is the
// fix-round-1 regression test for the second instance of finding 1: an
// eliminated player's own permanent staying on the battlefield (via a
// replacement effect an alive player's own permanent carries -- ValidCard$
// on a replacement is matched against the MOVING object, not scoped to the
// mover's controller, so an alive player's ward can legally intercept
// anyone's exile) must not make checkLoseConditions' removal sweep believe
// something changed on every pass either. Ward, played by player 0
// (who stays alive), redirects any Battlefield -> Exile move into a
// 1-life gain instead; Victim, controlled by player 1 (who is eliminated),
// is what the removal sweep tries and fails to exile. Before the fix, "the
// zone was non-empty going in" was enough to report changed, regardless of
// whether removePermanents' own MoveZone actually did anything -- against
// the unfixed 7b68be7 code this scenario gains 32 life and grows the log by
// 36 events from one Submit; after the fix, the pass loop notices the
// removal is not succeeding and stops after the one pass it takes to
// confirm that, for 2 life and 6 events -- structurally the same bound
// destroyLethalDamage's own fix (previous test) achieves.
func TestRemovePermanentsDoesNotAmplifyWhenAReplacementKeepsThePermanent(t *testing.T) {
	e := newSeats(t, 2)
	onBoard(t, e, 0, `Name:Ward
ManaCost:1 W
Types:Artifact
R:Event$ Moved | Origin$ Battlefield | Destination$ Exile | ValidCard$ Card | ReplaceWith$ RepLife | Description$ x
SVar:RepLife:DB$ GainLife | Defined$ You | LifeAmount$ 1
Oracle:x
`)
	victim := onBoard(t, e, 1, `Name:Victim
ManaCost:1
Types:Artifact
Oracle:x
`)
	e.G.Players[1].Life = 0

	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "pass" {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no pass option: %+v", d.Options)
	}
	beforeLife := e.G.Players[0].Life
	beforeEvents := len(e.L.Events)

	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit pass: %v", err)
	}

	if !e.G.Players[1].Lost {
		t.Fatal("player 1 should have been eliminated at 0 life")
	}
	if got := e.G.Obj(victim).Zone; got != state.ZBattlefield {
		t.Fatalf("victim zone = %s, want battlefield: Ward's replacement keeps it there", got)
	}
	if gained := e.G.Players[0].Life - beforeLife; gained != 2 {
		t.Fatalf("life gained after one Submit = %d, want exactly 2, not a 32-pass-amplified count", gained)
	}
	if added := len(e.L.Events) - beforeEvents; added != 6 {
		t.Fatalf("log grew by %d events after one Submit, want exactly 6, not a 32x-amplified count", added)
	}
}

// TestStateBasedActionsExhaustingThePassBudgetAreReported is the fix-round-1
// regression test for the should-fix finding: maxSBAPasses exhaustion used
// to fall out of checkStateBased's loop with no trace at all. A chain of
// creatures, each independently lethal (1 damage on a 1-toughness body) but
// kept alive by the PREVIOUS creature in the chain granting it +2
// toughness -- the same one-death-per-pass shape TestSBALoopsUntilStable
// uses for two creatures, scaled past maxSBAPasses (32) so a single
// checkStateBased call cannot possibly reach a fixed point: creature i only
// becomes actually lethal once creature i-1 has already died, one creature
// per pass, and there are more creatures than passes in the budget. Before
// the fix, a client could observe a creature sitting on the battlefield
// with lethal damage marked and be handed priority over it, with nothing
// in the log explaining why. After the fix, a Note event says the budget
// was exhausted.
func TestStateBasedActionsExhaustingThePassBudgetAreReported(t *testing.T) {
	e := layerEngine(t)
	const n = maxSBAPasses + 8
	ids := make([]state.ObjID, n)
	for i := 0; i < n; i++ {
		src := fmt.Sprintf("Name:Link%d\nManaCost:1\nTypes:Creature Chain%d\nPT:1/1\nOracle:x\n", i, i)
		ids[i] = onBoard(t, e, 0, src)
		e.emit(events.Event{Kind: events.Damage, Obj: ids[i], Amount: 1})
	}
	for i := 1; i < n; i++ {
		e.AddContinuous(ContinuousEffect{Source: ids[i-1], Timestamp: uint32(i), Layer: LPT, Sub: SubModify,
			Affects: fmt.Sprintf("Chain%d", i), Controller: 0, AddToughness: 2})
	}

	beforeEvents := len(e.L.Events)
	e.checkStateBased()

	found := false
	for _, ev := range e.L.Events[beforeEvents:] {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "pass budget") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected a Note event reporting that the state-based-action pass budget was exhausted")
	}

	survivors := 0
	for _, id := range ids {
		if e.G.Obj(id).Zone == state.ZBattlefield {
			survivors++
		}
	}
	if survivors == 0 {
		t.Fatal("expected some creatures to still be on the battlefield: the chain must outlast the pass budget for this to be a real test of exhaustion")
	}
	if e.G.Obj(ids[0]).Zone != state.ZGraveyard {
		t.Fatal("the first, independently-lethal creature in the chain should already be dead")
	}
}

// TestDestructionTextDistinguishesToughnessFromLethalDamage is the
// fix-round-1 regression test for the reviewer's note: CR 704.5f
// (toughness <= 0) and CR 704.5g (lethal damage/deathtouch) are
// rules-distinct -- only one of them is actually "destruction" -- and the
// log should be able to tell them apart even though Text carries no rules
// weight of its own.
func TestDestructionTextDistinguishesToughnessFromLethalDamage(t *testing.T) {
	e := layerEngine(t)
	shrunk := onBoard(t, e, 0, "Name:Shrunk\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.AddContinuous(ContinuousEffect{Source: shrunk, Timestamp: 1, Layer: LPT, Sub: SubModify,
		Affects: "Card.Self", AddPower: -3, AddToughness: -3})
	lethal := onBoard(t, e, 0, "Name:Lethal\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.emit(events.Event{Kind: events.Damage, Obj: lethal, Amount: 2})

	beforeEvents := len(e.L.Events)
	e.checkStateBased()

	var shrunkText, lethalText string
	for _, ev := range e.L.Events[beforeEvents:] {
		if ev.Kind != events.MoveZone {
			continue
		}
		switch ev.Obj {
		case shrunk:
			shrunkText = ev.Text
		case lethal:
			lethalText = ev.Text
		}
	}
	if shrunkText != "toughness <= 0" {
		t.Fatalf("zero-toughness destruction Text = %q, want %q", shrunkText, "toughness <= 0")
	}
	if lethalText != "lethal damage" {
		t.Fatalf("lethal-damage destruction Text = %q, want %q", lethalText, "lethal damage")
	}
	if shrunkText == lethalText {
		t.Fatal("CR 704.5f and 704.5g are rules-distinct and must not share the same Text")
	}
}
