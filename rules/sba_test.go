package rules

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
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
// fix-round-1 regression test for the second instance of finding 1,
// updated in fix round 3 for the tighter bound T22-n's swept set achieves.
// An eliminated player's own permanent staying on the battlefield (via a
// replacement effect an alive player's own permanent carries -- ValidCard$
// on a replacement is matched against the MOVING object, not scoped to the
// mover's controller, so an alive player's ward can legally intercept
// anyone's exile) must not make checkLoseConditions' removal sweep believe
// something changed on every pass either. Ward, played by player 0
// (who stays alive), redirects any Battlefield -> Exile move into a
// 1-life gain instead; Victim, controlled by player 1 (who is eliminated),
// is what the removal sweep tries and fails to exile.
//
// Against the unfixed 7b68be7 code (no bound at all) this scenario gains 32
// life and grows the log by 36 events from one Submit. Fix round 1's
// before/after-zone-length check bounded that, but with no memory of a
// prior attempt: pass 1 attempts the sweep (blocked); pass 2 finds the
// zone still non-empty and attempts it AGAIN before finally reporting no
// change and stopping -- 2 attempts, 2 life, 6 events. Fix round 3's swept
// set (T22-n, mirroring destroyLethalDamage's attempted from round 2)
// remembers that player 1 was already swept on pass 1, so pass 2 skips the
// attempt entirely instead of repeating it: exactly 1 attempt per player
// per checkStateBased call, for 1 life and 5 events -- the number moved
// (from round 1's 2/6, confirmed by rerunning this exact scenario before
// the round-3 fix) because the bound genuinely got tighter, not because
// anything broke; see the fix-round-3 report section for the full trace.
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
	if gained := e.G.Players[0].Life - beforeLife; gained != 1 {
		t.Fatalf("life gained after one Submit = %d, want exactly 1 (one swept attempt per player "+
			"per checkStateBased call, not a re-attempt on every pass and not a 32-pass-amplified count)", gained)
	}
	if added := len(e.L.Events) - beforeEvents; added != 5 {
		t.Fatalf("log grew by %d events after one Submit, want exactly 5, not a 32x-amplified count", added)
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

// TestNoPendingDecisionWithAZeroLifePlayerNotYetLost is the fix-round-2
// regression test for the re-review's N1 finding: the outcome-keyed
// termination fix-round-1 shipped (destroyLethalDamage reporting "changed"
// only from what actually left the battlefield) can under-report when a
// replacement effect keeps the permanent in play but its substitute effect
// still changes SBA-relevant state -- here, draining the caster's own life
// instead of moving the shield. Because checkLoseConditions runs BEFORE
// destroyLethalDamage within one pass, a life change caused by
// destroyLethalDamage's own replacement is only visible to a SUBSEQUENT
// pass -- and reporting "nothing left the battlefield" as "nothing changed"
// denies it one. CR 704.5a: a player at 0 or less life must never be left
// un-Lost while the game keeps handing out decisions. Driven through
// repeated real Submits (the public path), checking the invariant after
// every single one -- not just at a specific submit count -- so this
// catches the violation regardless of which pass it would show up on.
func TestNoPendingDecisionWithAZeroLifePlayerNotYetLost(t *testing.T) {
	e := newSeats(t, 3)
	shield := onBoard(t, e, 0, `Name:Shield
ManaCost:1 G
Types:Creature Wall
PT:0/4
R:Event$ Moved | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | ReplaceWith$ RepDrain
SVar:RepDrain:DB$ LoseLife | Defined$ You | LifeAmount$ 1
Oracle:x
`)
	e.emit(events.Event{Kind: events.Damage, Obj: shield, Amount: 4})
	e.G.Players[0].Life = 4

	for i := 0; i < 8 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			break
		}
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("submit %d: no pass option: %+v", i, d.Options)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
			t.Fatalf("submit %d: %v", i, err)
		}
		if e.G.Players[0].Life <= 0 && !e.G.Players[0].Lost {
			t.Fatalf("after submit %d: player 0 is at life %d but Lost=false -- CR 704.5a is "+
				"outstanding on a board state the client can observe and be handed a decision over",
				i, e.G.Players[0].Life)
		}
	}
}

