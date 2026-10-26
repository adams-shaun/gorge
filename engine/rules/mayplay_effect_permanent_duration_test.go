package rules

// mayplay_effect_permanent_duration_test.go pins CR 611.2a for an
// Effect-created grant with an explicit Duration$ Permanent whose SOURCE is
// a permanent, not an instant or sorcery: the grant lasts until the game
// ends (or its own ForgetOnMoved$/cast end fires), regardless of where its
// source sits when the Effect resolves.
//
//   - Emrakul, the Exigent Doom: the {3}-exile-from-hand activation's own
//     cost moves the card to exile, so the may-play grant's source is never
//     on the battlefield when it is registered; the "you may cast this card
//     for as long as it remains exiled" half must still work, and the grant
//     must end when the card is cast (the Soul Partition tail).
//   - Valakut Exploration: the landfall dig's may-play grant on the exiled
//     top card is the exiled card's grant ("for as long as it remains
//     exiled"), not the enchantment's, so it survives Valakut Exploration
//     leaving the battlefield — while the exiled card's own non-cast
//     departure (ForgetOnMoved$ Exile) still kills it.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEmrakulExigentDoomMayPlayFromExile pins the exile-and-play half of
// Emrakul's hand activation: the ability exiles the card as its cost, the
// Effect registers its may-play grant from exile (a permanent source), and
// the cast offer must be there on a LATER turn and must survive Emrakul's
// own SpellCast untap trigger. Casting spends the permission: the grant ends
// at the cast (the card left exile), so the card exiled again is not castable.
func TestEmrakulExigentDoomMayPlayFromExile(t *testing.T) {
	t.Parallel()
	const scenario = `{"name":"emrakul-exigent-doom-mayplay-from-exile","cr":["611.2a"],"why":"inline",` +
		`"setup":{"p0":{"hand":["Emrakul, the Exigent Doom"],"battlefield":["Mountain"]}},"steps":[` +
		`{"op":"activate","seat":0,"card":"p0:Emrakul, the Exigent Doom","ability_index":0,"mana":"RRR","targets":["p0:Mountain"]},` +
		`{"op":"resolve"},` +
		// Precondition: the cost moved Emrakul to exile, and the grant is
		// already live this turn.
		`{"op":"mana","seat":0,"mana":"RRRRRRRRRR","expect":[` +
		`{"card":"p0:Emrakul, the Exigent Doom","zone":"exile"},` +
		`{"offered":{"seat":0,"kind":"cast","card":"p0:Emrakul, the Exigent Doom"},"want":true}]},` +
		// Past this turn's cleanup, into P1's next turn and back to P0's.
		`{"op":"pass_to","seat":0,"step":"upkeep","active":"p1"},` +
		`{"op":"pass_to","seat":0,"step":"main1","active":"p0"},` +
		`{"op":"mana","seat":0,"mana":"RRRRRRRRRR","expect":[` +
		`{"offered":{"seat":0,"kind":"cast","card":"p0:Emrakul, the Exigent Doom"},"want":true}]},` +
		`{"op":"cast","seat":0,"card":"p0:Emrakul, the Exigent Doom","mana":"RRRRRRRRRR"},` +
		`{"op":"resolve","expect":[{"card":"p0:Emrakul, the Exigent Doom","zone":"battlefield"}]},` +
		// Cast from exile: the permission is spent (the grant ended when the
		// card left exile); the card exiled again is not castable.
		`{"op":"move","seat":0,"card":"p0:Emrakul, the Exigent Doom","to":"exile"},` +
		`{"op":"mana","seat":0,"mana":"RRRRRRRRRR","expect":[` +
		`{"offered":{"seat":0,"kind":"cast","card":"p0:Emrakul, the Exigent Doom"},"want":false}]}]}`
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(scenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
}

// valakutGrantOn returns the Effect-created may-play grant that still
// remembers id, or nil.
func valakutGrantOn(e *Engine, id state.ObjID) *state.ContinuousEffect {
	for i := range e.continuous {
		ce := &e.continuous[i]
		if ce.MayPlay && objIDIn(ce.Remembered, id) {
			return ce
		}
	}
	return nil
}

// TestValakutExplorationGrantOutlivesSource pins the second half of CR
// 611.2a: Valakut Exploration's landfall dig exiles the top land card and
// grants "you may play it for as long as it remains exiled" — the exiled
// card's grant, not the enchantment's — so the grant stays live and offered
// after Valakut Exploration itself leaves the battlefield. The exiled card's
// OWN non-cast departure still sweeps it (the ForgetOnMoved$ Exile ender).
func TestValakutExplorationGrantOutlivesSource(t *testing.T) {
	t.Parallel()
	reg := choiceCorpusRegistry(t)
	val := choiceCorpusCard(t, "Valakut Exploration")
	e := corpusEngine(t, reg, []*cards.Card{val}, nil)
	findCardObj(t, e, 0, "Valakut Exploration", state.ZBattlefield)
	// The dig's top card: the fixture decks are all Mountains, and a LAND
	// exiled this way is playable (not castable), so put a cheap creature on
	// top of the library and fund its {1}{G} cast first.
	bear := putTopOfLibrary(t, e, choiceCorpusCard(t, "Grizzly Bears"), 0)
	addMana(t, e, 0, "GG")
	// Landfall: the land entering digs the top card to exile and registers
	// the may-play grant on it.
	battlefieldFixture(t, e, 0, "Name:Land\nTypes:Land\nOracle:x\n")
	e.pending = nil
	e.priorityRound()
	passUntilStackEmpty(t, e, 30)
	if o := e.G.Obj(bear); o.Zone != state.ZExile {
		t.Fatalf("precondition: the dug card zone = %v, want exile", o.Zone)
	}
	// The dug CARD: the exile zone also parks the trigger's own ability
	// object after it resolves (CR 608.2m), so skip Card==nil entries.
	var exiledID state.ObjID
	for _, id := range e.G.Zone(state.ZExile, 0) {
		if o := e.G.Obj(id); o != nil && o.Card != nil && o.Zone == state.ZExile {
			if exiledID != 0 {
				t.Fatalf("two cards in exile after the landfall dig: %v", e.G.Zone(state.ZExile, 0))
			}
			exiledID = id
		}
	}
	if exiledID != bear {
		t.Fatalf("precondition: the exile-walk found id %d, want the dug creature %d", exiledID, bear)
	}
	grant := valakutGrantOn(e, exiledID)
	if grant == nil {
		t.Fatalf("precondition: may-play grant does not remember the exiled card %d: %+v", exiledID, e.continuous)
	}
	if grant.Source == 0 {
		t.Fatalf("precondition: grant carries no source: %+v", grant)
	}
	if src := e.G.Obj(grant.Source); src == nil || src.Card == nil || src.Card.Faces[0].Name != "Valakut Exploration" {
		t.Fatalf("precondition: grant source is not Valakut Exploration: %+v", src)
	}
	if !e.continuousLive(grant) {
		t.Fatal("precondition: grant is not live while Valakut Exploration is on the battlefield")
	}

	// Valakut Exploration leaves the battlefield: the grant must not follow
	// its source.
	valID := findCardObj(t, e, 0, "Valakut Exploration", state.ZBattlefield)
	e.pending = nil
	e.emit(events.Event{Kind: events.MoveZone, Obj: valID, From: state.ZBattlefield, To: state.ZGraveyard})
	if o := e.G.Obj(valID); o.Zone != state.ZGraveyard {
		t.Fatalf("Valakut Exploration zone = %v, want graveyard", o.Zone)
	}
	grant = valakutGrantOn(e, exiledID)
	if grant == nil {
		t.Fatal("grant stopped remembering the exiled card when Valakut Exploration left the battlefield")
	}
	if !e.continuousLive(grant) {
		t.Fatal("may-play grant is dead after Valakut Exploration left the battlefield")
	}
	e.priorityRound()
	if !castOffered(e, exiledID) {
		t.Fatalf("cast from exile not offered after Valakut Exploration left: %+v", e.Pending())
	}
	found := false
	for _, off := range e.mayPlaySpellIds(0) {
		if off.id == exiledID {
			found = true
		}
	}
	if !found {
		t.Fatal("mayPlaySpellIds does not offer the exiled card after Valakut Exploration left")
	}

	// The exiled card's own non-cast departure still kills the grant.
	e.pending = nil
	e.emit(events.Event{Kind: events.MoveZone, Obj: exiledID, From: state.ZExile, To: state.ZGraveyard})
	if o := e.G.Obj(exiledID); o.Zone != state.ZGraveyard {
		t.Fatalf("exiled card zone = %v, want graveyard", o.Zone)
	}
	if ce := valakutGrantOn(e, exiledID); ce != nil {
		t.Fatalf("may-play grant still remembers the card that left exile: %+v", ce.Remembered)
	}
	e.priorityRound()
	if castOffered(e, exiledID) {
		t.Fatal("cast from exile still offered after the exiled card left exile")
	}
}
