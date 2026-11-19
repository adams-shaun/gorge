package rules

// The behaviour half of the EDH deck-blocker API class: each api:* the
// coverage test (cmd/forgec/edh_apis_test.go) registers plays its printed
// text through a real game here -- legal actions, cast or activate, resolve,
// then assert on state. One test per mechanism.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// answerWithBid drives a BidLife resolution: priority passes; a bid ask is
// answered with the topping player's bid when tops names one, else the first
// (pass) option; every other decision takes its first Min options. Bounded.
func answerWithBid(t *testing.T, e *Engine, tops map[string]int) {
	t.Helper()
	for i := 0; i < 300 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			return
		}
		switch {
		case d.Kind == decision.KPriority:
			passOnce(t, e)
		case d.Kind == decision.KChoose && d.ResumeKind == "bid_top" && d.Player == state.PlayerID(tops["player"]) && tops["bid"] > 0:
			idx := -1
			for _, o := range d.Options {
				if o.Amount == tops["bid"] {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("no bid-%d option for seat %d: %+v", tops["bid"], d.Player, d.Options)
			}
			submitChoices(t, e, idx)
			tops["bid"] = 0 // one top only; later asks pass
		default:
			ch := []int{}
			for k := 0; k < d.Min && k < len(d.Options); k++ {
				ch = append(ch, d.Options[k].Index)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: ch}); err != nil {
				t.Fatalf("submit %+v: %v", d, err)
			}
		}
	}
	if !e.G.Over && len(e.G.Stack) != 0 {
		t.Fatalf("the game did not settle (stack %d)", len(e.G.Stack))
	}
}

// countControlledBattlefield names one face's battlefield objects p controls.
func countControlledBattlefield(e *Engine, p state.PlayerID, name string) int {
	n := 0
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name && o.Controller == p {
			n++
		}
	}
	return n
}

// TestEDHAPIsBalance is api:Balance's leaf (real corpus Magus of the
// Balance): the activated Balance keeps the fewest count in every zone it
// runs -- seat 0's two Mountains and seat 1's four both end at two, seat 1
// sacrificing the two it kept nothing of -- and the Zone$ Hand leg discards
// the larger hand down to the smaller one.
func TestEDHAPIsBalance(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	mountain := lookup(t, reg, "Mountain")
	e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Magus of the Balance")}, nil)
	magus := moveNamedTo(t, e, 0, "Magus of the Balance", state.ZHand)
	for i := 0; i < 2; i++ {
		onBoardCard(t, e, 0, mountain)
	}
	for i := 0; i < 4; i++ {
		onBoardCard(t, e, 1, mountain)
	}
	// The SubAbility$ chain's third leg (BalanceCreatures, Valid$ Creature):
	// seat 0 keeps one Bear, seat 1 holds two, so the creatures leg is not
	// vacuous either. The Magus itself is sacrificed by the activation cost
	// before resolution, so it never counts.
	const bear = "Name:Test Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	bear0 := onBoard(t, e, 0, bear)
	bear1 := onBoard(t, e, 1, bear)
	bear2 := onBoard(t, e, 1, bear)
	for _, id := range []state.ObjID{bear0, bear1, bear2} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: creature %d is not on the battlefield", id)
		}
	}
	// Precondition: the hands differ, so the Zone$ Hand leg is not vacuous.
	hand0, hand1 := len(e.G.Zone(state.ZHand, 0)), len(e.G.Zone(state.ZHand, 1))
	if hand0 == hand1 {
		t.Fatalf("precondition: equal opening hands (%d) make the hand leg vacuous", hand0)
	}
	// Cast the Magus first ({1}{W}); the Balance is its {4}{W}, {T},
	// Sacrifice-activation on the permanent, and the {T} cost needs the
	// creature past its summoning-sick turn (CR 302.6), so seat 0 takes its
	// next turn before activating.
	addMana(t, e, 0, "WW")
	submitChoices(t, e, castOptionFor(t, e, magus).Index)
	answerQuiet(t, e, 200)
	magus = findNamedInZone(e, 0, state.ZBattlefield, "Magus of the Balance")
	// driveToStepAll: seat 1's creatures now ask for an attackers decision on
	// its own turn; the drive declares none of them.
	driveToStepAll(t, e, 3, 0, state.StepMain1)
	if magus == 0 {
		t.Fatal("the Magus never entered the battlefield")
	}
	// Five white mana, one tap, the Magus itself.
	addMana(t, e, 0, "WWWWW")
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == magus {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no ability option for the Magus: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	answerQuiet(t, e, 400)

	if got := countControlledBattlefield(e, 0, "Mountain"); got != 2 {
		t.Errorf("seat 0 controls %d Mountains, want the fewest (2)", got)
	}
	if got := countControlledBattlefield(e, 1, "Mountain"); got != 2 {
		t.Errorf("seat 1 controls %d Mountains, want the fewest (2)", got)
	}
	if len(e.G.Zone(state.ZHand, 0)) != hand1 || len(e.G.Zone(state.ZHand, 1)) != hand1 {
		t.Errorf("hands after Balance = %d/%d, want both at the fewest (%d)",
			len(e.G.Zone(state.ZHand, 0)), len(e.G.Zone(state.ZHand, 1)), hand1)
	}
	if got := countControlledBattlefield(e, 0, "Test Bear"); got != 1 {
		t.Errorf("seat 0 controls %d Bears, want the fewest (1)", got)
	}
	if got := countControlledBattlefield(e, 1, "Test Bear"); got != 1 {
		t.Errorf("seat 1 controls %d Bears, want the fewest (1) -- the second was sacrificed", got)
	}
	if findNamedInZone(e, 0, state.ZGraveyard, "Magus of the Balance") == 0 {
		t.Error("the Magus activation cost did not sacrifice the Magus")
	}
}

