package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Continuous SharedKeywordsZone$/SharedRestrictions$ class, pinned on the
// real corpus carriers Eater of Virtue and Cairn Wanderer. Before the filter,
// Eater of Virtue's AddKeyword$ line granted all thirteen listed keywords with
// nothing exiled, and Cairn Wanderer always had every keyword on its line.

// eaterOfVirtueKeywords is the AddKeyword$ list on Eater of Virtue's shared
// static, in script order.
var eaterOfVirtueKeywords = []string{
	"Flying", "First Strike", "Double Strike", "Deathtouch", "Haste", "Hexproof",
	"Indestructible", "Lifelink", "Menace", "Protection", "Reach", "Trample", "Vigilance",
}

// sharedZoneFlyerSrc is an authored fixture creature carrying exactly the two
// printed keywords the exiled-card test shares. It is inline, never a Forge
// .txt.
const sharedZoneFlyerSrc = "Name:Shared Zone Flyer\nManaCost:1 W\nTypes:Creature Bird\nPT:1/1\nK:Flying\nK:Lifelink\nOracle:x\n"

// sharedZonePlainSrc is the fresh equipment bearer with no keywords of its own.
const sharedZonePlainSrc = "Name:Shared Zone Plain\nManaCost:1\nTypes:Creature Bear\nPT:1/1\nOracle:x\n"

// sharedKeywordBoard builds a two-seat corpus game whose seat 0 deck carries
// the named cards, advances to the first priority, and returns the engine and
// the config replayCheck needs.
func sharedKeywordBoard(t *testing.T, seed uint64, deck ...*cards.Card) (*Engine, Config) {
	t.Helper()
	e, cfg := tokenReplGame(t, seed, deck...)
	e.pending = nil
	e.Advance()
	return e, cfg
}

// drainSharedKeywordStack clears the pending priority and resolves any
// triggered ability the last emitted event queued (Eater of Virtue's dies
// trigger).
func drainSharedKeywordStack(t *testing.T, e *Engine) {
	t.Helper()
	e.pending = nil
	e.Advance()
	if len(e.G.Stack) > 0 {
		passUntilStackEmpty(t, e, 30)
	}
}

// sharedKeywordSet reports id's derived keywords as a set.
func sharedKeywordSet(e *Engine, id state.ObjID) map[string]bool {
	out := map[string]bool{}
	for _, kw := range e.Derived(id).Keywords {
		out[kw] = true
	}
	return out
}

func TestEaterOfVirtueGrantsNothingWithEmptyExile(t *testing.T) {
	t.Parallel()
	eater := corpusAlternativeCard(t, "Eater of Virtue")
	bears := corpusAlternativeCard(t, "Grizzly Bears")
	e, cfg := sharedKeywordBoard(t, 5, eater, bears)
	eaterID := moveByName(t, e, 0, "Eater of Virtue", state.ZBattlefield)
	bearsID := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	if o := e.G.Obj(eaterID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Eater of Virtue is not on the battlefield: %+v", o)
	}
	// Precondition: nothing is exiled, so no card can share a keyword with
	// the equipped creature.
	for _, p := range []state.PlayerID{0, 1} {
		if got := len(e.G.Zone(state.ZExile, p)); got != 0 {
			t.Fatalf("precondition: seat %d has %d exiled cards, want none", p, got)
		}
	}
	attachFixture(t, e, eaterID, bearsID)
	// Precondition: the same static's AddPower$ half is live (+2/+0), so the
	// keyword check below is about the filter, not a dead static.
	if d := e.Derived(bearsID); d.Power != 4 || d.Toughness != 2 {
		t.Fatalf("precondition: equipped Grizzly Bears is %d/%d, want 4/2 from +2/+0", d.Power, d.Toughness)
	}
	if kws := sharedKeywordSet(e, bearsID); len(kws) != 0 {
		t.Fatalf("Grizzly Bears has %v with nothing exiled with Eater of Virtue, want no keyword", e.Derived(bearsID).Keywords)
	}
	replayCheck(t, e, cfg)
}

