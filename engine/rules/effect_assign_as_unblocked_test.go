package rules

// cr-g157: an Effect-delivered Mode$ AssignCombatDamageAsUnblocked (a DB$/
// SP$ Effect's StaticAbilities$ body) registers as a continuous restriction
// and reaches the ordinary as-unblocked election/assignment path
// (engine/rules/combat.go asUnblockedNeeding), beside the printed S: static
// route asUnblockedStaticMatches already walks. Predatory Focus (Optional$
// True, Creature.YouCtrl), Outmaneuver (mandatory, Card.IsRemembered over its
// X targeted blocked creatures) and Gurzigost are the corpus carriers.
//
// Every leaf pins a REAL corpus card, drives the ordinary cast/declare
// paths, asserts its preconditions, and ends replay-verified.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// boardObj finds seat p's battlefield permanent named name.
func boardObj(t *testing.T, e *Engine, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	t.Fatalf("seat %d's battlefield has no %q", p, name)
	return 0
}

// submitBlockerPairsNoDrain answers the pending blockers decision with the
// named (attacker, blocker) pairs WITHOUT crossing the CR 509.2 priority
// window that follows, so the caller can act in it (Outmaneuver is cast
// there, with the blockers already declared).
func submitBlockerPairsNoDrain(t *testing.T, e *Engine, pairs ...[2]state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("expected a blockers decision, got %+v", d)
	}
	var choices []int
	for _, pr := range pairs {
		idx := -1
		for _, o := range d.Options {
			if o.Obj == pr[1] && o.Attacker == pr[0] {
				idx = o.Index
				break
			}
		}
		if idx < 0 {
			t.Fatalf("no block option pairing attacker %d with blocker %d: %+v", pr[0], pr[1], d.Options)
		}
		choices = append(choices, idx)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
		t.Fatalf("submit blocker pairs: %v", err)
	}
}

// electionChooseAskAfter reports whether a choose ask (the KChoose
// election's DecisionAsk) was posed to player p after marker: the broad
// DecisionAsk scan electionAskedAfter uses would also match the CR 509.2
// priority ask the blocker declaration drains through, which is not an
// election.
func electionChooseAskAfter(t *testing.T, e *Engine, marker int, player state.PlayerID) bool {
	t.Helper()
	for _, ev := range e.L.Events[marker:] {
		if ev.Kind == events.DecisionAsk && ev.Player == player && ev.Text == "choose" {
			return true
		}
	}
	return false
}

// driveThroughOpponentTurn answers every decision the intervening opponent
// turn poses -- pass priority, empty attackers/blockers declarations -- until
// seat 0's turn-th declare-attackers ask is pending (the Effect's grant is
// UntilEOT, so the next seat-0 turn is where its expiry is observable).
func driveThroughOpponentTurn(t *testing.T, e *Engine, turn int32) {
	t.Helper()
	for i := 0; i < 4000; i++ {
		if e.G.Turn == turn && e.G.Active == 0 && e.G.Step == state.StepDeclareAttackers {
			return
		}
		if e.G.Over {
			t.Fatalf("game ended before turn %d (at turn %d seat %d step %v)", turn, e.G.Turn, e.G.Active, e.G.Step)
		}
		d := e.Pending()
		if d == nil {
			t.Fatalf("no pending decision at turn %d seat %d step %v", e.G.Turn, e.G.Active, e.G.Step)
		}
		switch d.Kind {
		case decision.KPriority:
			idx := castOptionIdx(d, "pass", 0)
			if idx < 0 {
				t.Fatalf("priority decision with no pass option: %+v", d)
			}
			submitChoices(t, e, idx)
		case decision.KAttackers, decision.KBlockers:
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player}); err != nil {
				t.Fatalf("empty declaration: %v", err)
			}
		default:
			t.Fatalf("unexpected %v decision during the drive: %+v", d.Kind, d)
		}
	}
	t.Fatal("drive budget exhausted")
}

