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
//
// Scope note: this task's fix is the Compare head in effects/count.go alone.
// The card-text MOVEMENT contract ("one Forest onto the battlefield tapped,
// the rest into hand") rides on three downstream mechanisms outside this
// task's authorized scope, each measured and filed separately: the
// `Card.IsRemembered` filter predicate (unknown, fails closed -- the largest
// unknown-predicate family, 266 cards / 365 uses), the mid-resolution
// Remembered set surviving a suspension (a cast spell's Remembered lives only
// in the resolving Ctx frame, lost when DBBattlefield's own ask suspends the
// walk), and object-target dispatch for an exactly-`Origin$ Library`
// ChangeZone carrying `Defined$` (effSearchLibrary reads a Defined$ as the
// library-OWNER selector). Measured with them absent: after the main search
// is answered, DBBattlefield poses a zero-option 0..0 "choose 0 card(s)"
// search ask, the empty answer is its only legal one, and the chain completes
// without moving anything -- so the tests below pin the decision bounds (the
// user-reported defect) and the chain completing without wedging, and the
// movement contract is NOT pinned here.

// drainSearchChain answers every follow-on hidden-search choose the
// resolution chain poses, passing priority when nothing else is pending, and
// returns once the game is back at a priority decision or over. A zero-option
// 0..0 ask takes the empty answer (its only legal one); a one-option ask
// takes that option.
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
		if len(d.Options) == 0 {
			if d.Min != 0 || d.Max != 0 {
				t.Fatalf("zero-option sub-ask is not 0..0: %+v", d)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: nil}); err != nil {
				t.Fatalf("submit empty answer: %v", err)
			}
			continue
		}
		if len(d.Options) == 1 {
			submitChoices(t, e, d.Options[0].Index)
			continue
		}
		t.Fatalf("unexpected multi-option sub-ask: %+v", d)
	}
}

func TestNissasPilgrimageSearchMaxFollowsSpellMastery(t *testing.T) {
	reg := searchTestRegistry(t)
	e, cfg := searchEngine(t, reg, "Nissa's Pilgrimage")
	_, d := castSearchSpell(t, e, "Nissa's Pilgrimage")
	// Empty spell graveyard: mastery fails, so "up to two" -- Min 0 (a
	// stated-quality filter keeps the fail-to-find allowance) and Max 2.
	// Before the Compare head this read max=0: "choose up to 0 card(s)",
	// the user-reported defect.
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
	// The search is answerable with two Forests now, and the resolution
	// chain completes: DBBattlefield's remembered-filter sub-search poses
	// its zero-option 0..0 ask (see the scope note above), the empty answer
	// closes it, and no sub-ability wedges the game. The whole game still
	// replays byte-for-byte from the log.
	submitChoices(t, e, d.Options[0].Index, d.Options[1].Index)
	drainSearchChain(t, e, 20)
	d = e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("after the search chain the game must be back at priority, got %+v", d)
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
		cfg := Config{Seed: 4210, Names: []string{"willcaster", "opponent"}, PinnedStart: true,
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
