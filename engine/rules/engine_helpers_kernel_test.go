package rules

// Test helpers against a REAL engine: a synthetic SA resolves under a kernel probe (e.probe), the ask it poses is
// read off e.Pending, and the answering Submit re-executes the resolution
// from its checkpoint with the answer served by the tape.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// fixtureEngine is a seats-player engine (all-Mountain decks) driven to seat 0's
// first main phase, ready for eventless fixture placement.
func fixtureEngine(t *testing.T, seats int) *Engine {
	t.Helper()
	return tokenFixtureEngine(t, seats, map[string]*cards.Card{})
}

// corpusFixtureEngine is fixtureEngine with the corpus token scripts available.
func corpusFixtureEngine(t *testing.T, seats int) *Engine {
	t.Helper()
	return tokenFixtureEngine(t, seats, searchTestRegistry(t).Tokens)
}

func tokenFixtureEngine(t *testing.T, seats int, tokens map[string]*cards.Card) *Engine {
	t.Helper()
	names := []string{"a", "b", "c", "d"}[:seats]
	decks := make([][]*cards.Card, seats)
	for i := range decks {
		decks[i] = mountainDeck(t, 40)
	}
	e := New(seatZeroStart(Config{Seed: 1, Names: names, Decks: decks, Tokens: tokens}))
	e.Advance()
	toMain1(t, e)
	return e
}

// parseFixtureSA parses one ability line ("SP$ ..." / "DB$ ...") into a linked SA.
func parseFixtureSA(t *testing.T, line string, svars ...string) *cards.SA {
	t.Helper()
	src := "Name:T\nTypes:Sorcery\nA:" + line + "\n"
	for _, s := range svars {
		src += s + "\n"
	}
	return card(t, src+"Oracle:x\n").Faces[0].Abilities[0]
}

// placeCard adds a card eventlessly to p's zone (appended at the end; for a
// library that is the bottom).
func placeCard(t *testing.T, e *Engine, p state.PlayerID, c *cards.Card, z state.Zone) state.ObjID {
	t.Helper()
	if z == state.ZBattlefield {
		return onBoardCard(t, e, p, c)
	}
	o := e.G.AddObject(c, p)
	o.Zone = z
	e.G.SetZone(z, p, append(e.G.Zone(z, p), o.ID))
	e.staticEpoch = -1
	e.activeEpoch = -1
	return o.ID
}

// placeSrc is placeCard for a script string.
func placeSrc(t *testing.T, e *Engine, p state.PlayerID, src string, z state.Zone) state.ObjID {
	t.Helper()
	return placeCard(t, e, p, card(t, src), z)
}

// addToLibrary replaces p's library with n fresh copies of src (top first).
func addToLibrary(t *testing.T, e *Engine, p state.PlayerID, src string, n int) []state.ObjID {
	t.Helper()
	c := card(t, src)
	for _, id := range e.G.Zone(state.ZLibrary, p) {
		e.G.Obj(id).Zone = state.ZExile
		e.G.SetZone(state.ZExile, p, append(e.G.Zone(state.ZExile, p), id))
	}
	ids := make([]state.ObjID, 0, n)
	for i := 0; i < n; i++ {
		o := e.G.AddObject(c, p)
		o.Zone = state.ZLibrary
		ids = append(ids, o.ID)
	}
	e.G.SetZone(state.ZLibrary, p, ids)
	return ids
}

// runProbeSA resolves sa under a kernel probe with a fresh Ctx from mk on every
// (re-)execution. It returns the posed decision, or nil when the resolution
// completed without asking. *last (when non-nil) receives the Ctx of the
// latest execution, so a test can read what the resolution left on it.
func runProbeSA(t *testing.T, e *Engine, sa *cards.SA, mk func() *effects.Ctx, last **effects.Ctx) *decision.Decision {
	t.Helper()
	e.pending = nil
	e.probe(func() {
		c := mk()
		if last != nil {
			*last = c
		}
		effects.Resolve(e, c, sa)
	})
	return e.Pending()
}

// answerAsk submits choices to the pending decision and returns whatever is
// pending afterwards (nil once the probed resolution completed).
func answerAsk(t *testing.T, e *Engine, choices ...int) *decision.Decision {
	t.Helper()
	d := e.Pending()
	if d == nil {
		t.Fatal("answerAsk: no decision pending")
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
		t.Fatalf("submit %v to %s/%s: %v", choices, d.Kind, d.ResumeKind, err)
	}
	return e.Pending()
}

// optionIdxForObj returns the index of d's option carrying obj, failing when absent.
func optionIdxForObj(t *testing.T, d *decision.Decision, obj state.ObjID) int {
	t.Helper()
	for _, o := range d.Options {
		if o.Obj == obj {
			return o.Index
		}
	}
	t.Fatalf("no option for object %d in %+v", obj, d.Options)
	return -1
}

