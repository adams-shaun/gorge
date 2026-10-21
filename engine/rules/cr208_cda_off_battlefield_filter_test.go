package rules

// CR 208.2a / 604.3: a characteristic-defining ability that defines power and
// toughness functions in every zone, so a card's power off the battlefield is
// the CDA's value, never its printed star read as 0. Forge applies a
// CharacteristicDefining$ static to its host in every zone but an
// ExcludeZone$ one (StaticAbility.java:345 skips the in-play gate;
// StaticAbilityContinuous.getAffectedCards :1073-1081). The rules-tier seam
// (Engine.matchesSpec) already binds Derived in every zone; these tests pin
// the effects tier, where a resolving Dig's ChangeValid$ is matched.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// cdaRallierEngine is squadRallierEngine with Consuming Aberration and
// Grizzly Bears in seat 0's deck; it mills oppMill cards from seat 1's library
// (Consuming Aberration's CDA counts them) and pins the window to
// [Consuming Aberration, Grizzly Bears, Forest, Forest].
func cdaRallierEngine(t *testing.T, oppMill int) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	squadRallierDigIndex(t)
	rallier := lookup(t, reg, "Squad Rallier")
	ab := lookup(t, reg, "Consuming Aberration")
	bears := lookup(t, reg, "Grizzly Bears")
	forest := lookup(t, reg, "Forest")
	if ab.Faces[0].PT != "*/*" {
		t.Fatalf("Consuming Aberration PT = %q, want */* (the test's CDA assumption)", ab.Faces[0].PT)
	}
	filler := func(n int) []*cards.Card {
		out := make([]*cards.Card, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, forest)
		}
		return out
	}
	cfg := Config{Seed: 11, Tokens: reg.Tokens, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{rallier, ab, bears}, filler(37)...),
			filler(40),
		}}
	cfg = seatZeroStart(cfg)
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	id := findInZones(t, e, 0, "Squad Rallier")
	from := state.ZHand
	for _, cand := range e.G.Zone(state.ZLibrary, 0) {
		if cand == id {
			from = state.ZLibrary
		}
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: from, To: state.ZBattlefield, Player: 0})
	for i := 0; i < oppMill; i++ {
		lib := e.G.Zone(state.ZLibrary, 1)
		e.emit(events.Event{Kind: events.MoveZone, Obj: lib[len(lib)-1], From: state.ZLibrary, To: state.ZGraveyard})
	}
	lib := digReorder(t, e, "Consuming Aberration", "Grizzly Bears")
	e.pending = nil
	e.priorityRound()
	return e, cfg, lib[0], lib[1]
}

// rallierOffered activates Squad Rallier and returns the ids its Dig offers.
func rallierOffered(t *testing.T, e *Engine) map[state.ObjID]bool {
	t.Helper()
	activateSquadRallier(t, e)
	ask := passUntilNonPriority(t, e, 20)
	if ask == nil || ask.Kind != decision.KChoose {
		t.Fatalf("take ask = %+v, want the Dig's KChoose", ask)
	}
	got := map[state.ObjID]bool{}
	for _, opt := range ask.Options {
		if opt.Kind == "dig" {
			got[opt.Obj] = true
		}
	}
	return got
}

// TestCR208_2aDigReadsLibraryCDAPower: Consuming Aberration in the library
// over three cards in the opponent's graveyard has power 3 (its CDA works in
// every zone), so Squad Rallier's "creature card with power 2 or less" does
// not offer it. Grizzly Bears (2/2) stays offered.
func TestCR208_2aDigReadsLibraryCDAPower(t *testing.T) {
	t.Parallel()
	e, cfg, ab, bears := cdaRallierEngine(t, 3)
	if p := e.Power(ab); p != 3 {
		t.Fatalf("precondition: library Consuming Aberration power = %d, want 3 (rules Derived applies the CDA off the battlefield)", p)
	}
	got := rallierOffered(t, e)
	if !got[bears] {
		t.Fatalf("precondition: Grizzly Bears %v not offered: %v", bears, got)
	}
	if got[ab] {
		t.Errorf("CR 208.2a/604.3 seq %d: Squad Rallier offers Consuming Aberration (power 3 over three opposing graveyard cards) as power 2 or less: the effects-tier filter read its printed */* as 0",
			len(e.L.Events))
	}
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 20)
	replayCheck(t, e, cfg)
}

// TestCR208_2aDigCDAGuard: over an empty opposing graveyard the CDA value is 0,
// so Consuming Aberration IS a creature card with power 2 or less. Passes at
// base and with the fix.
func TestCR208_2aDigCDAGuard(t *testing.T) {
	t.Parallel()
	e, cfg, ab, bears := cdaRallierEngine(t, 0)
	got := rallierOffered(t, e)
	if !got[ab] || !got[bears] {
		t.Errorf("CR 208.2a seq %d: offered %v, want both Consuming Aberration %v (power 0) and Grizzly Bears %v", len(e.L.Events), got, ab, bears)
	}
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 20)
	replayCheck(t, e, cfg)
}