// TestNoPendingDecisionWithAZeroLifePlayerNotYetLostViaRemovalSweep is the
// fix-round-3 regression test: checkLoseConditions' removal sweep had the
// identical under-report shape as destroyLethalDamage (fix round 2's N1),
// one loop over -- "changed" came from whether a swept player's
// battlefield actually shrank, so a replacement that blocks the exile but
// changes SBA-relevant state elsewhere (another player's life total) was
// invisible to this sweep and denied the later pass that would have caught
// it.
//
// Ward, seat 1's own permanent, intercepts ANY Battlefield -> Exile move
// (ValidCard$ Card, not scoped to a controller) and substitutes a 1-life
// drain against Ward's controller's opponents -- which, with seat 0
// already eliminated, resolves to exactly seat 2 -- instead of letting
// seat 0's own permanent (Victim) actually leave. Seat 2 starts at low
// life so a couple of blocked sweep attempts reach 0. Driven through
// repeated real Submits, checking after every single one (not a fixed
// count) that no player sits at life <= 0 with Lost=false.
func TestNoPendingDecisionWithAZeroLifePlayerNotYetLostViaRemovalSweep(t *testing.T) {
	e := newSeats(t, 3)
	onBoard(t, e, 0, `Name:Victim
ManaCost:1
Types:Artifact
Oracle:x
`)
	onBoard(t, e, 1, `Name:Ward
ManaCost:1 W
Types:Artifact
R:Event$ Moved | Origin$ Battlefield | Destination$ Exile | ValidCard$ Card | ReplaceWith$ RepDrain | Description$ x
SVar:RepDrain:DB$ LoseLife | Defined$ Opponent | LifeAmount$ 1
Oracle:x
`)
	e.G.Players[0].Lost = true
	e.G.Players[2].Life = 2

	for i := 0; i < 8 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			break
		}
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("submit %d: no pass option: %+v", i, d.Options)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
			t.Fatalf("submit %d: %v", i, err)
		}
		if e.G.Players[2].Life <= 0 && !e.G.Players[2].Lost {
			t.Fatalf("after submit %d: player 2 is at life %d but Lost=false -- CR 704.5a is "+
				"outstanding on a board state the client can observe and be handed a decision over",
				i, e.G.Players[2].Life)
		}
	}
}

// TestLethalDamageIsRetriedWhenTheReplacementsControllerIsEliminatedMidCall
// is the fix-round-4 regression test for the re-review's N6 finding: the
// attempted-set discipline rounds 2 and 3 introduced (one attempt per
// object and per player per checkStateBased call) can under-COMPLETE, which
// is the mirror image of the under-report round 2 fixed.
//
// Whether a destruction attempt succeeds is decided by the replacement
// effects that apply to it, and applyReplacements only ever looks at
// objects controlled by a player who is still alive (trigger.go's
// forEachObject walks AliveFrom(0)). So a blocker stops blocking the
// instant its controller is eliminated -- and the blocked attempt's own
// substitute effect is routinely what eliminates them, as here: Doomed is
// a 1/1 carrying 4 damage with nothing of its own to save it, and Guardian
// (seat 2, two life) replaces every OTHER creature's death with "Guardian's
// controller loses 1 life". Two blocked attempts take seat 2 to 0, the
// next pass marks it Lost, and from that moment Guardian cannot replace
// anything at all -- but before this round Doomed was already in the
// attempted set, so nothing retried it and the decision went out over a
// board carrying a state-based action that nothing on it could prevent.
//
// The invariant is checked after every Submit rather than at a fixed count:
// once no living player controls anything that could replace Doomed's
// destruction (here, exactly "seat 2 is Lost"), Doomed must not still be
// sitting on the battlefield with lethal damage marked. Against bd3c730
// this fails on the very first Submit.
func TestLethalDamageIsRetriedWhenTheReplacementsControllerIsEliminatedMidCall(t *testing.T) {
	e := newSeats(t, 3)
	doomed := onBoard(t, e, 0, "Name:Doomed\nManaCost:G\nTypes:Creature Goat\nPT:1/1\nOracle:x\n")
	guardian := onBoard(t, e, 2, `Name:Guardian
ManaCost:2 W
Types:Creature Spirit
PT:2/2
R:Event$ Moved | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Creature.Other | ReplaceWith$ RepDrain | Description$ x
SVar:RepDrain:DB$ LoseLife | Defined$ You | LifeAmount$ 1
Oracle:x
`)
	e.emit(events.Event{Kind: events.Damage, Obj: doomed, Amount: 4})
	e.G.Players[2].Life = 2

	blocked := false
	for i := 0; i < 8 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			break
		}
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("submit %d: no pass option: %+v", i, d.Options)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
			t.Fatalf("submit %d: %v", i, err)
		}
		if e.G.Players[2].Life < 2 {
			blocked = true
		}
		if e.G.Players[2].Lost && e.G.Obj(doomed).Zone == state.ZBattlefield {
			t.Fatalf("after submit %d: Guardian's controller is eliminated, so nothing can replace "+
				"Doomed's destruction any more, yet Doomed is still on the battlefield with 4 damage "+
				"marked on a 1-toughness body -- CR 704.5g is outstanding on a board the client is "+
				"being handed a decision over", i)
		}
	}
	if !blocked {
		t.Fatal("Guardian never actually replaced anything: this scenario has to reach the " +
			"blocked-attempt path to be a regression test at all")
	}
	if e.G.Obj(doomed).Zone != state.ZGraveyard {
		t.Fatalf("doomed zone = %s, want graveyard", e.G.Obj(doomed).Zone)
	}
	if e.G.Obj(guardian).Zone == state.ZBattlefield {
		t.Fatalf("guardian zone = %s, want off the battlefield: its controller left the game",
			e.G.Obj(guardian).Zone)
	}
}

