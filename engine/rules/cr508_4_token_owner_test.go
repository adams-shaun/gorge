package rules

// CR 508.4's chooser is the entering creature's controller, not the
// resolving ability's controller. Najeela, the Blade-Blossom is the corpus
// carrier where those differ: its Attack trigger is controlled by Najeela's
// controller, but `TokenOwner$ TriggeredAttackerController` mints the Warrior
// for the ATTACKING player, so the token's own controller must be asked which
// defender it attacks.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func najeelaWarriorAttacks(t *testing.T) (*Engine, Config) {
	t.Helper()
	reg := searchTestRegistry(t)
	e, cfg := corpusEngineCfg(t, reg,
		[]*cards.Card{lookup(t, reg, "Najeela, the Blade-Blossom"), lookup(t, reg, "Jace, the Mind Sculptor")},
		[]*cards.Card{lookup(t, reg, "Bloodsoaked Champion")})
	moveByName(t, e, 0, "Najeela, the Blade-Blossom", state.ZBattlefield)
	jace := moveByName(t, e, 0, "Jace, the Mind Sculptor", state.ZBattlefield)
	e.emit(events.Event{Kind: events.CounterChange, Obj: jace, Counter: "LOYALTY", Amount: 3})
	warrior := moveByName(t, e, 1, "Bloodsoaked Champion", state.ZBattlefield)
	if o := e.G.Obj(jace); o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("precondition: Jace = %+v, want battlefield under seat 0", o)
	}
	// Seat 1 is the attacking player (the synthetic DeclareAttackers names
	// the DEFENDING seat), so the Warrior's controller and the token's owner
	// are seat 1 while Najeela's controller -- the resolving trigger's
	// controller -- is seat 0. Drive to seat 1's real declare-attackers step
	// so the event stream replays exactly.
	driveToStep(t, e, 2, 1, state.StepDeclareAttackers)
	if d := e.Pending(); d != nil && d.Kind == decision.KAttackers {
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player}); err != nil {
			t.Fatalf("submit no attackers: %v", err)
		}
	}
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 0, IDs: []state.ObjID{warrior}})
	e.putTriggersOnStack()
	answerTriggerOrders(t, e)
	if len(e.G.Stack) == 0 {
		t.Fatal("Najeela's Attack trigger did not reach the stack")
	}
	return e, cfg
}

func drainNajeela(t *testing.T, e *Engine, limit int, pick func(*decision.Decision) int) int {
	t.Helper()
	asked := 0
	for i := 0; i < limit && !e.G.Over; i++ {
		if len(e.pendingTriggers) > 0 {
			e.putTriggersOnStack()
			answerTriggerOrders(t, e)
			continue
		}
		if len(e.G.Stack) == 0 {
			break
		}
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while draining the stack (depth %d)", len(e.G.Stack))
		}
		switch {
		case d.Kind == decision.KTriggerOptional:
			submitChoices(t, e, 0)
		case d.Kind == decision.KChoose && d.ResumeKind == "entering_defender":
			asked++
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pick(d)}}); err != nil {
				t.Fatalf("submit entering_defender: %v", err)
			}
		case d.Kind == decision.KPriority:
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("priority decision has no pass option: %+v", d)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
				t.Fatalf("submit pass: %v", err)
			}
		default:
			t.Fatalf("unexpected decision %+v", d)
		}
	}
	return asked
}

