package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// partnerWithFaceTrigger returns the ETB trigger the kw:Partner with expansion
// synthesizes on a face, identified by its Keyword$ tag. A face that carries
// the keyword but no such trigger is the unexpanded state -- the defect this
// ticket fixes -- so the helper lets the test assert the precondition the
// whole test depends on rather than assuming it.
func partnerWithFaceTrigger(c *cards.Card) (cards.Trigger, bool) {
	for _, f := range c.Faces {
		for _, tr := range f.Triggers {
			if tr.Params["Keyword"] == "Partner with" {
				return tr, true
			}
		}
	}
	return cards.Trigger{}, false
}

// TestPartnerWithETBOffersNamedPartnerSearch is the named proof for the
// kw:Partner with expansion, over the real corpus carrier Kamber, the
// Plunderer (K:Partner with:Laurine, the Diversion). CR 702.128's partner-with
// keyword prints real rules text: "When this creature enters, target player
// may put Laurine into their hand from their library, then shuffle." No
// carrier's script prints that ability separately, so the expansion must
// synthesize it; without the expansion Kamber's face carries the keyword as a
// bare, behaviourless label.
//
// The test walks the whole ability:
//  1. PRECONDITION: the corpus Kamber face carries the synthesized
//     Partner-with trigger (fails loudly if the expansion regressed or the
//     corpus pin moved), and the named partner is actually in the library.
//  2. entering Kamber poses the trigger's target-player ask;
//  3. the search asks seat 0 and offers exactly the named partner, not some
//     other library card -- the Card.named<name> filter's whole point,
//     exercised on a name that carries a raw comma;
//  4. taking it moves Laurine to hand and shuffles exactly once (CR 701.23:
//     "then shuffle").
func TestPartnerWithETBOffersNamedPartnerSearch(t *testing.T) {
	reg := searchTestRegistry(t)
	kamberCard := searchCorpusCard(t, reg, "Kamber, the Plunderer")
	if _, ok := partnerWithFaceTrigger(kamberCard); !ok {
		t.Fatal("Kamber carries K:Partner with but no synthesized Partner-with ETB trigger (expansion missing)")
	}
	e, cfg := searchEngine(t, reg, "Kamber, the Plunderer", "Laurine, the Diversion")

	// The named partner must start in seat 0's library, or the search has
	// nothing to offer and every later assertion is vacuous.
	laurine := state.ObjID(0)
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, 0) {
			o := e.G.Obj(id)
			if o != nil && o.Face() != nil && o.Face().Name == "Laurine, the Diversion" {
				laurine = id
			}
		}
	}
	if laurine == 0 || e.G.Obj(laurine).Zone != state.ZLibrary {
		t.Fatalf("fixture: Laurine is not in seat 0's library (%d)", laurine)
	}

	// Enter Kamber (a raw MoveZone fixture ETB): the expansion's trigger
	// queues and, at priority, asks its target player.
	kamber := searchMoveByName(t, e, "Kamber, the Plunderer", state.ZBattlefield)
	if o := e.G.Obj(kamber); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Kamber did not enter the battlefield: %+v", o)
	}
	targetAsk := passUntilNonPriority(t, e, 30)
	if targetAsk.Kind != decision.KTarget {
		t.Fatalf("Partner-with ETB did not ask its target player: %+v", targetAsk)
	}
	playerOpt := -1
	for _, o := range targetAsk.Options {
		if o.Kind == "player" && o.Player == 0 {
			playerOpt = o.Index
		}
	}
	if playerOpt < 0 {
		t.Fatalf("Partner-with target ask offers no player-0 option: %+v", targetAsk.Options)
	}
	start := len(e.L.Events)
	submitChoices(t, e, playerOpt)

	// The search resolves to a KChoose for the targeted player, offering the
	// named partner only.
	searchAsk := passUntilNonPriority(t, e, 30)
	if searchAsk.Kind != decision.KChoose || searchAsk.ResumeKind != "search" {
		t.Fatalf("Partner-with ETB did not pose a search: %+v", searchAsk)
	}
	if searchAsk.Player != 0 {
		t.Fatalf("search decision player = %d, want the targeted seat 0", searchAsk.Player)
	}
	if searchAsk.Min != 0 {
		t.Fatalf("search Min = %d, want 0 (the CR 701.23 may-find)", searchAsk.Min)
	}
	offered := -1
	for _, o := range searchAsk.Options {
		if o.Obj == 0 {
			continue
		}
		if o.Obj != laurine {
			t.Fatalf("search offered %d (%q), want only the named partner %d",
				o.Obj, e.G.Obj(o.Obj).Face().Name, laurine)
		}
		offered = o.Index
	}
	if offered < 0 {
		t.Fatalf("search options carry no card option: %+v", searchAsk.Options)
	}
	submitChoices(t, e, offered)

	// CR 701.23: taking the card, then shuffle -- one shuffle, and the
	// partner is in hand.
	if got := e.G.Obj(laurine).Zone; got != state.ZHand {
		t.Fatalf("Laurine zone after search = %s, want hand", got)
	}
	shuffles := 0
	for _, ev := range e.L.Events[start:] {
		if ev.Kind == events.Shuffle && ev.Player == 0 {
			shuffles++
		}
	}
	if shuffles != 1 {
		t.Fatalf("seat-0 shuffles = %d, want exactly 1 (CR 701.23 \"then shuffle\")", shuffles)
	}
	replayCheck(t, e, cfg)
}
