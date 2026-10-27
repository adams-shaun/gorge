package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// A fixed `Type$ AddToken | Amount$ 1` replacement modifies the creation EVENT
// once (CR 614.1a, 616), however many tokens that event creates: "create two
// Food" under Quina is two Food and ONE Frog. One Token resolution is one
// creation event, though each of its mints is its own TokenCreate.

// tokenMultiForgeSrc is tokenForgeSrc with TokenAmount$ n.
func tokenMultiForgeSrc(script string, n string) string {
	return "Name:Multi Forge\nTypes:Artifact\n" +
		"A:AB$ Token | Cost$ T | TokenAmount$ " + n + " | TokenScript$ " + script + " | TokenOwner$ You | SpellDescription$ Create tokens.\n" +
		"Oracle:x\n"
}

func quinaBoard(t *testing.T, seed uint64, maker *cards.Card) (*Engine, Config, state.ObjID) {
	t.Helper()
	quina := tokenReplCorpusCard(t, "Quina, Qu Gourmet")
	e, cfg := tokenReplGame(t, seed, quina, maker)
	qid := moveSeededCard(t, e, 0, quina, state.ZBattlefield)
	if e.G.Obj(qid).Zone != state.ZBattlefield {
		t.Fatal("precondition: Quina must be on the battlefield")
	}
	m := moveSeededCard(t, e, 0, maker, state.ZBattlefield)
	return e, cfg, m
}

func TestQuinaAddsOneFrogPerCreationEvent(t *testing.T) {
	t.Parallel()
	e, cfg, m := quinaBoard(t, 59, cardByName(t, tokenMultiForgeSrc("c_a_food_sac", "2")))
	activateTokenForge(t, e, m)
	if got := countTokensNamedOnSeat(t, e, 0, "Food Token"); got != 2 {
		t.Fatalf("precondition: Food Tokens = %d, want 2", got)
	}
	if got := countTokensNamedOnSeat(t, e, 0, "Frog Token"); got != 1 {
		t.Fatalf("Frog Tokens = %d, want 1 (one creation event)", got)
	}
	replayCheck(t, e, cfg)
}

func TestQuinaTwoScriptsOneFrog(t *testing.T) {
	t.Parallel()
	e, cfg, m := quinaBoard(t, 59, cardByName(t, tokenForgeSrc("c_a_food_sac,c_a_treasure_sac")))
	activateTokenForge(t, e, m)
	if f, tr := countTokensNamedOnSeat(t, e, 0, "Food Token"), countTokensNamedOnSeat(t, e, 0, "Treasure Token"); f != 1 || tr != 1 {
		t.Fatalf("precondition: Food/Treasure = %d/%d, want 1/1", f, tr)
	}
	if got := countTokensNamedOnSeat(t, e, 0, "Frog Token"); got != 1 {
		t.Fatalf("Frog Tokens = %d, want 1", got)
	}
	replayCheck(t, e, cfg)
}

func TestQuinaTwoResolutionsTwoFrogs(t *testing.T) {
	t.Parallel()
	maker := cardByName(t, tokenForgeSrc("c_a_food_sac"))
	quina := tokenReplCorpusCard(t, "Quina, Qu Gourmet")
	e, cfg := tokenReplGame(t, 59, quina, maker, maker)
	moveSeededCard(t, e, 0, quina, state.ZBattlefield)
	m1 := moveSeededCard(t, e, 0, maker, state.ZBattlefield)
	m2 := moveSeededCard(t, e, 0, maker, state.ZBattlefield)
	if m1 == m2 {
		t.Fatal("precondition: two distinct makers")
	}
	activateTokenForge(t, e, m1)
	if got := countTokensNamedOnSeat(t, e, 0, "Frog Token"); got != 1 {
		t.Fatalf("precondition: Frog Tokens after the first resolution = %d, want 1", got)
	}
	activateTokenForge(t, e, m2)
	if got := countTokensNamedOnSeat(t, e, 0, "Food Token"); got != 2 {
		t.Fatalf("precondition: Food Tokens = %d, want 2", got)
	}
	if got := countTokensNamedOnSeat(t, e, 0, "Frog Token"); got != 2 {
		t.Fatalf("Frog Tokens = %d, want 2 (the batch must not leak across resolutions)", got)
	}
	replayCheck(t, e, cfg)
}

func TestCaseOfPilferedProofAddsOneClue(t *testing.T) {
	t.Parallel()
	cs := tokenReplCorpusCard(t, "Case of the Pilfered Proof")
	var seed []*cards.Card
	for _, n := range []string{"Det One", "Det Two", "Det Three"} {
		seed = append(seed, cardByName(t, "Name:"+n+"\nTypes:Creature Detective Human\nPT:1/1\nOracle:x\n"))
	}
	maker := cardByName(t, tokenMultiForgeSrc("c_a_food_sac", "2"))
	e, cfg := tokenReplGame(t, 59, append([]*cards.Card{cs, maker}, seed...)...)
	moveSeededCard(t, e, 0, cs, state.ZBattlefield)
	for _, d := range seed {
		moveSeededCard(t, e, 0, d, state.ZBattlefield)
	}
	m := moveSeededCard(t, e, 0, maker, state.ZBattlefield)
	addMana(t, e, 0, "") // a fresh priority decision after the seeded moves
	turn := e.G.Turn
	for i := 0; i < 400 && !(e.G.Turn == turn && e.G.Step == state.StepEnd); i++ {
		d := e.Pending()
		if d == nil {
			continue
		}
		if d.Kind == decision.KPriority {
			passOnce(t, e)
			continue
		}
		// The three Detectives' counter triggers: any order.
		picks := make([]int, 0, d.Min)
		for _, o := range d.Options[:d.Min] {
			picks = append(picks, o.Index)
		}
		submitChoices(t, e, picks...)
	}
	if e.G.Turn != turn || e.G.Step != state.StepEnd {
		t.Fatalf("did not reach the end step (turn %d %s)", e.G.Turn, e.G.Step)
	}
	passUntilStackEmpty(t, e, 20)
	if got := countTokensNamedOnSeat(t, e, 0, "Clue Token"); got != 0 {
		t.Fatalf("precondition: %d Clue Tokens before any creation", got)
	}
	submitChoices(t, e, abilityOption(t, e, m, 0).Index)
	passUntilStackEmpty(t, e, 20)
	if got := countTokensNamedOnSeat(t, e, 0, "Food Token"); got != 2 {
		t.Fatalf("precondition: Food Tokens = %d, want 2 (the Case must be solved and the maker resolved)", got)
	}
	if got := countTokensNamedOnSeat(t, e, 0, "Clue Token"); got != 1 {
		t.Fatalf("Clue Tokens = %d, want 1", got)
	}
	replayCheck(t, e, cfg)
}
