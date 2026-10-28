package rules

import (
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// kw:Read ahead (CR 702.155): the Saga's controller chooses 1..final chapter as
// it enters, it enters with that many lore counters, and on its entry turn only
// the chapter whose number equals the lore count triggers. The cards are the
// real DMU Elder Dragon War (3 chapters) and Urza's Saga (no read ahead).

// readAheadGame seats the named corpus card on top of seat 0's library over
// Mountains, with no priority decision pending (an entry's as-enters ask is
// only posed while nothing else is outstanding), and returns the engine and
// its replayable config.
func readAheadGame(t *testing.T, reg *cards.Registry, name string) (*Engine, Config) {
	t.Helper()
	mountain := lookup(t, reg, "Mountain")
	deck := func(first ...*cards.Card) []*cards.Card {
		out := append([]*cards.Card{}, first...)
		for len(out) < 40 {
			out = append(out, mountain)
		}
		return out
	}
	cfg := seatZeroStart(Config{Seed: 278, Names: []string{"a", "b"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{deck(lookup(t, reg, name)), deck()}})
	return New(cfg), cfg
}

// readAheadChapters lists the chapter SVars the Saga has queued or pushed, in
// the order they were queued: the entry's answer continuation may already have
// moved a queued chapter onto the stack, so the log's DelayedPush events (whose
// Counter is the chapter SVar) count too.
func readAheadChapters(e *Engine, id state.ObjID) []string {
	var got []string
	for _, ev := range e.L.Events {
		if ev.Kind == events.DelayedPush && ev.Obj == id {
			got = append(got, ev.Counter)
		}
	}
	for _, pt := range e.pendingTriggers {
		if pt.Source == id {
			got = append(got, pt.Execute)
		}
	}
	return got
}

// readAheadCast casts the named card from seat 0's hand at main 1 with four
// red mana and drives the stack, answering the as-enters read ahead ask with
// chapter (any other ask gets sagaAnswer's first-Min answer), until the Saga
// is on the battlefield.
func readAheadCast(t *testing.T, e *Engine, name string, chapter int) state.ObjID {
	t.Helper()
	var id state.ObjID
	for _, oid := range e.G.Zone(state.ZHand, 0) {
		if o := e.G.Obj(oid); o != nil && o.Face() != nil && o.Face().Name == name {
			id = oid
		}
	}
	if id == 0 {
		id = moveByName(t, e, 0, name, state.ZHand)
	}
	if o := e.G.Obj(id); o.Zone != state.ZHand {
		t.Fatalf("precondition: %s is in %s, want the hand", name, o.Zone)
	}
	addMana(t, e, 0, "RRRR")
	castFirstOf(t, e, id)
	asked := false
	for range 200 {
		if o := e.G.Obj(id); o.Zone == state.ZBattlefield {
			if !asked {
				t.Fatal("the cast Saga entered without a read ahead ask")
			}
			return id
		}
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while the cast Saga resolves")
		}
		if d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "readahead" {
			asked = true
			submitChoices(t, e, chapter-1)
			continue
		}
		sagaAnswer(t, e, d)
	}
	t.Fatal("the cast Saga never entered")
	return 0
}

func describeAsk(d *decision.Decision) string {
	if d == nil {
		return "no decision"
	}
	return string(d.Kind) + " with " + strconv.Itoa(len(d.Options)) + " options"
}

// answerReadAheadEntry answers a pending as-enters read ahead ask with chapter,
// for fixtures that seat a read-ahead Saga as scenery. It does nothing when no
// such ask is pending.
func answerReadAheadEntry(t *testing.T, e *Engine, chapter int) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "readahead" {
		return
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{chapter - 1}}); err != nil {
		t.Fatalf("submit read ahead chapter %d: %v", chapter, err)
	}
}

// readAheadEnter moves the named card to the battlefield and answers the posed
// read-ahead KChoose with chapter. It asserts the ask is exactly the
// final-chapter-many ascending options before answering.
func readAheadEnter(t *testing.T, e *Engine, name string, chapters, chapter int) state.ObjID {
	t.Helper()
	id := moveByName(t, e, 0, name, state.ZBattlefield)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) != chapters {
		t.Fatalf("read ahead entry choice is %s, want a %d-option KChoose", describeAsk(d), chapters)
	}
	for i, o := range d.Options {
		if o.Kind != "readahead" || o.Amount != i+1 {
			t.Fatalf("option %d = %+v, want Kind readahead Amount %d", i, o, i+1)
		}
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{chapter - 1}}); err != nil {
		t.Fatalf("submit read ahead chapter %d: %v", chapter, err)
	}
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("%s did not enter after the answer: %+v", name, o)
	}
	return id
}

func TestReadAheadChoiceSetsLoreAndOnlyThatChapterTriggers(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, sagaCards(t, reg, "The Elder Dragon War"), nil)
	life := e.G.Players[1].Life
	id := readAheadCast(t, e, "The Elder Dragon War", 2)
	o := e.G.Obj(id)
	if !o.EnteredThisTurn {
		t.Fatal("precondition: the Saga is not marked as entered this turn")
	}
	if got := o.Counter("LORE"); got != 2 {
		t.Fatalf("LORE after choosing chapter II = %d, want 2", got)
	}
	if got := readAheadChapters(e, id); len(got) != 1 || got[0] != "DBDiscard" {
		t.Fatalf("queued chapters = %v, want only [DBDiscard] (chapter II)", got)
	}
	sagaSettle(t, e, 200)
	if got := e.G.Players[1].Life; got != life {
		t.Fatalf("opponent life %d -> %d: skipped chapter I dealt damage", life, got)
	}
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield {
		t.Fatalf("Saga at lore 2 of 3 left the battlefield (%s)", o.Zone)
	}
	replayCheck(t, e, cfg)
}