// TestEDHAPIsBidLife is api:BidLife's leaf (real corpus Illicit Auction):
// seat 0's opening bid of 0 is topped by seat 1's bid of 3, seat 0 passes,
// and the high bidder pays the 3 life and gains control of the creature.
func TestEDHAPIsBidLife(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Illicit Auction")}, nil)
	moveNamedTo(t, e, 0, "Illicit Auction", state.ZHand)
	bear := onBoard(t, e, 0, "Name:Blocking Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	addMana(t, e, 0, "RRRRR")
	illicit := findNamedInZone(e, 0, state.ZHand, "Illicit Auction")
	submitChoices(t, e, castOptionFor(t, e, illicit).Index)
	// The target ask: the bear.
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Obj == bear {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("the bear was not offered as a target: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	answerWithBid(t, e, map[string]int{"player": 1, "bid": 3})
	if got := e.G.Players[1].Life; got != 17 {
		t.Errorf("seat 1's life = %d, want 17 (paid the winning bid of 3)", got)
	}
	if o := e.G.Obj(bear); o.Controller != 1 {
		t.Errorf("the bear's controller = %d, want the high bidder (seat 1)", o.Controller)
	}
	if got := e.G.Players[0].Life; got != 20 {
		t.Errorf("seat 0's life = %d, want 20 (it passed and pays nothing)", got)
	}
}

// TestEDHAPIsBidLifeCounter is Mages' Contest's bid variant: only the caster
// and the targeted spell's controller bid, the opening bid is 1, and the
// caster winning counters the spell (ConditionPlayerDefined$ Remembered |
// ConditionPlayerContains$ You).
func TestEDHAPIsBidLifeCounter(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg,
		[]*cards.Card{lookup(t, reg, "Mages' Contest")},
		[]*cards.Card{lookup(t, reg, "Ornithopter")})
	driveToStep(t, e, 2, 1, state.StepMain1)
	// Seat 1 casts its free Ornithopter, then passes so seat 0 can respond.
	orn := moveNamedTo(t, e, 1, "Ornithopter", state.ZHand)
	submitChoices(t, e, castOptionFor(t, e, orn).Index)
	passOnce(t, e)
	// Seat 0 answers with Mages' Contest targeting the Ornithopter spell.
	mages := moveNamedTo(t, e, 0, "Mages' Contest", state.ZHand)
	addMana(t, e, 0, "RRR")
	submitChoices(t, e, castOptionFor(t, e, mages).Index)
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Obj != 0 && o.Obj == orn {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("the Ornithopter spell was not offered as a target: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	answerWithBid(t, e, nil) // seat 1 passes; the opening bid of 1 stands
	if got := e.G.Players[0].Life; got != 19 {
		t.Errorf("seat 0's life = %d, want 19 (it won the bidding at 1)", got)
	}
	if findNamedInZone(e, 1, state.ZGraveyard, "Ornithopter") == 0 {
		t.Error("the winning caster did not counter the targeted spell")
	}
	if got := countControlledBattlefield(e, 1, "Ornithopter"); got != 0 {
		t.Errorf("the countered Ornithopter is on the battlefield")
	}

	// The caster-loses branch: the bidder is the targeted spell's controller
	// (seat 1), not the caster, so ConditionPlayerContains$ You reads false
	// and Mages' Contest resolves without countering. Seat 1 tops the
	// opening bid at 2, seat 0 passes, seat 1 pays the 2 and keeps the spell.
	e2 := corpusEngine(t, reg,
		[]*cards.Card{lookup(t, reg, "Mages' Contest")},
		[]*cards.Card{lookup(t, reg, "Ornithopter")})
	driveToStep(t, e2, 2, 1, state.StepMain1)
	orn2 := moveNamedTo(t, e2, 1, "Ornithopter", state.ZHand)
	submitChoices(t, e2, castOptionFor(t, e2, orn2).Index)
	passOnce(t, e2)
	mages2 := moveNamedTo(t, e2, 0, "Mages' Contest", state.ZHand)
	addMana(t, e2, 0, "RRR")
	submitChoices(t, e2, castOptionFor(t, e2, mages2).Index)
	d2 := e2.Pending()
	idx2 := -1
	for _, o := range d2.Options {
		if o.Obj != 0 && o.Obj == orn2 {
			idx2 = o.Index
		}
	}
	if idx2 < 0 {
		t.Fatalf("the Ornithopter spell was not offered as a target: %+v", d2.Options)
	}
	submitChoices(t, e2, idx2)
	answerWithBid(t, e2, map[string]int{"player": 1, "bid": 2})
	if got := e2.G.Players[1].Life; got != 18 {
		t.Errorf("seat 1's life = %d, want 18 (it topped the bid at 2 and paid it)", got)
	}
	if got := e2.G.Players[0].Life; got != 20 {
		t.Errorf("seat 0's life = %d, want 20 (it passed and pays nothing)", got)
	}
	if got := countControlledBattlefield(e2, 1, "Ornithopter"); got != 1 {
		t.Errorf("seat 1 controls %d Ornithopters, want the resolved spell's 1", got)
	}
	if findNamedInZone(e2, 1, state.ZGraveyard, "Ornithopter") != 0 {
		t.Error("the spell resolved but was still put in the graveyard")
	}
}

// TestEDHAPIsReverseTurnOrder is Aeon Engine's leaf: the activation toggles
// the game's turn order, and the NEXT turn after seat 0's goes to seat 2,
// not seat 1 -- the reversed NextAlive the turn advance reads.
func TestEDHAPIsReverseTurnOrder(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngineThree(t, reg, []*cards.Card{lookup(t, reg, "Aeon Engine")}, nil, nil)
	moveNamedTo(t, e, 0, "Aeon Engine", state.ZHand)
	addMana(t, e, 0, "CCCCC")
	aeon := findNamedInZone(e, 0, state.ZHand, "Aeon Engine")
	submitChoices(t, e, castOptionFor(t, e, aeon).Index)
	answerQuiet(t, e, 400)
	aeon = findNamedInZone(e, 0, state.ZBattlefield, "Aeon Engine")
	if aeon == 0 {
		t.Fatal("Aeon Engine never entered the battlefield")
	}
	// It enters tapped (its printed replacement); the untap it needs is the
	// one seat 0's own next turn gives, so drive to turn 4 and activate.
	driveToStep(t, e, 4, 0, state.StepMain1)
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == aeon {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no ability option for Aeon Engine: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	answerQuiet(t, e, 400)
	if !e.G.ReverseTurnOrder {
		t.Fatal("the activation did not reverse the game's turn order")
	}
	if findNamedInZone(e, 0, state.ZExile, "Aeon Engine") == 0 {
		t.Error("the activation cost did not exile Aeon Engine")
	}
	// The turn advance follows the reversal: after seat 0's turn, seat 2
	// (the seat BEFORE seat 0 in seat order) is next. driveToStep fails if
	// the unreversed order (seat 1) is what the engine takes instead.
	driveToStep(t, e, 5, 2, state.StepMain1)
	if e.G.Active != 2 {
		t.Fatalf("the turn after seat 0's belongs to seat %d, want the reversed seat 2", e.G.Active)
	}
}

// TestEDHAPIsGameDrawn is Celestial Convergence's leaf: after the seventh
// omen counter leaves at its controller's seventh upkeep, the tied highest
// life totals (20/20) make the game a draw, the count:
// PlayerCountPlayers$TiedForHighestLife gate reading 2.
func TestEDHAPIsGameDrawn(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Celestial Convergence")}, nil)
	moveNamedTo(t, e, 0, "Celestial Convergence", state.ZHand)
	addMana(t, e, 0, "WWWW")
	cc := findNamedInZone(e, 0, state.ZHand, "Celestial Convergence")
	submitChoices(t, e, castOptionFor(t, e, cc).Index)
	answerQuiet(t, e, 200)
	cc = findNamedInZone(e, 0, state.ZBattlefield, "Celestial Convergence")
	if cc == 0 {
		t.Fatal("Celestial Convergence never entered the battlefield")
	}
	if n := e.G.Obj(cc).Counter("OMEN"); n != 7 {
		t.Fatalf("Celestial Convergence entered with %d omen counters, want 7", n)
	}
	// Seat 0's upkeeps are its odd turns; cast on turn 1, its first upkeep
	// trigger is turn 3's, so the seventh removal is turn 15's.
	driveToStep(t, e, 15, 0, state.StepUpkeep)
	answerQuiet(t, e, 200)
	if !e.G.Over {
		t.Fatal("the game did not end at the seventh omen removal")
	}
	if !e.G.Draw {
		t.Errorf("the tied 20/20 game ended with Draw=%v, want a draw", e.G.Draw)
	}
}

// answerTapOrUntapElections settles the stack the way answerQuiet does but
// answers every "taporuntapall" election with elect ("tap" or "untap") and
// returns how many elections were put to which seat.
func answerTapOrUntapElections(t *testing.T, e *Engine, elect string, limit int) (n int, seats []state.PlayerID) {
	t.Helper()
	for i := 0; i < limit && !e.G.Over; i++ {
		if len(e.pendingTriggers) == 0 && len(e.G.Stack) == 0 {
			return n, seats
		}
		d := e.Pending()
		if d == nil {
			continue
		}
		var pick = -1
		switch {
		case d.ResumeKind == "taporuntapall":
			n++
			seats = append(seats, d.Player)
			for _, o := range d.Options {
				if o.Kind == elect {
					pick = o.Index
				}
			}
		case d.Kind == decision.KPriority:
			for _, o := range d.Options {
				if o.Kind == "pass" {
					pick = o.Index
				}
			}
		default:
			if len(d.Options) == 0 {
				t.Fatalf("empty decision %+v", d)
			}
			pick = d.Options[0].Index
		}
		if pick < 0 {
			t.Fatalf("no option to answer %+v with", d)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pick}}); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}
	return n, seats
}

// TestEDHAPIsTaporUntapAll is Faces of the Past's leaf. Forge folds
// TaporUntapAll into TapOrUntapAll, so a creature dying is ONE tap-or-untap
// election for Faces' controller that sweeps every creature sharing the dying
// creature's type the same way: on a board with one tapped Bear and one
// untapped Bear, "untap" must untap only the tapped one and leave the other
// untapped (a per-creature toggle would tap it), and a second death answered
// "tap" taps every remaining Bear. The Goblin is never touched.
func TestEDHAPIsTaporUntapAll(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Faces of the Past")}, nil)
	moveNamedTo(t, e, 0, "Faces of the Past", state.ZBattlefield)
	const bear = "Name:Test Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	const goblin = "Name:Test Goblin\nManaCost:1 R\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n"
	a := onBoard(t, e, 0, bear)
	b := onBoard(t, e, 0, bear)
	c := onBoard(t, e, 0, bear)
	d := onBoard(t, e, 0, bear)
	gob := onBoard(t, e, 0, goblin)
	e.G.Obj(b).Tapped = true
	// Preconditions: the Bears differ in state, so a per-creature toggle and
	// a one-election sweep give different outcomes.
	if !e.G.Obj(b).Tapped || e.G.Obj(c).Tapped || e.G.Obj(gob).Tapped {
		t.Fatal("precondition: want b tapped, c and the Goblin untapped")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: a, From: state.ZBattlefield, To: state.ZGraveyard})
	e.priorityRound()
	n, seats := answerTapOrUntapElections(t, e, "untap", 200)
	if n != 1 || len(seats) != 1 || seats[0] != 0 {
		t.Fatalf("elections = %d to seats %v, want exactly one to seat 0", n, seats)
	}
	if e.G.Obj(b).Tapped {
		t.Error("untap elected: the tapped Bear did not untap")
	}
	if e.G.Obj(c).Tapped || e.G.Obj(d).Tapped {
		t.Errorf("untap elected: untapped Bears must stay untapped, got c=%v d=%v", e.G.Obj(c).Tapped, e.G.Obj(d).Tapped)
	}
	if e.G.Obj(gob).Tapped {
		t.Error("the Goblin does not share the dying Bear's type and must stay untouched")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: b, From: state.ZBattlefield, To: state.ZGraveyard})
	e.priorityRound()
	n, seats = answerTapOrUntapElections(t, e, "tap", 200)
	if n != 1 || len(seats) != 1 || seats[0] != 0 {
		t.Fatalf("second death: elections = %d to seats %v, want exactly one to seat 0", n, seats)
	}
	if !e.G.Obj(c).Tapped || !e.G.Obj(d).Tapped {
		t.Errorf("tap elected: every Bear should tap, got c=%v d=%v", e.G.Obj(c).Tapped, e.G.Obj(d).Tapped)
	}
	if e.G.Obj(gob).Tapped {
		t.Error("the Goblin was tapped by the second death")
	}
}

// TestEDHAPIsExchangePower is Serene Master's leaf: when it blocks, its
// power and the blocked creature's power exchange until end of combat -- the
// 2/2 attacker becomes 0 power (and dies to the blocker's now-2 damage in
// the damage step), and the blocker's 0 power reverts to 0 when combat ends.
func TestEDHAPIsExchangePower(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := combatEngine(t)
	e.G.Active = 1 // seat 1 attacks; seat 0's Serene Master blocks
	atk := onBoardReady(t, e, 1, "Name:Charging Bear\nManaCost:2\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	sm := onBoardCard(t, e, 0, lookup(t, reg, "Serene Master"))
	e.askAttackers()
	submitAttackersOnly(t, e, atk)
	drainCombatPriority(t, e)
	d := e.Pending()
	if d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("expected a blockers decision, got %+v", d)
	}
	submitBlockersOnly(t, e, sm)
	// The Blocks trigger asks for its one legal target (the creature it's
	// blocking) and then resolves in the post-declaration priority rounds:
	// answerQuiet passes priority and answers that ask with its only option.
	answerQuiet(t, e, 400)
	if got := e.Power(sm); got != 2 {
		t.Errorf("the blocker's power = %d, want the attacker's 2", got)
	}
	if got := e.Power(atk); got != 0 {
		t.Errorf("the attacker's power = %d, want the blocker's 0", got)
	}
	// The damage step: the 0-power attacker deals nothing, the 2-power
	// blocker kills it.
	driveToStep(t, e, e.G.Turn, 1, state.StepCombatDamage)
	passUntilStackEmpty(t, e, 40)
	if findNamedInZone(e, 1, state.ZGraveyard, "Charging Bear") == 0 {
		t.Error("the 0-power attacker survived the blocker's 2 damage")
	}
	if got := e.Power(sm); got != 2 {
		t.Errorf("the blocker's power = %d before combat ends, want 2", got)
	}
	// Until end of combat: past the end-of-combat step, the exchange is gone.
	driveToStep(t, e, e.G.Turn, 1, state.StepEnd)
	if got := e.Power(sm); got != 0 {
		t.Errorf("the blocker's power = %d after end of combat, want its printed 0", got)
	}
}

// TestEDHAPIsPermanentNoncreature is Bridge from Below's leaf: the SP$
// PermanentNoncreature cast resolves onto the battlefield as the
// enchantment it is (the registration retires the unimplemented-API Note
// the resolution used to emit on the way).
func TestEDHAPIsPermanentNoncreature(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Bridge from Below")}, nil)
	moveNamedTo(t, e, 0, "Bridge from Below", state.ZHand)
	addMana(t, e, 0, "BBB")
	bridge := findNamedInZone(e, 0, state.ZHand, "Bridge from Below")
	submitChoices(t, e, castOptionFor(t, e, bridge).Index)
	answerQuiet(t, e, 200)
	bridge = findNamedInZone(e, 0, state.ZBattlefield, "Bridge from Below")
	if bridge == 0 {
		t.Fatal("Bridge from Below never entered the battlefield")
	}
	types := e.G.Obj(bridge).Face().Types
	ench := false
	for _, ty := range types {
		if ty == "Enchantment" {
			ench = true
		}
	}
	if !ench {
		t.Errorf("Bridge from Below entered with types %v, want an Enchantment", types)
	}
	// The registered API is what retires the unimplemented-API fallback Note
	// the SP$ body used to emit on the way to the battlefield.
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "unimplemented API PermanentNoncreature") {
			t.Errorf("the resolution still emitted %q", ev.Text)
		}
	}
}