// TestRemovalSweepFiringsDoNotScaleWithAnUnrelatedDeathChain pins the bound
// fix round 3 established and fix round 4's re-arm had to preserve: a
// removal sweep that a replacement keeps blocking fires exactly ONCE per
// checkStateBased call -- twice per Submit, which makes two such calls (its
// own, then step's before it hands out the next decision) -- no matter how
// many passes something else drives the fixed-point loop through.
//
// Ward (seat 0, alive) turns any Battlefield -> Exile into a 1-life gain, so
// seat 0's life total counts blocked sweep attempts exactly. Victim belongs
// to seat 1, who is eliminated, so the sweep keeps trying to exile it. Seat
// 2 carries an N-link death chain (each link only becomes lethal once the
// previous one has died) that forces the loop through N passes with nothing
// to do with the sweep at all.
//
// This was measured but never committed as a test in fix round 3; it is
// committed now because fix round 4 makes a blocked attempt retryable, and
// the whole point of keying that re-arm on the alive-player count alone is
// that an unrelated death chain must not pay for one. Keying it on the
// wider "some other state-based action actually succeeded" instead gives
// 2/3/7/22/61 for these five chain lengths -- 29fa00d's own pre-round-3
// amplification, measured by building that variant, not assumed -- so this
// is the assertion that catches that mistake being made again.
func TestRemovalSweepFiringsDoNotScaleWithAnUnrelatedDeathChain(t *testing.T) {
	for _, chain := range []int{0, 1, 5, 20, 60} {
		e := newSeats(t, 3)
		onBoard(t, e, 0, `Name:Ward
ManaCost:1 W
Types:Artifact
R:Event$ Moved | Origin$ Battlefield | Destination$ Exile | ValidCard$ Card | ReplaceWith$ RepLife | Description$ x
SVar:RepLife:DB$ GainLife | Defined$ You | LifeAmount$ 1
Oracle:x
`)
		onBoard(t, e, 1, "Name:Victim\nManaCost:1\nTypes:Artifact\nOracle:x\n")
		e.G.Players[1].Life = 0

		ids := make([]state.ObjID, chain)
		for i := 0; i < chain; i++ {
			ids[i] = onBoard(t, e, 2, fmt.Sprintf(
				"Name:Link%d\nManaCost:1\nTypes:Creature Chain%d\nPT:1/1\nOracle:x\n", i, i))
			e.emit(events.Event{Kind: events.Damage, Obj: ids[i], Amount: 1})
		}
		for i := 1; i < chain; i++ {
			e.AddContinuous(ContinuousEffect{Source: ids[i-1], Timestamp: uint32(i), Layer: LPT,
				Sub: SubModify, Affects: fmt.Sprintf("Chain%d", i), Controller: 2, AddToughness: 2})
		}

		for submit := 0; submit < 3; submit++ {
			before := e.G.Players[0].Life
			d := e.Pending()
			if d == nil || d.Kind != decision.KPriority {
				t.Fatalf("chain=%d submit=%d: no priority decision", chain, submit)
			}
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("chain=%d submit=%d: no pass option: %+v", chain, submit, d.Options)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
				t.Fatalf("chain=%d submit=%d: %v", chain, submit, err)
			}
			if got := e.G.Players[0].Life - before; got != 2 {
				t.Fatalf("chain=%d submit=%d: blocked sweep fired %d times, want exactly 2 "+
					"(one per checkStateBased call) regardless of chain length -- a sweep that "+
					"scales with an unrelated death chain is the T22-h amplification returning",
					chain, submit, got)
			}
		}
	}
}