func TestReadAheadChapterOneIsTheOrdinaryEntry(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := readAheadGame(t, reg, "The Elder Dragon War")
	life := e.G.Players[1].Life
	id := readAheadEnter(t, e, "The Elder Dragon War", 3, 1)
	if got := e.G.Obj(id).Counter("LORE"); got != 1 {
		t.Fatalf("LORE after choosing chapter I = %d, want 1", got)
	}
	if got := readAheadChapters(e, id); len(got) != 1 || got[0] != "DBDealDamage" {
		t.Fatalf("queued chapters = %v, want only [DBDealDamage] (chapter I)", got)
	}
	sagaSettle(t, e, 200)
	if got := e.G.Players[1].Life; got != life-2 {
		t.Fatalf("opponent life %d -> %d, want chapter I's 2 damage", life, got)
	}
	replayCheck(t, e, cfg)
}

func TestReadAheadFinalChapterSacrificesAfterIt(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := readAheadGame(t, reg, "The Elder Dragon War")
	life := e.G.Players[1].Life
	id := readAheadEnter(t, e, "The Elder Dragon War", 3, 3)
	if got := e.G.Obj(id).Counter("LORE"); got != 3 {
		t.Fatalf("LORE after choosing chapter III = %d, want 3", got)
	}
	if got := readAheadChapters(e, id); len(got) != 1 || got[0] != "DBToken" {
		t.Fatalf("queued chapters = %v, want only [DBToken] (chapter III)", got)
	}
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Saga left before chapter III resolved (%s)", o.Zone)
	}
	sagaSettle(t, e, 200)
	if got := e.G.Players[1].Life; got != life {
		t.Fatalf("opponent life %d -> %d: a skipped chapter dealt damage", life, got)
	}
	dragons := 0
	for _, oid := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(oid); o != nil && o.IsToken && o.Face() != nil && o.Face().Name == "Dragon Token" {
			dragons++
		}
	}
	if dragons != 1 {
		t.Fatalf("Dragon tokens on the battlefield = %d, want 1", dragons)
	}
	if o := e.G.Obj(id); o.Zone != state.ZGraveyard {
		t.Fatalf("Saga after chapter III is in %s, want the graveyard (CR 714.4)", o.Zone)
	}
	replayCheck(t, e, cfg)
}

func TestReadAheadSameTurnExtraCounterTriggersExactMatch(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := readAheadGame(t, reg, "The Elder Dragon War")
	id := readAheadEnter(t, e, "The Elder Dragon War", 3, 1)
	sagaSettle(t, e, 200)
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield || o.Counter("LORE") != 1 || !o.EnteredThisTurn {
		t.Fatalf("precondition: Saga %+v is not a lore-1 entry-turn permanent", o)
	}
	sagaAddLore(e, id, 1)
	if got := readAheadChapters(e, id); len(got) != 2 || got[1] != "DBDiscard" {
		t.Fatalf("chapters after a second lore counter = %v, want [DBDealDamage DBDiscard] (count is exactly 2)", got)
	}
	sagaSettle(t, e, 200)
	replayCheck(t, e, cfg)
}

func TestReadAheadNextTurnAdvancesNormally(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, sagaCards(t, reg, "The Elder Dragon War"), nil)
	id := readAheadCast(t, e, "The Elder Dragon War", 1)
	sagaSettle(t, e, 200)
	turn := e.G.Turn
	for range 600 {
		if len(readAheadChapters(e, id)) > 1 {
			break
		}
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while passing to the next turn")
		}
		sagaAnswer(t, e, d)
	}
	if e.G.Turn != turn+2 {
		t.Fatalf("turn = %d, want the controller's next turn %d", e.G.Turn, turn+2)
	}
	o := e.G.Obj(id)
	if o.EnteredThisTurn {
		t.Fatal("precondition: the Saga still reads as entered this turn")
	}
	if got := o.Counter("LORE"); got != 2 {
		t.Fatalf("LORE at the controller's next precombat main = %d, want 2", got)
	}
	if got := readAheadChapters(e, id); len(got) != 2 || got[1] != "DBDiscard" {
		t.Fatalf("chapters over two turns = %v, want [DBDealDamage DBDiscard]", got)
	}
	replayCheck(t, e, cfg)
}

func TestReadAheadAbsentSagaEntersWithOneLoreAndAsksNothing(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := readAheadGame(t, reg, "Urza's Saga")
	id := moveByName(t, e, 0, "Urza's Saga", state.ZBattlefield)
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
		t.Fatalf("a Saga without read ahead posed a choice: %+v", d)
	}
	if got := e.G.Obj(id).Counter("LORE"); got != 1 {
		t.Fatalf("LORE = %d, want 1", got)
	}
	replayCheck(t, e, cfg)
}
