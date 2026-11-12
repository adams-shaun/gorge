package rules

// Test helpers against a REAL engine: a synthetic SA resolves under a kernel probe (e.probe), the ask it poses is
// read off e.Pending, and the answering Submit re-executes the resolution
// from its checkpoint with the answer served by the tape.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
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

// indexOfObjOption returns the index of d's option carrying id, or -1 when
// no option does. It does not fail: a test that requires the option to be
// present reads the -1 itself.
func indexOfObjOption(d *decision.Decision, id state.ObjID) int {
	for _, o := range d.Options {
		if o.Obj == id {
			return o.Index
		}
	}
	return -1
}

// optionIndexForKind returns the index of d's option of the given kind, or
// -1 when no option has it.
func optionIndexForKind(d *decision.Decision, kind string) int {
	for _, o := range d.Options {
		if o.Kind == kind {
			return o.Index
		}
	}
	return -1
}

// optionKindOffered reports whether d offers an option of the given kind.
func optionKindOffered(d *decision.Decision, kind string) bool {
	return optionIndexForKind(d, kind) >= 0
}

// offersObjOption reports whether d offers an option carrying id.
func offersObjOption(d *decision.Decision, id state.ObjID) bool {
	return indexOfObjOption(d, id) >= 0
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

// scryDecision casts id and returns the pending KArrange decision.
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

func runFlashZoneScenario(t *testing.T, raw string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleScenarioJSON(reg, []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
}

func logDrawsFor(e *Engine, from int, p state.PlayerID) int {
	n := 0
	for _, ev := range e.L.Events[from:] {
		if ev.Kind == events.Draw && ev.Player == p {
			n++
		}
	}
	return n
}

func runInline(t *testing.T, sc string) OracleResult {
	t.Helper()
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(sc))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
	return res
}

// onHand places a card straight into a player's hand, appended after their
// existing hand in hand-zone order, the same direct-setup stance as onBoard
// for the battlefield. A test that needs a player mid-turn to hold more than
// maxHandSize adds a few of these on top of the opening hand.
func onHand(t *testing.T, e *Engine, p state.PlayerID, src string) state.ObjID {
	t.Helper()
	o := e.G.AddObject(card(t, src), p)
	o.Zone = state.ZHand
	e.G.SetZone(state.ZHand, p, append(e.G.Zone(state.ZHand, p), o.ID))
	return o.ID
}

func cfdPass(t *testing.T, e *Engine, d *decision.Decision) {
	t.Helper()
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
}

// submitBlockerPairsOnly is submitBlockerPairs without the trailing priority
// drain, for fixtures that must inspect the decision the damage step poses
// next.
func submitBlockerPairsOnly(t *testing.T, e *Engine, pairs ...[2]state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("expected a blockers decision, got %+v", d)
	}
	var choices []int
	for _, pr := range pairs {
		idx := -1
		for _, o := range d.Options {
			if o.Obj == pr[1] && o.Attacker == pr[0] {
				idx = o.Index
				break
			}
		}
		if idx < 0 {
			t.Fatalf("no block option pairing attacker %d with blocker %d: %+v", pr[0], pr[1], d.Options)
		}
		choices = append(choices, idx)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
		t.Fatalf("submit blocker pairs: %v", err)
	}
}

