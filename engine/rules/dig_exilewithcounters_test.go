package rules

// dig_exilewithcounters_test.go pins api:Dig's ExileWithCounters$ (cr-g190):
// a card a Dig moves to exile gets one counter of the named kind there
// (CR 122.1/122.6), on both the primary arm and the DestinationZone2$ rest
// arm, and a Dig without the key leaves the exiled card counterless. The
// carriers are the real corpus cards whose follow-ups read the counter:
// Karn, Scion of Urza (SILVER), Evelyn, the Covetous (COLLECTION), The
// Dragon-Kami Reborn (HATCHING, face down) and Ugin, the Ineffable (no key).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// digExileNamed returns the one exiled object with the given face name, or
// fails. The precondition every counter assertion depends on is that the Dig
// really moved the card to exile.
func digExileNamed(t *testing.T, e *Engine, name string) *state.Object {
	t.Helper()
	var found *state.Object
	for p := state.PlayerID(0); p < state.PlayerID(len(e.G.Players)); p++ {
		for _, id := range e.G.Zone(state.ZExile, p) {
			o := e.G.Obj(id)
			if o != nil && o.Face() != nil && o.Face().Name == name {
				if found != nil {
					t.Fatalf("precondition: %s appears twice in exile", name)
				}
				found = o
			}
		}
	}
	if found == nil {
		t.Fatalf("precondition: the Dig did not exile %s", name)
	}
	return found
}

// digHandHas reports whether name is in p's hand.
func digHandHas(e *Engine, p state.PlayerID, name string) bool {
	for _, id := range e.G.Zone(state.ZHand, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return true
		}
	}
	return false
}

// TestDigExileWithCountersKarnSilver: Karn's +1 takes one of the top two to
// hand and exiles the other through DestinationZone2$ with a silver counter;
// the -1's Card.YouOwn+counters_GE1_SILVER pick then offers that card and
// returns it. The rest arm is the only move in this Dig, so dropping its
// counter call (mutation M1) leaves the -1 with nothing to offer.
func TestDigExileWithCountersKarnSilver(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc := oracleScenario{
		Setup: map[string]oracleSeat{"p0": {
			Battlefield: []string{"Karn, Scion of Urza"},
			LibraryTop:  []string{"Savannah Lions", "Grizzly Bears"},
		}},
		Steps: []oracleStep{
			{Op: "activate", Seat: 0, Card: "p0:Karn, Scion of Urza", AbilityIndex: intp(0)},
			{Op: "resolve", Answers: []oracleAnswer{{Kind: "choose", Pick: []string{"p0:Savannah Lions"}}}},
			// CR 606.3: one loyalty ability per turn, so the -1's offer is
			// read on p0's next turn.
			{Op: "pass_to", Seat: 0, Step: "upkeep", Active: "p0"},
			{Op: "pass_to", Seat: 0, Step: "main1", Active: "p0"},
		},
	}
	fails, transcript, run := runOracleScenario(reg, sc)
	if len(fails) > 0 {
		t.Fatalf("scenario failed: %v\ntranscript:\n%v", fails, transcript)
	}
	e := run.e
	// Precondition: the +1 really split the pair -- the chosen card is in
	// hand and the rest arm exiled the other with a silver counter.
	if !digHandHas(e, 0, "Savannah Lions") {
		t.Fatalf("precondition: Savannah Lions not in p0's hand after the +1")
	}
	grizzly := digExileNamed(t, e, "Grizzly Bears")
	if got := grizzly.Counter("SILVER"); got != 1 {
		t.Fatalf("Grizzly Bears SILVER counters = %d, want 1 (exile zone %v)", got, digExileNamedNames(e))
	}
	// The -1's Card.YouOwn+counters_GE1_SILVER pick then offers that card and
	// returns it.
	var karn state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Karn, Scion of Urza" {
			karn = id
		}
	}
	if karn == 0 {
		t.Fatalf("precondition: Karn is not on the battlefield")
	}
	activate := -1
	for _, o := range e.legalActions(0) {
		if o.Kind == "ability" && o.Obj == karn && o.Ability == 1 {
			activate = o.Index
			break
		}
	}
	if activate < 0 {
		t.Fatalf("Karn's -1 is not offered with a silver-countered card in exile (loyalty %d): %+v", e.G.Obj(karn).Counter("LOYALTY"), e.legalActions(0))
	}
	submitChoices(t, e, activate)
	for i := 0; i < 50; i++ {
		d := e.Pending()
		if d == nil {
			break
		}
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 {
				break
			}
			submitChoicePass(t, e)
			continue
		}
		if d.Kind != decision.KChoose {
			t.Fatalf("unexpected decision while resolving Karn's -1: %+v", d)
		}
		pick := -1
		for _, o := range d.Options {
			if o.Obj == grizzly.ID {
				pick = o.Index
			}
		}
		if pick < 0 {
			t.Fatalf("the silver-countered Grizzly Bears is not offered by Karn's -1: %+v", d.Options)
		}
		submitChoices(t, e, pick)
	}
	if !digHandHas(e, 0, "Grizzly Bears") {
		t.Fatalf("Grizzly Bears did not return from exile: %v", digExileNamedNames(e))
	}
	replayCheck(t, e, run.cfg)
}