func TestEaterOfVirtueSharesExiledCardsKeywords(t *testing.T) {
	t.Parallel()
	eater := corpusAlternativeCard(t, "Eater of Virtue")
	bears := corpusAlternativeCard(t, "Grizzly Bears")
	flyer := card(t, sharedZoneFlyerSrc)
	fresh := card(t, sharedZonePlainSrc)
	e, cfg := sharedKeywordBoard(t, 6, eater, bears, flyer, fresh)
	eaterID := moveByName(t, e, 0, "Eater of Virtue", state.ZBattlefield)
	bearsID := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	flyerID := moveByName(t, e, 0, "Shared Zone Flyer", state.ZBattlefield)
	freshID := moveByName(t, e, 0, "Shared Zone Plain", state.ZBattlefield)
	attachFixture(t, e, eaterID, bearsID)
	if hasKeywordNamed(e, bearsID, "Flying") {
		t.Fatalf("precondition: empty-exile Eater of Virtue already grants Grizzly Bears Flying: %v",
			e.Derived(bearsID).Keywords)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: bearsID, From: state.ZBattlefield, To: state.ZGraveyard})
	drainSharedKeywordStack(t, e)
	if o := e.G.Obj(bearsID); o == nil || o.Zone != state.ZExile || o.ExiledWith != eaterID {
		t.Fatalf("precondition: equipped Grizzly Bears was not exiled by Eater of Virtue: %+v", o)
	}
	attachFixture(t, e, eaterID, flyerID)
	if got := sharedKeywordSet(e, flyerID); !got["Flying"] || !got["Lifelink"] || got["Trample"] || len(got) != 2 {
		t.Fatalf("equipped Shared Zone Flyer has %v, want exactly Flying and Lifelink from the exiled Grizzly Bears", e.Derived(flyerID).Keywords)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: flyerID, From: state.ZBattlefield, To: state.ZGraveyard})
	drainSharedKeywordStack(t, e)
	if o := e.G.Obj(flyerID); o == nil || o.Zone != state.ZExile || o.ExiledWith != eaterID {
		t.Fatalf("precondition: equipped Shared Zone Flyer was not exiled by Eater of Virtue: %+v", o)
	}
	attachFixture(t, e, eaterID, freshID)
	if d := e.Derived(freshID); d.Power != 3 || d.Toughness != 1 {
		t.Fatalf("precondition: equipped Shared Zone Plain is %d/%d, want 3/1 from +2/+0", d.Power, d.Toughness)
	}
	got := sharedKeywordSet(e, freshID)
	if len(got) != 2 || !got["Flying"] || !got["Lifelink"] {
		t.Fatalf("fresh bearer has %v, want exactly Flying and Lifelink from the two cards exiled with Eater of Virtue",
			e.Derived(freshID).Keywords)
	}
	if !slices.Contains(e.Derived(freshID).Keywords, "Flying") || slices.Contains(e.Derived(freshID).Keywords, "Trample") {
		t.Fatalf("fresh bearer keywords = %v, want Flying and not Trample", e.Derived(freshID).Keywords)
	}
	replayCheck(t, e, cfg)
}

func TestCairnWandererSharesGraveyardKeywords(t *testing.T) {
	t.Parallel()
	cairn := corpusAlternativeCard(t, "Cairn Wanderer")
	flyer := card(t, sharedZoneFlyerSrc)
	e, cfg := sharedKeywordBoard(t, 7, cairn, flyer)
	cairnID := moveByName(t, e, 0, "Cairn Wanderer", state.ZBattlefield)
	flyerID := moveByName(t, e, 0, "Shared Zone Flyer", state.ZGraveyard)
	if o := e.G.Obj(flyerID); o == nil || o.Zone != state.ZGraveyard || !o.Face().IsCreature() {
		t.Fatalf("precondition: Shared Zone Flyer is not a creature card in the graveyard: %+v", o)
	}
	if !hasKeywordNamed(e, cairnID, "Flying") {
		t.Fatalf("Cairn Wanderer has %v, want Flying from the graveyard flyer", e.Derived(cairnID).Keywords)
	}
	if hasKeywordNamed(e, cairnID, "Trample") {
		t.Fatalf("Cairn Wanderer has Trample, but no graveyard card has it: %v", e.Derived(cairnID).Keywords)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: flyerID, From: state.ZGraveyard, To: state.ZExile})
	if hasKeywordNamed(e, cairnID, "Flying") {
		t.Fatalf("Cairn Wanderer kept Flying after the only flying card left the graveyard: %v", e.Derived(cairnID).Keywords)
	}
	replayCheck(t, e, cfg)
}