// castCommanderOnce casts id from the command zone and drains the stack, but
// does NOT return it to the command zone. Used only by the replay test, where
// every event must be produced by the recorded Intents alone (a fixture
// MoveZone back would not replay).
func castCommanderOnce(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	opt := commanderCastOption(e, id)
	if opt == nil {
		t.Fatalf("no command-zone cast option for commander %d: %+v", id, e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	passUntilStackEmpty(t, e, 30)
}

// counterN returns the number of `kind` counters on o.
func counterN(o *state.Object, kind string) int {
	for i := range o.Counters {
		if o.Counters[i].Kind == kind {
			return int(o.Counters[i].N)
		}
	}
	return 0
}

// handByName returns seat p's card named name, moving it from the library
// into the hand when it was not drawn.
func handByName(t *testing.T, e *Engine, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(state.ZHand, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	return moveByName(t, e, p, name, state.ZHand)
}

// runForEachScenario runs one inline XMage-port scenario and requires a clean
// audit: no fails and an empty replay diff.
func runForEachScenario(t *testing.T, sc string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleAuditScenarioJSON(reg, []byte(sc), true)
	if err != nil || len(res.Fails) != 0 {
		t.Fatalf("scenario: err=%v fails=%v", err, res.Fails)
	}
}

// noUnimplementedCopyPermanent fails when the log carries an "unimplemented
// API" note for the primitive under test.
func noUnimplementedCopyPermanent(t *testing.T, e *Engine) {
	t.Helper()
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "unimplemented API") {
			t.Fatalf("log carries an unimplemented-API note: %q", ev.Text)
		}
	}
}

// battlefieldFixture puts a permanent onto p's battlefield through a real MoveZone
// so every log-head-keyed cache (mana sources, statics) sees it.
func battlefieldFixture(t *testing.T, e *Engine, p state.PlayerID, src string) state.ObjID {
	t.Helper()
	o := e.G.AddObject(card(t, src), p)
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	return o.ID
}

// runRememberScenario runs one oracle-form scenario and fails with its transcript.
func runRememberScenario(t *testing.T, raw string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	var sc oracleScenario
	if err := json.Unmarshal([]byte(raw), &sc); err != nil {
		t.Fatal(err)
	}
	if fails, tr, _ := runOracleScenario(reg, sc); len(fails) > 0 {
		t.Fatalf("%s\n  transcript:\n    %s", strings.Join(fails, "\n"), strings.Join(tr, "\n    "))
	}
}

// miscBoardObj finds the id of the battlefield permanent named name
// controlled by seat p.
func miscBoardObj(t *testing.T, e *Engine, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	t.Fatalf("seat %d's battlefield has no %q", p, name)
	return 0
}

// cr70114TargetFightSA is the fixture fight: the resolving source fights the one
// targeted creature (the TestFightZeroPowerAndOffBattlefieldNoWedge shape).
func cr70114TargetFightSA() *cards.SA {
	return &cards.SA{Kind: "DB", API: "Fight", Params: map[string]string{
		"Defined": "Self", "ValidTgts": "Creature",
		"TargetMin": "0", "TargetMax": "1", "TgtPrompt": "x"}}
}

// revealNotes returns the non-secret Notes carrying ids (the reveal Notes).
func revealNotes(e *Engine) []events.Event {
	var out []events.Event
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && !ev.Secret && len(ev.IDs) > 0 {
			out = append(out, ev)
		}
	}
	return out
}

// mobVerdictVoteMove moves one corpus card from seat p's hand or library to
// zone `to` through a logged MoveZone, then re-asks priority.
func mobVerdictVoteMove(t *testing.T, e *Engine, p state.PlayerID, name string, to state.Zone) state.ObjID {
	t.Helper()
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, p) {
			o := e.G.Obj(id)
			if o != nil && o.Face() != nil && o.Face().Name == name {
				if z != to {
					e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: to})
				}
				e.pending = nil
				e.priorityRound()
				return id
			}
		}
	}
	t.Fatalf("corpus card %q absent from seat %d hand/library", name, p)
	return 0
}

// battlefieldIDOrFail finds a seat-0 battlefield object by face name.
func battlefieldIDOrFail(t *testing.T, e *Engine, name string) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	t.Fatalf("no battlefield object named %q", name)
	return 0
}

func passPriorityOption(t *testing.T, d *decision.Decision) int {
	t.Helper()
	for _, o := range d.Options {
		if o.Kind == "pass" {
			return o.Index
		}
	}
	t.Fatalf("priority decision with no pass option: %+v", d)
	return -1
}

func findBattlefieldCard(t *testing.T, e *Engine, name string) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	t.Fatalf("%s is not on seat 0's battlefield", name)
	return 0
}

// graveCreature creates a card object and places it in p's graveyard (a
// fixture placement, the same shape mayplay_test uses).
func graveCreature(t *testing.T, e *Engine, p state.PlayerID, src string) state.ObjID {
	t.Helper()
	o := e.G.AddObject(card(t, src), p)
	o.Zone = state.ZGraveyard
	e.G.SetZone(state.ZGraveyard, p, append(e.G.Zone(state.ZGraveyard, p), o.ID))
	return o.ID
}