// optionOfKind returns the index of d's option of the given Kind, failing when absent.
func optionOfKind(t *testing.T, d *decision.Decision, kind string) int {
	t.Helper()
	for _, o := range d.Options {
		if o.Kind == kind {
			return o.Index
		}
	}
	t.Fatalf("no %q option in %+v", kind, d.Options)
	return -1
}

// optionIdxWithLabel returns the index of d's option labelled label, failing when absent.
func optionIdxWithLabel(t *testing.T, d *decision.Decision, label string) int {
	t.Helper()
	for _, o := range d.Options {
		if o.Label == label {
			return o.Index
		}
	}
	t.Fatalf("no option labelled %q in %+v", label, d.Options)
	return -1
}

// eventsSince returns the events logged from index start on.
func eventsSince(e *Engine, start int) []events.Event {
	return append([]events.Event(nil), e.L.Events[start:]...)
}

// countEventKind counts the events of kind in evs.
func countEventKind(evs []events.Event, kind events.Kind) int {
	n := 0
	for _, ev := range evs {
		if ev.Kind == kind {
			n++
		}
	}
	return n
}

// corpusCardNamed looks a corpus card up by name (skipping without .cards).
func corpusCardNamed(t *testing.T, name string) *cards.Card {
	t.Helper()
	return searchCorpusCard(t, searchTestRegistry(t), name)
}

// svarSA resolves a corpus card's named SVar into its SA.
func svarSA(t *testing.T, c *cards.Card, name string) *cards.SA {
	t.Helper()
	sa := cards.ResolveSVar(c.Faces[0].SVars, name)
	if sa == nil {
		t.Fatalf("%s has no compiled %s", c.Faces[0].Name, name)
	}
	return sa
}

// setHandSrcs replaces p's hand with fresh cards from srcs (in order), the
// previous hand set aside in exile.
func setHandSrcs(t *testing.T, e *Engine, p state.PlayerID, srcs ...string) []state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(state.ZHand, p) {
		e.G.Obj(id).Zone = state.ZExile
		e.G.SetZone(state.ZExile, p, append(e.G.Zone(state.ZExile, p), id))
	}
	e.G.SetZone(state.ZHand, p, nil)
	ids := make([]state.ObjID, 0, len(srcs))
	for _, s := range srcs {
		ids = append(ids, placeSrc(t, e, p, s, state.ZHand))
	}
	return ids
}

// vanillaCreatureSrc is a vanilla 1/1 creature script named name.
func vanillaCreatureSrc(name string) string {
	return "Name:" + name + "\nTypes:Creature\nPT:1/1\nOracle:x\n"
}

// objectInZone reports whether id sits in p's zone z.
func objectInZone(e *Engine, z state.Zone, p state.PlayerID, id state.ObjID) bool {
	for _, v := range e.G.Zone(z, p) {
		if v == id {
			return true
		}
	}
	return false
}

// setLibrarySrcs replaces p's library with fresh cards from srcs (top first),
// the previous library set aside in exile.
func setLibrarySrcs(t *testing.T, e *Engine, p state.PlayerID, srcs ...string) []state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(state.ZLibrary, p) {
		e.G.Obj(id).Zone = state.ZExile
		e.G.SetZone(state.ZExile, p, append(e.G.Zone(state.ZExile, p), id))
	}
	e.G.SetZone(state.ZLibrary, p, nil)
	ids := make([]state.ObjID, 0, len(srcs))
	for _, s := range srcs {
		ids = append(ids, placeSrc(t, e, p, s, state.ZLibrary))
	}
	return ids
}

// requireOfferSet asserts d offers exactly want's objects, in order.
func requireOfferSet(t *testing.T, d *decision.Decision, want ...state.ObjID) {
	t.Helper()
	got := make([]state.ObjID, 0, len(d.Options))
	for _, o := range d.Options {
		got = append(got, o.Obj)
	}
	if len(got) != len(want) {
		t.Fatalf("options = %v, want exactly %v", got, want)
	}
	for i, id := range want {
		if got[i] != id {
			t.Fatalf("options = %v, want %v (slot %d differs)", got, want, i)
		}
	}
}

// requireZone asserts id sits in zone z.
func requireZone(t *testing.T, e *Engine, id state.ObjID, z state.Zone) {
	t.Helper()
	if got := e.G.Obj(id).Zone; got != z {
		t.Fatalf("object %d (%s) in %v, want %v", id, e.G.Obj(id).Face().Name, got, z)
	}
}