// digExileNamedNames lists the exile zone for failure messages.
func digExileNamedNames(e *Engine) []string {
	var out []string
	for p := state.PlayerID(0); p < state.PlayerID(len(e.G.Players)); p++ {
		for _, id := range e.G.Zone(state.ZExile, p) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil {
				out = append(out, o.Face().Name)
			}
		}
	}
	return out
}

// TestDigExileWithCountersEvelynCollection: Evelyn's enters trigger exiles
// each player's top card, and every exiled card carries one collection
// counter. (The follow-up may-play static also reads Card.ExiledByYou, an
// unread predicate; that separate gate is filed rather than masked here.)
func TestDigExileWithCountersEvelynCollection(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc := oracleScenario{
		Setup: map[string]oracleSeat{
			"p0": {Hand: []string{"Evelyn, the Covetous"}, LibraryTop: []string{"Mountain"}},
			"p1": {LibraryTop: []string{"Island"}},
		},
		Steps: []oracleStep{
			{Op: "cast", Seat: 0, Card: "p0:Evelyn, the Covetous", Mana: "CCUBBR"},
			{Op: "resolve"},
		},
	}
	fails, transcript, run := runOracleScenario(reg, sc)
	if len(fails) > 0 {
		t.Fatalf("scenario failed: %v\ntranscript:\n%v", fails, transcript)
	}
	e := run.e
	// Precondition: the trigger exiled exactly the two seeded top cards (one
	// per player), proving the Dig ran before the counter asserts.
	mountain := digExileNamed(t, e, "Mountain")
	island := digExileNamed(t, e, "Island")
	for _, o := range []*state.Object{mountain, island} {
		if got := o.Counter("COLLECTION"); got != 1 {
			t.Errorf("%s COLLECTION counters = %d, want 1", o.Face().Name, got)
		}
	}
	replayCheck(t, e, run.cfg)
}

// TestDigExileWithCountersFaceDownDragonKami: The Dragon-Kami Reborn's
// chapter dig exiles its picked card face down with a hatching counter.
func TestDigExileWithCountersFaceDownDragonKami(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc := oracleScenario{
		Setup: map[string]oracleSeat{"p0": {
			Hand:       []string{"The Dragon-Kami Reborn"},
			LibraryTop: []string{"Grizzly Bears", "Savannah Lions", "Hill Giant", "Forest"},
		}},
		Steps: []oracleStep{
			{Op: "cast", Seat: 0, Card: "p0:The Dragon-Kami Reborn", Mana: "CCG"},
			{Op: "resolve", Answers: []oracleAnswer{{Kind: "choose", Pick: []string{"p0:Grizzly Bears"}}}},
		},
	}
	fails, transcript, run := runOracleScenario(reg, sc)
	if len(fails) > 0 {
		t.Fatalf("scenario failed: %v\ntranscript:\n%v", fails, transcript)
	}
	e := run.e
	o := digExileNamed(t, e, "Grizzly Bears")
	// Precondition: the Dig exiled the picked card FACE DOWN (the same card
	// still carries the counter, Forge's post-move placement).
	if !o.FaceDown {
		t.Fatalf("precondition: Grizzly Bears is not face down in exile")
	}
	if got := o.Counter("HATCHING"); got != 1 {
		t.Errorf("HATCHING counters on the face-down card = %d, want 1", got)
	}
	replayCheck(t, e, run.cfg)
}

// TestDigWithoutExileWithCountersUnchanged: Ugin's +1 exiles face down with
// no ExileWithCounters$, so the card gets no counter at all.
func TestDigWithoutExileWithCountersUnchanged(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc := oracleScenario{
		Setup: map[string]oracleSeat{"p0": {
			Battlefield: []string{"Ugin, the Ineffable"},
			LibraryTop:  []string{"Forest"},
		}},
		Steps: []oracleStep{
			{Op: "activate", Seat: 0, Card: "p0:Ugin, the Ineffable", AbilityIndex: intp(0)},
			{Op: "resolve"},
		},
	}
	fails, transcript, run := runOracleScenario(reg, sc)
	if len(fails) > 0 {
		t.Fatalf("scenario failed: %v\ntranscript:\n%v", fails, transcript)
	}
	e := run.e
	o := digExileNamed(t, e, "Forest")
	// Precondition: the card really is the face-down exile the +1 made.
	if !o.FaceDown {
		t.Fatalf("precondition: Forest is not face down in exile")
	}
	if len(o.Counters) != 0 {
		t.Errorf("Forest carries counters %v, want none (no ExileWithCounters$)", o.Counters)
	}
	replayCheck(t, e, run.cfg)
}

func intp(n int) *int { return &n }