func TestEffectAsUnblockedPredatoryFocus(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	focus := lookup(t, reg, "Predatory Focus")
	atkCard := card(t, "Name:Brute\nManaCost:3 G\nTypes:Creature Beast\nPT:4/4\nOracle:x\n")
	blkCard := card(t, "Name:Pup\nManaCost:1 W\nTypes:Creature Dog\nPT:1/1\nOracle:x\n")
	e, cfg := restrictionGame(t, 9317,
		[][]*cards.Card{{focus}, nil},
		[][]*cards.Card{{atkCard}, {blkCard}})
	atk := boardObj(t, e, 0, "Brute")
	blk := boardObj(t, e, 1, "Pup")
	focusID := moveByName(t, e, 0, "Predatory Focus", state.ZHand)

	if pw := e.Power(atk); pw != 4 {
		t.Fatalf("precondition: attacker power = %d, want 4", pw)
	}
	if tg := e.Toughness(blk); tg != 1 {
		t.Fatalf("precondition: blocker toughness = %d, want 1", tg)
	}
	if e.G.Obj(atk).SummonSick {
		t.Fatal("precondition: attacker is summoning sick")
	}

	addMana(t, e, 0, "GGGGG")
	d := e.Pending()
	idx := castOptionIdx(d, "cast", focusID)
	if idx < 0 {
		t.Fatalf("precondition: Predatory Focus is not offered for cast: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 20)
	if hasNote(e, "AssignCombatDamageAsUnblocked unimplemented") {
		t.Fatal("the readable Effect-delivered as-unblocked static was not registered")
	}

	driveToStep(t, e, e.G.Turn, 0, state.StepDeclareAttackers)
	submitAttackers(t, e, atk)
	marker := len(e.L.Events)
	submitBlockerPairs(t, e, [2]state.ObjID{atk, blk})

	if !electionChooseAskAfter(t, e, marker, 0) {
		t.Fatalf("no as-unblocked election was asked (step %s, pending %+v)", e.G.Step, e.Pending())
	}
	answerElection(t, e, true)

	if got := e.G.Players[1].Life; got != 16 {
		t.Fatalf("defending player life = %d, want 16 (20 - the 4/4's power)", got)
	}
	if got := e.G.Obj(blk).Damage; got != 0 {
		t.Fatalf("blocker damage = %d, want 0", got)
	}
	if z := e.G.Obj(blk).Zone; z != state.ZBattlefield {
		t.Fatalf("blocker zone = %v, want still on the battlefield", z)
	}
	if got := e.G.Obj(atk).Damage; got != 1 {
		t.Fatalf("attacker damage = %d, want 1 (the blocker hit back)", got)
	}
	replayCheck(t, e, cfg)
}

func TestEffectAsUnblockedDeclined(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	focus := lookup(t, reg, "Predatory Focus")
	atkCard := card(t, "Name:Brute\nManaCost:3 G\nTypes:Creature Beast\nPT:4/4\nOracle:x\n")
	blkCard := card(t, "Name:Pup\nManaCost:1 W\nTypes:Creature Dog\nPT:1/1\nOracle:x\n")
	e, cfg := restrictionGame(t, 9319,
		[][]*cards.Card{{focus}, nil},
		[][]*cards.Card{{atkCard}, {blkCard}})
	atk := boardObj(t, e, 0, "Brute")
	blk := boardObj(t, e, 1, "Pup")
	focusID := moveByName(t, e, 0, "Predatory Focus", state.ZHand)

	if pw := e.Power(atk); pw != 4 {
		t.Fatalf("precondition: attacker power = %d, want 4", pw)
	}
	if tg := e.Toughness(blk); tg != 1 {
		t.Fatalf("precondition: blocker toughness = %d, want 1", tg)
	}

	addMana(t, e, 0, "GGGGG")
	d := e.Pending()
	idx := castOptionIdx(d, "cast", focusID)
	if idx < 0 {
		t.Fatalf("precondition: Predatory Focus is not offered for cast: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 20)

	driveToStep(t, e, e.G.Turn, 0, state.StepDeclareAttackers)
	submitAttackers(t, e, atk)
	submitBlockerPairs(t, e, [2]state.ObjID{atk, blk})

	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != 2 {
		t.Fatalf("expected the as-unblocked election KChoose, got %+v", d)
	}
	answerElection(t, e, false)

	if got := e.G.Players[1].Life; got != 20 {
		t.Fatalf("defending player life = %d, want 20 (a declined election is not Trample)", got)
	}
	if z := e.G.Obj(blk).Zone; z != state.ZGraveyard {
		t.Fatalf("blocker zone = %v, want graveyard (the declined election assigned the 4/4's power to it)", z)
	}
	if got := e.G.Obj(atk).Damage; got != 1 {
		t.Fatalf("attacker damage = %d, want 1 (the blocker hit back)", got)
	}
	replayCheck(t, e, cfg)
}

func TestEffectAsUnblockedOutmaneuverMandatory(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	out := lookup(t, reg, "Outmaneuver")
	atk1Card := card(t, "Name:Brute\nManaCost:3 G\nTypes:Creature Beast\nPT:4/4\nOracle:x\n")
	atk2Card := card(t, "Name:Pal\nManaCost:3 G\nTypes:Creature Beast\nPT:4/4\nOracle:x\n")
	blk1Card := card(t, "Name:Pup\nManaCost:1 W\nTypes:Creature Dog\nPT:1/1\nOracle:x\n")
	blk2Card := card(t, "Name:Runt\nManaCost:1 W\nTypes:Creature Dog\nPT:1/1\nOracle:x\n")
	e, cfg := restrictionGame(t, 9329,
		[][]*cards.Card{{out}, nil},
		[][]*cards.Card{{atk1Card, atk2Card}, {blk1Card, blk2Card}})
	atk1 := boardObj(t, e, 0, "Brute")
	atk2 := boardObj(t, e, 0, "Pal")
	blk1 := boardObj(t, e, 1, "Pup")
	blk2 := boardObj(t, e, 1, "Runt")
	outID := moveByName(t, e, 0, "Outmaneuver", state.ZHand)

	if pw := e.Power(atk1); pw != 4 || e.Power(atk2) != 4 {
		t.Fatalf("precondition: attacker powers = %d/%d, want 4/4", e.Power(atk1), e.Power(atk2))
	}
	if e.Toughness(blk1) != 1 || e.Toughness(blk2) != 1 {
		t.Fatal("precondition: blockers are not 1/1s")
	}

	// CR 500.4 empties every pool at each step change, so the X = 1 + {R}
	// the cast needs is added INSIDE the CR 509.2 priority window, the only
	// instant-speed window where the spell's blocked-creature targets exist.
	driveToStep(t, e, e.G.Turn, 0, state.StepDeclareAttackers)
	submitAttackers(t, e, atk1, atk2)
	submitBlockerPairsNoDrain(t, e, [2]state.ObjID{atk1, blk1}, [2]state.ObjID{atk2, blk2})
	for _, r := range "RR" {
		e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: string(r), Amount: 1})
	}
	e.priorityRound()

	// CR 509.2's priority window: Outmaneuver (instant, X blocked creatures
	// as targets) is cast here, with both attackers blocked.
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("expected the CR 509.2 priority window, got %+v", d)
	}
	idx := castOptionIdx(d, "cast", outID)
	if idx < 0 {
		t.Fatalf("precondition: Outmaneuver is not offered in the 509.2 window: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	d = e.Pending()
	if d != nil && d.Kind == decision.KChoose {
		xIdx := -1
		for _, o := range d.Options {
			if o.Kind == "x" && o.Label == "X = 1" {
				xIdx = o.Index
			}
		}
		if xIdx < 0 {
			t.Fatalf("no X = 1 option offered: %+v", d.Options)
		}
		submitChoices(t, e, xIdx)
	}
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected Outmaneuver's target ask, got %+v", d)
	}
	tIdx := -1
	for _, o := range d.Options {
		if o.Obj == atk1 {
			tIdx = o.Index
		}
	}
	if tIdx < 0 {
		t.Fatalf("Outmaneuver did not offer the blocked Brute: %+v", d.Options)
	}
	submitChoices(t, e, tIdx)
	passUntilStackEmpty(t, e, 20)
	// passUntilStackEmpty leaves the CR 509.2 window still open (the stack
	// drained, priority continues), and a mandatory match auto-accepts with
	// no ask, so the window's remaining priority passes carry the pass's
	// damage. Pass until the damage lands, bounded.
	for i := 0; i < 20; i++ {
		d = e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			break
		}
		if e.G.Players[1].Life != 20 {
			break
		}
		submitChoices(t, e, castOptionIdx(d, "pass", 0))
	}

	// Mandatory (no Optional$): no election was asked and the targeted
	// attacker's whole power went to the defending player; the second
	// attacker assigned normally to its blocker.
	if got := e.G.Players[1].Life; got != 16 {
		t.Fatalf("defending player life = %d, want 16 (20 - the Outmaneuver target's 4)", got)
	}
	if got := e.G.Obj(blk1).Damage; got != 0 {
		t.Fatalf("targeted attacker's blocker damage = %d, want 0", got)
	}
	if z := e.G.Obj(blk1).Zone; z != state.ZBattlefield {
		t.Fatalf("targeted attacker's blocker zone = %v, want alive", z)
	}
	if z := e.G.Obj(blk2).Zone; z != state.ZGraveyard {
		t.Fatalf("second blocker zone = %v, want graveyard (the untouched attacker assigned normally)", z)
	}
	if got := e.G.Obj(atk1).Damage; got != 1 {
		t.Fatalf("targeted attacker damage = %d, want 1 (its blocker hit back)", got)
	}
	replayCheck(t, e, cfg)
}

func TestEffectAsUnblockedExpires(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	focus := lookup(t, reg, "Predatory Focus")
	atkCard := card(t, "Name:Brute\nManaCost:3 G\nTypes:Creature Beast\nPT:4/4\nOracle:x\n")
	blkCard := card(t, "Name:Pup\nManaCost:1 W\nTypes:Creature Dog\nPT:1/1\nOracle:x\n")
	e, cfg := restrictionGame(t, 9341,
		[][]*cards.Card{{focus}, nil},
		[][]*cards.Card{{atkCard}, {blkCard}})
	atk := boardObj(t, e, 0, "Brute")
	blk := boardObj(t, e, 1, "Pup")
	focusID := moveByName(t, e, 0, "Predatory Focus", state.ZHand)

	if e.Power(atk) != 4 || e.Toughness(blk) != 1 {
		t.Fatal("precondition: expected a 4/4 attacker and a 1/1 blocker")
	}

	addMana(t, e, 0, "GGGGG")
	d := e.Pending()
	idx := castOptionIdx(d, "cast", focusID)
	if idx < 0 {
		t.Fatalf("precondition: Predatory Focus is not offered for cast: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 20)

	driveToStep(t, e, e.G.Turn, 0, state.StepDeclareAttackers)
	submitAttackers(t, e, atk)
	submitBlockerPairs(t, e, [2]state.ObjID{atk, blk})
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected the as-unblocked election KChoose, got %+v", d)
	}
	answerElection(t, e, true)
	if got := e.G.Players[1].Life; got != 16 {
		t.Fatalf("defending player life = %d, want 16 after the accepted election", got)
	}

	// The grant is UntilEOT: seat 0's NEXT turn, the same attack assigns to
	// the blocker and no election is asked.
	next := e.G.Turn + 2
	driveThroughOpponentTurn(t, e, next)
	submitAttackers(t, e, atk)
	marker := len(e.L.Events)
	submitBlockers(t, e, blk)
	if electionChooseAskAfter(t, e, marker, 0) {
		t.Fatal("an election was asked after the Effect's static expired")
	}
	if z := e.G.Obj(blk).Zone; z != state.ZGraveyard {
		t.Fatalf("blocker zone = %v, want graveyard (the expired static assigns normally)", z)
	}
	if got := e.G.Players[1].Life; got != 16 {
		t.Fatalf("defending player life = %d, want 16", got)
	}
	replayCheck(t, e, cfg)
}