// phaseOutActivateOffered reports whether p's legal actions include an activation of
// obj.
func phaseOutActivateOffered(e *Engine, p state.PlayerID, obj state.ObjID) bool {
	for _, o := range e.legalActions(p) {
		if o.Kind == "activate" && o.Obj == obj {
			return true
		}
	}
	return false
}

// mayPlayBase is a 2-seat engine at seat 0's turn-1 main phase, hands
// emptied, so a fixture can place a granting permanent on the battlefield
// and a chosen number of lands in the hand.
func mayPlayBase(t *testing.T) *Engine {
	t.Helper()
	e := New(Config{Seed: 1, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
	for p := state.PlayerID(0); p < 2; p++ {
		e.G.SetZone(state.ZHand, p, nil)
		e.G.SetZone(state.ZGraveyard, p, nil)
	}
	e.G.Step = state.StepMain1
	e.G.Active, e.G.Priority = 0, 0
	e.G.Turn = 1
	return e
}

// adventureOption returns the cast option for id whose Mode is exactly mode,
// or nil when none is offered.
func adventureOption(t *testing.T, e *Engine, id state.ObjID, mode string) *decision.Option {
	t.Helper()
	opts := castOptions(t, e)
	for i := range opts {
		o := &opts[i]
		if o.Obj == id && o.Mode == mode {
			return o
		}
	}
	return nil
}

// battlefieldObjIDByName returns the id of seat 0's battlefield object named
// name, or 0.
func battlefieldObjIDByName(e *Engine, name string) state.ObjID {
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	return 0
}

// altFaceOption returns the cast option for id whose Mode is exactly mode, or
// nil when none is offered. It reads e.legalActions directly rather than the
// pending snapshot, so it can be called at any point in a transaction.
func altFaceOption(e *Engine, id state.ObjID, mode string) *decision.Option {
	for _, o := range e.legalActions(0) {
		if o.Kind == "cast" && o.Obj == id && o.Mode == mode {
			cp := o
			return &cp
		}
	}
	return nil
}

// altCastOptions returns the alternative-cost cast options legalActions
// offered for id (AltCostIndex > 0; the paid cast carries the zero value).
func altCastOptions(opts []decision.Option, id state.ObjID) []decision.Option {
	var out []decision.Option
	for _, o := range opts {
		if o.Kind == "cast" && o.Obj == id && o.AltCostIndex > 0 {
			out = append(out, o)
		}
	}
	return out
}

// effectNoteTexts returns the Note texts the log carries naming an unimplemented
// AnimateAll parameter.
func effectNoteTexts(e *Engine) []string {
	var out []string
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note {
			out = append(out, ev.Text)
		}
	}
	return out
}

// scryDecision casts arrangeSrc and returns the pending KArrange decision.
func scryDecision(t *testing.T, e *Engine, id state.ObjID) *decision.Decision {
	t.Helper()
	d := castFixture(t, e, id, -1)
	if d == nil || d.Kind != decision.KArrange {
		t.Fatalf("expected a pending KArrange decision, got %+v", d)
	}
	return d
}

// attachActivateResolve activates obj's ability idx, answers every
// non-priority ask (the equip target) with its first option, and resolves
// the stack.
func attachActivateResolve(t *testing.T, e *Engine, obj state.ObjID, idx int) {
	t.Helper()
	submitChoices(t, e, abilityOption(t, e, obj, idx).Index)
	// CR 602.2a: the ability is on the stack before its target is chosen, so
	// the loop runs until the activation's decisions are answered, not until
	// the stack is non-empty.
	for i := 0; i < 5; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack %v after activating %d, want the ability", e.G.Stack, obj)
	}
	passUntilStackEmpty(t, e, 20)
}

// replicateOption returns the "cast" option casting id in the given mode, or
// fails when it is absent.
func replicateOption(t *testing.T, e *Engine, id state.ObjID, mode string) decision.Option {
	t.Helper()
	d := e.Pending()
	if d == nil {
		t.Fatal("no decision pending")
	}
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id && o.Mode == mode {
			return o
		}
	}
	t.Fatalf("no %q cast option for %d: %+v", mode, id, d.Options)
	return decision.Option{}
}

func bargainHasCastMode(e *Engine, id state.ObjID, mode string) bool {
	d := e.Pending()
	if d == nil {
		return false
	}
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id && o.Mode == mode {
			return true
		}
	}
	return false
}

// countTokenCreates counts the TokenCreate events minted for player p.
func countTokenCreates(e *Engine, p state.PlayerID) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.TokenCreate && ev.Player == p {
			n++
		}
	}
	return n
}

// castOffered reports whether the pending priority decision offers a
// "cast" option naming id.
func castOffered(e *Engine, id state.ObjID) bool {
	d := e.Pending()
	if d == nil {
		return false
	}
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			return true
		}
	}
	return false
}