// newFixtureDeckWithTokens is newFixtureDeck (replacement_updated_test.go)
// plus Config.Tokens holding two token fixtures authored here -- never the
// GPL-3.0 corpus's own .cards/tokenscripts -- so a live Engine built from
// this Config has something in Game.Tokens for a TokenCreate event to mint.
// Duplicated rather than threading a Tokens parameter through
// newFixtureDeck itself: that helper is shared by every Task 26/29 test in
// this package, and none of them needs Tokens, so widening its signature
// for this file alone would touch a file Task 13 has no reason to modify.
func newFixtureDeckWithTokens(t *testing.T, seed uint64, fixtureSrc string) (*Engine, Config, state.ObjID) {
	t.Helper()
	fixture := card(t, fixtureSrc)
	name := fixture.Faces[0].Name
	cfg := Config{Seed: seed, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{fixture}, mountainDeck(t, 39)...),
			mountainDeck(t, 40),
		},
		Tokens: map[string]*cards.Card{
			"r_1_1_goblin":                      card(t, "Name:Goblin Token\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n"),
			"c_3_3_a_phyrexian_wurm_deathtouch": card(t, "Name:Phyrexian Wurm Token\nTypes:Creature Phyrexian Wurm\nPT:3/3\nK:Deathtouch\nOracle:x\n"),
		},
	}
	e := New(cfg)
	e.Advance()

	var id state.ObjID
	for _, cand := range e.G.Zone(state.ZHand, 0) {
		if e.G.Obj(cand).Face().Name == name {
			id = cand
		}
	}
	if id == 0 {
		for _, cand := range e.G.Zone(state.ZLibrary, 0) {
			if e.G.Obj(cand).Face().Name == name {
				id = cand
			}
		}
		if id == 0 {
			t.Fatalf("fixture %q not found in seat 0's hand or library", name)
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZHand})
	}
	return e, cfg, id
}

// TestATokenThatDiesCeasesToExist is CR 111.7: a token that leaves the
// battlefield ceases to exist, which this build models by parking it in
// exile (state.Object.Ephemeral's own doc) rather than by deleting the
// object outright. Damaging the token lethally first sends it to the
// graveyard via destroyLethalDamage's own CR 704.5g move -- the SAME
// checkStateBased call's exileDeadTokens pass then finds it off the
// battlefield and relocates it to exile, so the two zones a naive
// implementation might leave it in (graveyard, because that is where
// ordinary lethal damage sends a creature) are both checked: it must NOT
// still be in the graveyard, and it must be in exile.
func TestATokenThatDiesCeasesToExist(t *testing.T) {
	e, _, _ := newFixtureDeckWithTokens(t, 51, "Name:Pyro\nManaCost:1 R\nTypes:Creature Human Shaman\nPT:2/1\nOracle:x\n")
	e.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "r_1_1_goblin"})
	tok := e.G.Zone(state.ZBattlefield, 0)[len(e.G.Zone(state.ZBattlefield, 0))-1]
	e.emit(events.Event{Kind: events.Damage, Obj: tok, Amount: 1})
	e.checkStateBased()
	o := e.G.Obj(tok)
	if o.Zone != state.ZExile || len(e.G.Zone(state.ZGraveyard, 0)) != 0 {
		t.Fatalf("dead token in %s; graveyard %v", o.Zone, e.G.Zone(state.ZGraveyard, 0))
	}
	n := len(e.L.Events)
	e.checkStateBased()
	if len(e.L.Events) != n {
		t.Fatal("an exiled token keeps being re-exiled")
	}
}