// graveLandOffered reports whether seat p, made the active player at priority,
// is offered play_land for id.
func graveLandOffered(e *Engine, p state.PlayerID, id state.ObjID) bool {
	e.G.Active, e.G.Priority = p, p
	for _, o := range e.legalActions(p) {
		if o.Kind == "play_land" && o.Obj == id {
			return true
		}
	}
	return false
}

// nogAnimates is the face indexes of Nog's two Animate abilities, in order.
func nogAnimates(t *testing.T, e *Engine, nog state.ObjID) []int {
	t.Helper()
	var out []int
	for i, a := range e.G.Obj(nog).Face().Abilities {
		if a.API == "Animate" {
			out = append(out, i)
		}
	}
	if len(out) != 2 {
		t.Fatalf("Nog has %d Animate abilities, want 2", len(out))
	}
	return out
}

// putOnBattlefield seeds a real corpus card directly onto seat p's
// battlefield as test setup (the handEngine/graveCreature direct-seed idiom),
// so a card's own enters-the-battlefield trigger does not have to be driven.
// The caller asserts the landed state.
func putOnBattlefield(t *testing.T, e *Engine, p state.PlayerID, c *cards.Card) state.ObjID {
	t.Helper()
	o := e.G.AddObject(c, p)
	o.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, p, append(e.G.Zone(state.ZBattlefield, p), o.ID))
	return o.ID
}

func fpPlanUses(got PaymentPlanOutcome, id state.ObjID) (decision.PaymentActivation, bool) {
	if got.Plan == nil {
		return decision.PaymentActivation{}, false
	}
	for _, a := range got.Plan.Activations {
		if a.Source == id {
			return a, true
		}
	}
	return decision.PaymentActivation{}, false
}

// findInHand returns the id of the card named name in seat p's hand, or 0.
func findInHand(e *Engine, p state.PlayerID, name string) state.ObjID {
	for _, id := range e.G.Zone(state.ZHand, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	return 0
}

// countVoteReveals counts the deferred "votes for" reveal Notes emitted
// so far (the player ballot defers them until every voter has answered).
func countVoteReveals(e *Engine) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.HasPrefix(ev.Text, "votes for ") {
			n++
		}
	}
	return n
}

// searchMoveByNameSeat is searchMoveByName for any seat: the named card moves from
// hand (or library) to the target zone through a logged MoveZone.
func searchMoveByNameSeat(t *testing.T, e *Engine, p state.PlayerID, name string, to state.Zone) state.ObjID {
	t.Helper()
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, p) {
			o := e.G.Obj(id)
			if o != nil && o.Face() != nil && o.Face().Name == name {
				if z != to {
					e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: to})
				}
				e.pending = nil
				e.priorityRound()
				return id
			}
		}
	}
	t.Fatalf("corpus fixture %q absent from seat %d's hand/library", name, p)
	return 0
}

// addManaQuiet is fundPool for a named seat: it emits ManaAdd events into
// seat p's pool without driving anywhere, then re-asks priority so the
// pending decision reflects the funded pool.
func addManaQuiet(t *testing.T, e *Engine, p state.PlayerID, symbols string) {
	t.Helper()
	for _, r := range symbols {
		e.emit(events.Event{Kind: events.ManaAdd, Player: p, Counter: string(r), Amount: 1})
	}
	e.priorityRound()
}

// runFixtureScenario plays a scenario as RunOracleScenarioJSON does
// (xmageFixture) and fails the test on any scenario failure.
func runFixtureScenario(t *testing.T, raw string) (OracleResult, *oracleRun) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	sc, err := decodeOracleScenario([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	sc.xmageFixture = true
	fails, transcript, run := runOracleScenario(reg, sc)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(transcript, "\n"))
	}
	return OracleResult{Fails: fails, Transcript: transcript, Snapshots: run.snaps}, run
}

