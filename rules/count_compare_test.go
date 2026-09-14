package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Tests for Count$Compare-driven card behaviour at the engine level:
// Nissa's Pilgrimage's spell-mastery search (ChangeNum$ X behind
// SVar:X:Count$Compare Y GE2.3.2) and the Will-of-the-X commander cycle's
// inline CharmNum$ Count$Compare Y GE1.2.1.

// drainSearchChain answers every follow-on hidden-search choose the
// resolution chain poses (DBBattlefield's "put one onto the battlefield",
// then any further ask), passing priority when nothing else is pending, and
// returns once the game is back at a priority decision or over. Each
// sub-search is answered with option 0 -- for a one-card remembered list
// that is the only real choice.
func drainSearchChain(t *testing.T, e *Engine, limit int) {
	t.Helper()
	for i := 0; i < limit && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision pending mid-chain (game over: %v)", e.G.Over)
		}
		if d.Kind == decision.KPriority {
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("priority decision with no pass option: %+v", d)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
				t.Fatalf("submit pass: %v", err)
			}
			continue
		}
		submitChoices(t, e, 0)
	}
}

func TestNissasPilgrimageSearchMaxFollowsSpellMastery(t *testing.T) {
	reg := searchTestRegistry(t)
	e, cfg := searchEngine(t, reg, "Nissa's Pilgrimage")
	_, d := castSearchSpell(t, e, "Nissa's Pilgrimage")
	// Empty spell graveyard: mastery fails, so "up to two" -- Min 0 (a
	// stated-quality filter keeps the fail-to-find allowance) and Max 2.
	if d.Kind != decision.KChoose || d.Min != 0 || d.Max != 2 {
		t.Fatalf("empty spell graveyard: search decision min=%d max=%d kind=%v, want KChoose 0..2", d.Min, d.Max, d.Kind)
	}
	forests := map[state.ObjID]bool{}
	for _, fid := range e.G.Zone(state.ZLibrary, 0) {
		if o := e.G.Obj(fid); o != nil && o.Face() != nil && o.Face().Name == "Forest" {
			forests[fid] = true
		}
	}
	if len(forests) == 0 || len(d.Options) != len(forests) {
		t.Fatalf("options = %d, library basic Forests = %d", len(d.Options), len(forests))
	}
	for _, o := range d.Options {
		if !forests[o.Obj] {
			t.Fatalf("search option %d is not a library basic Forest", o.Obj)
		}
	}
	// Answer with TWO Forests: DBBattlefield puts ONE remembered Forest
	// onto the battlefield tapped, DBHand moves the rest into hand. The
	// sub-search's options follow the library's zone order, not the answer
	// order, so assert the card-text contract (one tapped on the battlefield,
	// one in hand) rather than a specific assignment.
	picked := []state.ObjID{d.Options[0].Obj, d.Options[1].Obj}
	submitChoices(t, e, d.Options[0].Index, d.Options[1].Index)
	drainSearchChain(t, e, 20)
	var onBF, inHand state.ObjID
	for _, id := range picked {
		switch e.G.Obj(id).Zone {
		case state.ZBattlefield:
			onBF = id
		case state.ZHand:
			inHand = id
		default:
			t.Fatalf("picked Forest %d in %s, want battlefield or hand", id, e.G.Obj(id).Zone)
		}
	}
	if onBF == 0 || inHand == 0 {
		t.Fatalf("one Forest on the battlefield and one in hand, got bf=%d hand=%d", onBF, inHand)
	}
	if !e.G.Obj(onBF).Tapped {
		t.Fatal("the Forest put onto the battlefield entered untapped")
	}
	replayCheck(t, e, cfg)

	// Spell mastery: two instants in the graveyard raise the max to three.
	e2, _ := searchEngine(t, reg, "Nissa's Pilgrimage", "Giant Growth", "Giant Growth")
	searchMoveByName(t, e2, "Giant Growth", state.ZGraveyard)
	searchMoveByName(t, e2, "Giant Growth", state.ZGraveyard)
	_, d2 := castSearchSpell(t, e2, "Nissa's Pilgrimage")
	if d2.Kind != decision.KChoose || d2.Min != 0 || d2.Max != 3 {
		t.Fatalf("two instants in graveyard: search decision min=%d max=%d, want 0..3", d2.Min, d2.Max)
	}
}

func TestWillOfTheJeskaiCharmNumCountsACommander(t *testing.T) {
	reg := searchTestRegistry(t)
	will := searchCorpusCard(t, reg, "Will of the Jeskai")
	isamaru := searchCorpusCard(t, reg, "Isamaru, Hound of Konda")
	forest := searchCorpusCard(t, reg, "Forest")
	mountain := searchCorpusCard(t, reg, "Mountain")

	build := func(commander bool) (*Engine, state.ObjID) {
		deck := []*cards.Card{will, isamaru}
		for len(deck) < 40 {
			if len(deck)%2 == 0 {
				deck = append(deck, mountain)
			} else {
				deck = append(deck, forest)
			}
		}
		opp := make([]*cards.Card, 40)
		for i := range opp {
			opp[i] = mountain
		}
		cfg := Config{Seed: 4210, Names: []string{"willcaster", "opponent"},
			Decks:      [][]*cards.Card{deck, opp},
			Tokens:     reg.Tokens,
			Commanders: [][]int{{1}, {}}}
		e := New(cfg)
		e.Advance()
		toMain1(t, e)
		if commander {
			var cmdr state.ObjID
			for _, id := range e.G.Zone(state.ZCommand, 0) {
				cmdr = id
			}
			if cmdr == 0 {
				t.Fatal("no commander in the command zone")
			}
			e.emit(events.Event{Kind: events.MoveZone, Obj: cmdr, From: state.ZCommand, To: state.ZBattlefield})
			e.pending = nil
			e.priorityRound()
		}
		id := searchMoveByName(t, e, "Will of the Jeskai", state.ZHand)
		addMana(t, e, 0, "RRRR")
		return e, id
	}

	// No commander on the battlefield: the compared SVar counts 0, GE1
	// fails, and the modal ask is the plain choose-one.
	e, id := build(false)
	d := castFixture(t, e, id, -1)
	if d.Kind != decision.KModes || d.Min != 1 || d.Max != 1 {
		t.Fatalf("without a commander: modal decision min=%d max=%d kind=%v, want KModes 1..1", d.Min, d.Max, d.Kind)
	}

	// Commander on the battlefield: CharmNum resolves to 2 and the ask
	// accepts a both-modes answer.
	e2, id2 := build(true)
	d2 := castFixture(t, e2, id2, -1)
	if d2.Kind != decision.KModes || d2.Min != 2 || d2.Max != 2 {
		t.Fatalf("with a commander: modal decision min=%d max=%d, want KModes 2..2", d2.Min, d2.Max)
	}
	submitChoices(t, e2, d2.Options[0].Index, d2.Options[1].Index)
	passUntilStackEmpty(t, e2, 20)
	if z := e2.G.Obj(id2).Zone; z != state.ZGraveyard {
		t.Fatalf("resolved Will in %s, want graveyard", z)
	}
}