// TestCR508_4NajeelaTokenOwnerIsTheChooser: the Warrior token is minted for
// the attacking player (seat 1), so seat 1 -- not Najeela's controller seat 0
// -- is asked, and the token attacks the chosen Jace under seat 0.
func TestCR508_4NajeelaTokenOwnerIsTheChooser(t *testing.T) {
	t.Parallel()
	e, cfg := najeelaWarriorAttacks(t)
	var offered []decision.Option
	var jace state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Jace, the Mind Sculptor" {
			jace = id
		}
	}
	if jace == 0 {
		t.Fatal("precondition: Jace is not on seat 0's battlefield")
	}
	asked := drainNajeela(t, e, 60, func(d *decision.Decision) int {
		offered = d.Options
		if d.Player != 1 {
			t.Fatalf("entering_defender asked of seat %d, want the token's controller seat 1", d.Player)
		}
		for _, o := range d.Options {
			if o.Obj == jace {
				return o.Index
			}
		}
		t.Fatalf("entering_defender options %+v do not offer Jace %d", d.Options, jace)
		return 0
	})
	if asked != 1 {
		t.Fatalf("entering_defender asked %d times, want once for the one minted Warrior", asked)
	}
	if len(offered) != 2 || offered[0].Player != 0 || offered[1].Obj != jace {
		t.Fatalf("options = %+v, want [player 0, Jace %d]", offered, jace)
	}
	var warriors []*state.Object
	for _, id := range e.G.Zone(state.ZBattlefield, 1) {
		if o := e.G.Obj(id); o != nil && o.IsToken && o.Face() != nil && o.Face().Name == "Warrior Token" {
			warriors = append(warriors, o)
		}
	}
	if len(warriors) != 1 {
		t.Fatalf("Najeela minted %d Warrior tokens, want 1", len(warriors))
	}
	w := warriors[0]
	if !w.Tapped || !w.IsAttacking || w.Attacking != 0 || w.AttackingBattle != jace {
		t.Fatalf("Warrior tapped=%v attacking=%v defender=%d battle=%d, want tapped, attacking Jace %d under seat 0",
			w.Tapped, w.IsAttacking, w.Attacking, w.AttackingBattle, jace)
	}
	replayCheck(t, e, cfg)
}

// tokenForgeAttackingSrc is the authored maker whose token enters attacking
// without naming a defender, so the CR 508.4 chooser path runs.
func tokenForgeAttackingSrc(script string) string {
	return "Name:Token Forge Attacking\nTypes:Artifact\n" +
		"A:AB$ Token | Cost$ T | TokenScript$ " + script + " | TokenOwner$ You | TokenTapped$ True | TokenAttacking$ True | SpellDescription$ Create a tapped attacking token.\n" +
		"Oracle:x\n"
}

// TestCR508_4CraftyCutpurseRedirectedTokenDoesNotAttackItself: the token is
// minted for the attacking player seat 1, then Crafty Cutpurse's CreateToken
// replacement moves it under seat 0. Seat 0 is not the active player, so the
// frozen fallback would name seat 0 itself; the token must enter not attacking
// rather than attack its own controller.
func TestCR508_4CraftyCutpurseRedirectedTokenDoesNotAttackItself(t *testing.T) {
	t.Parallel()
	cutpurse := tokenReplCorpusCard(t, "Crafty Cutpurse")
	jace := lookup(t, searchTestRegistry(t), "Jace, the Mind Sculptor")
	maker := cardByName(t, tokenForgeAttackingSrc("g_1_1_squirrel"))
	e, cfg := tokenReplGameSeats(t, 1391, []*cards.Card{cutpurse, jace}, []*cards.Card{maker})
	driveToTurn(t, e, e.G.Turn+1, 1)
	toMain1(t, e)
	jaceID := moveSeededCard(t, e, 0, jace, state.ZBattlefield)
	e.emit(events.Event{Kind: events.CounterChange, Obj: jaceID, Counter: "LOYALTY", Amount: 3})
	cutID := moveSeededCard(t, e, 0, cutpurse, state.ZBattlefield)
	if o := e.G.Obj(cutID); o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("precondition: Crafty Cutpurse = %+v, want battlefield under seat 0", o)
	}
	// Resolve Cutpurse's ETB so the "this turn" replacement is live.
	e.Advance()
	e.putTriggersOnStack()
	answerTriggerOrders(t, e)
	if len(e.G.Stack) == 0 {
		t.Fatal("Crafty Cutpurse's ETB trigger did not reach the stack")
	}
	passUntilStackEmpty(t, e, 30)
	m := moveSeededCard(t, e, 1, maker, state.ZBattlefield)
	addMana(t, e, 1, "")
	submitChoices(t, e, abilityOption(t, e, m, 0).Index)
	drainAnsweringEnteringDefender(t, e, 40, func(d *decision.Decision) int {
		return d.Options[0].Index
	})
	tokens := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		o := e.G.Obj(id)
		if o == nil || !o.IsToken || o.Face() == nil || o.Face().Name != "Squirrel Token" {
			continue
		}
		tokens++
		if o.Controller != 0 {
			t.Fatalf("redirected token controller = %d, want Crafty's controller 0", o.Controller)
		}
		if o.IsAttacking {
			t.Fatalf("redirected token is attacking seat %d; seat 0 is not the active player, so it cannot attack", o.Attacking)
		}
	}
	if tokens != 1 {
		t.Fatalf("Crafty redirected %d Squirrel Tokens to seat 0, want 1", tokens)
	}
	replayCheck(t, e, cfg)
}