func runSetupAttachedScenario(t *testing.T, raw string) (fails, transcript []string, run *oracleRun) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	sc, err := decodeOracleScenario([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	sc.xmageFixture = true
	fails, transcript, run = runOracleScenario(reg, sc)
	return fails, transcript, run
}

func paymentPlanReask(t *testing.T, e *Engine) *decision.Decision {
	t.Helper()
	e.pending = nil
	e.askPriority(0)
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("pending = %#v, want seat-0 priority", d)
	}
	// Payment actions are published lazily: build them
	// for this ask, as an opted-in consumer would.
	e.EnsurePaymentActions()
	return d
}

func paymentPlanTapsSince(e *Engine, from int, id state.ObjID) int {
	n := 0
	for _, ev := range e.L.Events[from:] {
		if ev.Kind == events.Tap && ev.Obj == id {
			n++
		}
	}
	return n
}

func producedManaSince(e *Engine, from int) []string {
	var out []string
	for _, ev := range e.L.Events[from:] {
		if ev.Kind == events.ManaAdd && ev.Amount > 0 {
			for i := int32(0); i < ev.Amount; i++ {
				out = append(out, ev.Counter)
			}
		}
	}
	return out
}

func submitPaymentPlan(t *testing.T, e *Engine, d *decision.Decision, a decision.PaymentAction) {
	t.Helper()
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Payment: &decision.PaymentSelection{ActionID: a.ID, Plan: a.Plans[0]}}); err != nil {
		t.Fatalf("Submit planned cast: %v", err)
	}
}

// castAndReachRevealAsk casts the spell directly (cr601's harness shape: the
// direct proposal bypasses the offer gate, which a poolless seat needs for
// even a {0} cost's offer path) and drives every intermediate decision until
// the may-reveal ask itself is pending.
func castAndReachRevealAsk(t *testing.T, e *Engine, id state.ObjID) *decision.Decision {
	t.Helper()
	e.pending = nil
	e.beginCast(0, decision.Option{Kind: "cast", Obj: id})
	e.Advance()
	for i := 0; i < 200; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no pending decision and no may-reveal ask")
		}
		if d.Kind == decision.KChoose && d.ResumeKind == "reveal_optional" {
			return d
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("unexpected pending decision: %+v", d)
		}
		pass := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				pass = o.Index
				break
			}
		}
		if pass < 0 {
			t.Fatalf("no pass option: %+v", d)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pass}}); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}
	t.Fatal("never reached the may-reveal ask")
	return nil
}

// seekUnlessPay passes every priority decision until a mid-resolution
// unless_pay ask becomes pending, and returns it.
func seekUnlessPay(t *testing.T, e *Engine, limit int) *decision.Decision {
	t.Helper()
	for i := 0; i < limit && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			return nil
		}
		if d.Kind == decision.KModes && d.ResumeKind == "unless_pay" {
			return d
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("unexpected decision kind %v (seat %d) while seeking the unless-pay ask: %+v", d.Kind, d.Player, d)
		}
		castFirst(t, e, "pass")
	}
	return nil
}

// nogOfficer puts Nog on seat 0's battlefield and runs both Animate steps
// ({1}: Scout, then {3}: Officer), returning the engine and Nog's id.
func nogOfficer(t *testing.T) (*Engine, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Nog, Ambitious Cadet")},
		[]*cards.Card{lookup(t, reg, "Humility")})
	nog := moveCorpusCard(t, e, "Nog, Ambitious Cadet", 0, state.ZBattlefield)
	animates := nogAnimates(t, e, nog)
	for n, mana := range []string{"C", "CCC"} {
		addMana(t, e, 0, mana)
		pick := -1
		for _, o := range e.Pending().Options {
			if o.Kind == "ability" && o.Obj == nog && o.Ability == animates[n] {
				pick = o.Index
			}
		}
		if pick < 0 {
			t.Fatalf("Nog's animate #%d not offered: %+v", n, e.Pending().Options)
		}
		submitChoices(t, e, pick)
		passUntilStackEmpty(t, e, 50)
	}
	isOfficer := false
	for _, ty := range e.Chars(nog).Types {
		isOfficer = isOfficer || ty == "Officer"
	}
	if !isOfficer {
		t.Fatalf("precondition: Nog is not an Officer: %v", e.Chars(nog).Types)
	}
	return e, nog
}