// TestATokenBouncedToHandCeasesToExist covers a zone CR 111.7 applies to
// besides "died from the battlefield, via the graveyard, like an ordinary
// creature" (TestATokenThatDiesCeasesToExist above): a token returned to
// hand (bounced, the same as any "return to hand" effect might do) ceases
// to exist there just the same, exiled directly rather than lingering as a
// hand card. The rest of seat 0's real opening hand is left in the assert
// on purpose -- the check is that this specific token id is gone from
// wherever it landed, not that the whole hand emptied out.
func TestATokenBouncedToHandCeasesToExist(t *testing.T) {
	e, _, _ := newFixtureDeckWithTokens(t, 52, "Name:Pyro\nManaCost:1 R\nTypes:Creature Human Shaman\nPT:2/1\nOracle:x\n")
	e.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "r_1_1_goblin"})
	bf := e.G.Zone(state.ZBattlefield, 0)
	tok := bf[len(bf)-1]
	e.emit(events.Event{Kind: events.MoveZone, Obj: tok, From: state.ZBattlefield, To: state.ZHand})

	e.checkStateBased()

	if got := e.G.Obj(tok).Zone; got != state.ZExile {
		t.Fatalf("bounced token zone = %s, want exile", got)
	}
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if id == tok {
			t.Fatalf("hand = %v, still holds the token (it should have ceased to exist rather than "+
				"staying put)", e.G.Zone(state.ZHand, 0))
		}
	}
}

// TestATokenOnTheStackIsNotPrematurelyExiled: exileDeadTokens' zone
// exclusion list names the stack alongside the battlefield deliberately --
// a token copy of a spell or activated ability legitimately sits on the
// stack without being a permanent yet (Ephemeral's own IsCopy half covers
// that shape already; this is the token half of the same "not every
// off-battlefield placement is death" principle). Exercised directly
// against a manufactured stack object rather than a real copy effect (Task
// 13 does not implement CopySpellAbility) -- what exileDeadTokens reads is
// only IsToken and Zone, so a token object placed on the stack by hand is
// exactly as much of a test of the exclusion as a real spell copy would be.
func TestATokenOnTheStackIsNotPrematurelyExiled(t *testing.T) {
	e := newSeats(t, 2)
	goblin := card(t, "Name:Goblin Token\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n")
	o := e.G.AddObject(goblin, 0)
	o.IsToken = true
	events.Move(e.G, o.ID, state.ZLibrary, state.ZStack)

	e.checkStateBased()

	if got := e.G.Obj(o.ID).Zone; got != state.ZStack {
		t.Fatalf("token on the stack zone = %s, want stack (unchanged)", got)
	}
}

// TestExileDeadTokensDoesNotAmplifyWhenAReplacementBlocksTheMove is the
// regression test for Task 13 fix round 1's tried.tokens addition (review
// finding "minor 1"): Ward intercepts any move out of a graveyard and
// substitutes a 1-life gain instead, permanently keeping a dead token in
// the graveyard rather than letting it reach exile. Before tried.tokens,
// exileDeadTokens had no memory of the attempt and rediscovered the same
// token as a fresh candidate on every one of the 32 passes in the budget --
// spending the whole thing, and firing Ward's own replacement 32 times, on
// a single checkStateBased call. After, it is attempted exactly once per
// call, the same bound checkLoseConditions' removal sweep and
// destroyLethalDamage already hold for their own blocked attempts
// (TestDestroyLethalDamageDoesNotAmplifyWhenAReplacementKeepsThePermanent
// and TestRemovePermanentsDoesNotAmplifyWhenAReplacementKeepsThePermanent
// above are the same shape for their own passes).
func TestExileDeadTokensDoesNotAmplifyWhenAReplacementBlocksTheMove(t *testing.T) {
	e := newSeats(t, 2)
	onBoard(t, e, 0, `Name:Ward
ManaCost:1 W
Types:Artifact
R:Event$ Moved | Origin$ Graveyard | ValidCard$ Card | ReplaceWith$ RepLife | Description$ x
SVar:RepLife:DB$ GainLife | Defined$ You | LifeAmount$ 1
Oracle:x
`)
	goblin := card(t, "Name:Goblin Token\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n")
	o := e.G.AddObject(goblin, 0)
	o.IsToken = true
	events.Move(e.G, o.ID, state.ZLibrary, state.ZGraveyard)

	beforeLife := e.G.Players[0].Life
	beforeEvents := len(e.L.Events)

	e.checkStateBased()

	if got := e.G.Obj(o.ID).Zone; got != state.ZGraveyard {
		t.Fatalf("token zone = %s, want graveyard (Ward's replacement keeps it there)", got)
	}
	if gained := e.G.Players[0].Life - beforeLife; gained != 1 {
		t.Fatalf("life gained = %d, want exactly 1 (one exile attempt per checkStateBased call, "+
			"not the full 32-pass budget spent 32 times over)", gained)
	}
	if added := len(e.L.Events) - beforeEvents; added != 1 {
		t.Fatalf("log grew by %d events, want exactly 1 (the single blocked attempt's own "+
			"replacement-substituted event), not a 32x-amplified count", added)
	}
}
