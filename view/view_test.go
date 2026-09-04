package view

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
)

// var _ Chars = (*rules.Engine)(nil) is Ruling F2: rules never imports view,
// so this is the only place the shapes are checked against each other.
var _ Chars = (*rules.Engine)(nil)

// flatChars is a stand-in for the engine: printed characteristics only.
type flatChars struct{ g *state.Game }

func (c flatChars) Power(id state.ObjID) int32 {
	if o := c.g.Obj(id); o != nil && o.Face() != nil {
		return int32(o.Face().Power())
	}
	return 0
}
func (c flatChars) Toughness(id state.ObjID) int32 {
	if o := c.g.Obj(id); o != nil && o.Face() != nil {
		return int32(o.Face().Toughness())
	}
	return 0
}
func (c flatChars) Keywords(id state.ObjID) []string {
	if o := c.g.Obj(id); o != nil && o.Face() != nil {
		return o.Face().Keywords
	}
	return nil
}

// PendingTriggers is not exercised by flatChars-driven tests: they build
// their own board directly, never through an Engine, so there is never
// anything queued. The R3 integration tests below drive a real
// *rules.Engine instead, exactly so PendingTriggers is exercised for real.
func (c flatChars) PendingTriggers() []state.PendingTrigger { return nil }

func fourSeatBoard(t *testing.T) *state.Game {
	t.Helper()
	g := state.NewGame([]string{"alice", "bob", "carol", "dave"})
	c, _ := cards.ParseBytes("b.txt", []byte("Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nK:Trample\nOracle:x\n"))
	c.Link()
	for p := state.PlayerID(0); p < 4; p++ {
		var lib, hand, bf []state.ObjID
		for i := 0; i < 10; i++ {
			lib = append(lib, g.AddObject(c, p).ID)
		}
		for i := 0; i < 3; i++ {
			o := g.AddObject(c, p)
			o.Zone = state.ZHand
			hand = append(hand, o.ID)
		}
		o := g.AddObject(c, p)
		o.Zone = state.ZBattlefield
		bf = append(bf, o.ID)
		g.SetZone(state.ZLibrary, p, lib)
		g.SetZone(state.ZHand, p, hand)
		g.SetZone(state.ZBattlefield, p, bf)
		g.Players[p].Pool[state.MG] = 2
	}
	return g
}

func TestOnlyTheViewerSeesTheirHandAndPool(t *testing.T) {
	g := fourSeatBoard(t)
	for viewer := state.PlayerID(0); viewer < 4; viewer++ {
		v := Project(g, flatChars{g}, viewer, nil)
		for _, pv := range v.Players {
			if pv.ID == viewer {
				if len(pv.Hand) != 3 {
					t.Fatalf("viewer %d cannot see own hand", viewer)
				}
				if pv.Pool["G"] != 2 {
					t.Fatalf("viewer %d cannot see own pool", viewer)
				}
				continue
			}
			if pv.Hand != nil {
				t.Fatalf("viewer %d sees seat %d hand", viewer, pv.ID)
			}
			if pv.Pool != nil {
				t.Fatalf("viewer %d sees seat %d pool", viewer, pv.ID)
			}
			if pv.HandSize != 3 {
				t.Fatalf("hand size should still be public, got %d", pv.HandSize)
			}
		}
	}
}

// TestLibraryContentsNeverAppearInAnyProjection searches the marshalled
// payload for every OTHER seat's library object ids (the negative checks)
// but also, per Task 23 supplement §5, for the viewer's OWN hand ids (the
// positive control): with Go's default "ID": key capitalisation the
// negative checks would pass for ANY payload, proving nothing. The control
// fails loudly if that regresses.
func TestLibraryContentsNeverAppearInAnyProjection(t *testing.T) {
	g := fourSeatBoard(t)
	for viewer := state.PlayerID(0); viewer < 4; viewer++ {
		blob, err := json.Marshal(Project(g, flatChars{g}, viewer, nil))
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range g.Zone(state.ZHand, viewer) {
			if !strings.Contains(string(blob), `"id":`+itoa(int(id))+`,`) {
				t.Fatalf("positive control failed: viewer %d's own hand id %d is not findable in its own payload — the search pattern proves nothing", viewer, id)
			}
		}
		// Every library object id must be absent from the payload. Checking
		// ids rather than a field name catches accidental leaks through any
		// route, including a future field nobody thought about.
		for _, id := range g.Zone(state.ZLibrary, (viewer+1)%4) {
			if strings.Contains(string(blob), `"id":`+itoa(int(id))+`,`) {
				t.Fatalf("viewer %d payload contains library object %d", viewer, id)
			}
		}
	}
}

func TestDecisionIsAttachedOnlyToItsOwner(t *testing.T) {
	g := fourSeatBoard(t)
	d := &decision.Decision{Seq: 3, Player: 2, Kind: decision.KPriority,
		Options: []decision.Option{{Index: 0, Kind: "pass", Label: "Pass"}}}
	for viewer := state.PlayerID(0); viewer < 4; viewer++ {
		v := Project(g, flatChars{g}, viewer, d)
		if viewer == 2 {
			if v.Decision == nil {
				t.Fatal("owner did not receive their decision")
			}
			continue
		}
		if v.Decision != nil {
			t.Fatalf("viewer %d received seat 2's decision", viewer)
		}
	}
}

func TestCardViewUsesDerivedCharacteristics(t *testing.T) {
	g := fourSeatBoard(t)
	id := g.Zone(state.ZBattlefield, 0)[0]
	g.Obj(id).AddCounter("P1P1", 2)
	v := Project(g, boostedChars{flatChars{g}}, 0, nil)
	cv := v.Players[0].Battlefield[0]
	if cv.Power != 4 || cv.Toughness != 4 {
		t.Fatalf("view P/T = %d/%d, want the derived 4/4", cv.Power, cv.Toughness)
	}
	if len(cv.Keywords) == 0 || cv.Keywords[0] != "Trample" {
		t.Fatalf("keywords = %v", cv.Keywords)
	}
}

// boostedChars adds counters, standing in for the engine's layer system.
type boostedChars struct{ flatChars }

func (c boostedChars) Power(id state.ObjID) int32 {
	return c.flatChars.Power(id) + c.g.Obj(id).Counter("P1P1")
}
func (c boostedChars) Toughness(id state.ObjID) int32 {
	return c.flatChars.Toughness(id) + c.g.Obj(id).Counter("P1P1")
}

func TestRedactStripsSecretPayloadsFromOtherSeats(t *testing.T) {
	evs := []events.Event{
		{Seq: 0, Kind: events.Shuffle, Player: 1, IDs: []state.ObjID{4, 5, 6}, Secret: true},
		{Seq: 1, Kind: events.Draw, Player: 1, Obj: 4, Secret: true},
		{Seq: 2, Kind: events.Damage, Player: 1, Amount: 3},
	}
	own := RedactEvents(evs, 1)
	if len(own[0].IDs) != 3 || own[1].Obj != 4 {
		t.Fatal("the owning seat lost its own secret payloads")
	}
	other := RedactEvents(evs, 0)
	if other[0].IDs != nil || other[1].Obj != 0 {
		t.Fatalf("secret payload leaked to another seat: %+v", other[:2])
	}
	if other[0].Kind != events.Shuffle || other[0].Player != 1 {
		t.Fatal("redaction must keep the event's shape, only drop the payload")
	}
	if other[2].Amount != 3 {
		t.Fatal("public events must be untouched")
	}
	// Redaction must not mutate the caller's slice.
	if evs[0].IDs == nil {
		t.Fatal("RedactEvents mutated its input")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// ---------------------------------------------------------------------------
// Supplement §4: Draw before Winner.

// TestWinnerDistinguishesAWinFromADrawFromSeatZero is Task 22 finding 5:
// Winner's zero value is PlayerID(0), a real seat, so Over alone cannot say
// whether nobody won (a draw) or seat 0 did. A JSON null must be the only
// way a client can tell "no winner" from "seat 0 won".
func TestWinnerDistinguishesAWinFromADrawFromSeatZero(t *testing.T) {
	drawn := fourSeatBoard(t)
	drawn.Over, drawn.Draw = true, true
	dv := Project(drawn, flatChars{drawn}, 0, nil)
	if dv.Winner != nil {
		t.Fatalf("Winner = %v, want nil for a draw", *dv.Winner)
	}
	blob, err := json.Marshal(dv)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(blob), `"winner":null`) {
		t.Fatalf("drawn game JSON = %s, want \"winner\":null", blob)
	}
	if !strings.Contains(string(blob), `"draw":true`) {
		t.Fatalf("drawn game JSON = %s, want \"draw\":true", blob)
	}

	wonBySeatZero := fourSeatBoard(t)
	wonBySeatZero.Over, wonBySeatZero.Winner = true, 0
	wv := Project(wonBySeatZero, flatChars{wonBySeatZero}, 0, nil)
	if wv.Winner == nil || *wv.Winner != 0 {
		t.Fatal("seat 0 winning must not be confused with Winner's zero value")
	}
	blob2, err := json.Marshal(wv)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(blob2), `"winner":0`) {
		t.Fatalf("won game JSON = %s, want \"winner\":0", blob2)
	}

	wonBySeatTwo := fourSeatBoard(t)
	wonBySeatTwo.Over, wonBySeatTwo.Winner = true, 2
	wv2 := Project(wonBySeatTwo, flatChars{wonBySeatTwo}, 0, nil)
	if wv2.Winner == nil || *wv2.Winner != 2 {
		t.Fatalf("Winner = %v, want 2", wv2.Winner)
	}
}

// ---------------------------------------------------------------------------
// Supplement §5: the decision-marshal test the brief's prose promises but
// its code omits.

func TestDecisionMarshalOmitsServerOnlyFields(t *testing.T) {
	d := decision.Decision{Seq: 1, Player: 0, Kind: decision.KTarget, Source: 42,
		Options: []decision.Option{{Index: 0, Kind: "permanent", Label: "x", Obj: 7,
			Player: 0, Attacker: 9, AltCostIndex: 3}}}
	blob, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	s := string(blob)
	for _, forbidden := range []string{"source", "Source", "attacker", "Attacker", "alt_cost_index", "AltCostIndex"} {
		if strings.Contains(s, forbidden) {
			t.Fatalf("marshalled decision leaked a server-only field %q: %s", forbidden, s)
		}
	}
	if !strings.Contains(s, `"player":0`) {
		t.Fatalf("marshalled decision missing \"player\":0 for an option about seat 0: %s", s)
	}
}

// ---------------------------------------------------------------------------
// Supplement §7: totality. Project and RedactEvents must never panic.

func TestProjectAndRedactEventsAreTotal(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(t *testing.T)
	}{
		{"a nil game yields a zero view carrying only Viewer", func(t *testing.T) {
			v := Project(nil, flatChars{}, 2, nil)
			if v.Viewer != 2 {
				t.Fatalf("Viewer = %d, want 2", v.Viewer)
			}
			if v.Players != nil || v.Stack != nil || v.Pending != nil || v.Decision != nil {
				t.Fatalf("nil game produced a non-zero view: %+v", v)
			}
		}},
		{"a nil Chars degrades to zero P/T, no keywords, no pending", func(t *testing.T) {
			g := fourSeatBoard(t)
			v := Project(g, nil, 0, nil)
			cv := v.Players[0].Battlefield[0]
			if cv.Power != 0 || cv.Toughness != 0 {
				t.Fatalf("card view with nil Chars = %+v, want zero P/T", cv)
			}
			if cv.Keywords != nil {
				t.Fatalf("card view with nil Chars has keywords: %v", cv.Keywords)
			}
			if v.Pending != nil {
				t.Fatalf("Pending = %v, want none with nil Chars", v.Pending)
			}
		}},
		{"an out-of-range viewer is a spectator: public information only", func(t *testing.T) {
			g := fourSeatBoard(t)
			d := &decision.Decision{Player: 0, Kind: decision.KPriority,
				Options: []decision.Option{{Index: 0, Kind: "pass", Label: "Pass"}}}
			v := Project(g, flatChars{g}, state.PlayerID(99), d)
			for _, pv := range v.Players {
				if pv.Hand != nil {
					t.Fatalf("spectator saw seat %d's hand", pv.ID)
				}
				if pv.Pool != nil {
					t.Fatalf("spectator saw seat %d's pool", pv.ID)
				}
			}
			if v.Decision != nil {
				t.Fatal("spectator received a decision")
			}
		}},
		{"a zone entry whose object no longer exists is skipped, not panicked on", func(t *testing.T) {
			g := fourSeatBoard(t)
			g.SetZone(state.ZBattlefield, 0, append(g.Zone(state.ZBattlefield, 0), state.ObjID(999999)))
			v := Project(g, flatChars{g}, 0, nil)
			if len(v.Players[0].Battlefield) != 1 {
				t.Fatalf("battlefield = %d entries, want the one real object with the dangling id skipped",
					len(v.Players[0].Battlefield))
			}
		}},
		{"an out-of-range Winner projects as nil, not a wraparound seat", func(t *testing.T) {
			g := fourSeatBoard(t)
			g.Over, g.Winner = true, state.PlayerID(200)
			v := Project(g, flatChars{g}, 0, nil)
			if v.Winner != nil {
				t.Fatalf("Winner = %v, want nil for an out-of-range seat", *v.Winner)
			}
		}},
		{"RedactEvents(nil) is an empty, non-nil slice", func(t *testing.T) {
			out := RedactEvents(nil, 0)
			if out == nil {
				t.Fatal("RedactEvents(nil, ...) returned nil, want an empty slice")
			}
			if len(out) != 0 {
				t.Fatalf("len = %d, want 0", len(out))
			}
		}},
	} {
		t.Run(tc.name, tc.run)
	}
}

// ---------------------------------------------------------------------------
// Supplement §10: no aliasing.

// TestProjectDoesNotAliasEngineOrDecisionState mutates the returned view's
// keyword slice and the attached decision's options, then asserts the
// engine's own card face and the caller's own decision are untouched. A
// Seat (Task 25) holds a View in-process; it must not be able to corrupt
// live state through it.
func TestProjectDoesNotAliasEngineOrDecisionState(t *testing.T) {
	g := fourSeatBoard(t)
	id := g.Zone(state.ZBattlefield, 0)[0]
	d := &decision.Decision{Seq: 1, Player: 0, Kind: decision.KPriority,
		Options: []decision.Option{{Index: 0, Kind: "pass", Label: "Pass"}}}
	v := Project(g, flatChars{g}, 0, d)

	cv := v.Players[0].Battlefield[0]
	if len(cv.Keywords) == 0 {
		t.Fatal("test setup: expected the bear's Trample keyword in the view")
	}
	cv.Keywords[0] = "MUTATED"
	if v.Decision == nil {
		t.Fatal("test setup: expected the owner's decision to be attached")
	}
	v.Decision.Options[0].Label = "MUTATED"

	if kw := g.Obj(id).Face().Keywords; len(kw) == 0 || kw[0] != "Trample" {
		t.Fatalf("mutating the view's Keywords corrupted the card's own face: %v", kw)
	}
	if d.Options[0].Label != "Pass" {
		t.Fatalf("mutating the view's Decision corrupted the caller's own decision: %q", d.Options[0].Label)
	}
}

// ---------------------------------------------------------------------------
// Supplement §12: the R3 integration test, driven through a real engine.

// r3Card parses and links a standalone card the same two-step way rules'
// own test fixtures do (rules/turn_test.go's card helper) -- view_test.go
// cannot import that unexported helper, so it is reproduced here.
func r3Card(t *testing.T, src string) *cards.Card {
	t.Helper()
	c, diags := cards.ParseBytes("t.txt", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("diags: %v", diags)
	}
	c.Link()
	return c
}

// r3Filler is deck padding: a basic land, never cast in these tests, whose
// only job is to keep each deck at or above the seven-card opening hand so
// genesis (rules.New) does not deck anyone out.
func r3Filler(t *testing.T, n int) []*cards.Card {
	m := r3Card(t, "Name:Filler\nTypes:Basic Land Mountain\nOracle:x\n")
	out := make([]*cards.Card, n)
	for i := range out {
		out[i] = m
	}
	return out
}

const r3GainerSrc = `Name:R3Gainer
Types:Enchantment
T:Mode$ Phase | Phase$ Upkeep | Execute$ TrigGain | TriggerDescription$ gain 5 life
SVar:TrigGain:DB$ GainLife | LifeAmount$ 5 | Defined$ You
Oracle:x
`

const r3DrainerSrc = `Name:R3Drainer
Types:Enchantment
T:Mode$ Phase | Phase$ Upkeep | Execute$ TrigDrain | TriggerDescription$ lose life equal to your life total
SVar:TrigDrain:DB$ LoseLife | LifeAmount$ X | Defined$ You
SVar:X:Count$YourLifeTotal
Oracle:x
`

const r3OptionalSrc = `Name:R3Almsgiver
Types:Enchantment
T:Mode$ Phase | Phase$ Upkeep | OptionalDecider$ You | Execute$ TrigGain | TriggerDescription$ you may gain 4 life
SVar:TrigGain:DB$ GainLife | LifeAmount$ 4 | Defined$ You
Oracle:x
`

// driveThroughPriority passes priority for whichever seat currently holds
// it, casting each name in toCast (in order) the moment it becomes a legal
// option, until the pending decision stops being a priority decision or the
// game ends. It drives the engine through nothing but New/Advance/Pending/
// Submit -- Task 23 supplement §12 forbids poking e.pendingTriggers.
func driveThroughPriority(t *testing.T, e *rules.Engine, toCast []string) {
	t.Helper()
	for i := 0; i < 500; i++ {
		if e.G.Over {
			t.Fatal("the game ended before reaching the decision under test")
		}
		d := e.Pending()
		if d == nil {
			t.Fatal("nothing pending and the game is not over")
		}
		if d.Kind != decision.KPriority {
			return
		}
		idx := -1
		if len(toCast) > 0 {
			for _, o := range d.Options {
				if o.Kind == "cast" && strings.Contains(o.Label, toCast[0]) {
					idx = o.Index
					break
				}
			}
			if idx >= 0 {
				toCast = toCast[1:]
			}
		}
		if idx < 0 {
			for _, o := range d.Options {
				if o.Kind == "pass" {
					idx = o.Index
					break
				}
			}
		}
		if idx < 0 {
			t.Fatalf("no pass option offered: %+v", d)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}
	t.Fatal("driver did not reach the decision under test in time")
}

func submitChoices(t *testing.T, e *rules.Engine, choices ...int) {
	t.Helper()
	d := e.Pending()
	if d == nil {
		t.Fatal("no decision pending")
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
		t.Fatalf("submit %v: %v", choices, err)
	}
}

// TestR3PendingTriggersAndStackAreObservableThroughAFullEngine is the R3
// integration test: two of the controller's own permanents trigger
// simultaneously off a real upkeep, reached by playing the game forward
// through the engine's public surface (never by poking pendingTriggers
// directly, per §12). It asserts the whole observable chain: every seat's
// Pending lists both triggers, only the controller sees the ordering
// Decision (whose Options line up with Pending by position), and once that
// decision is answered the queue empties and the Stack shows both abilities
// in the chosen order.
func TestR3PendingTriggersAndStackAreObservableThroughAFullEngine(t *testing.T) {
	deck0 := append([]*cards.Card{r3Card(t, r3GainerSrc), r3Card(t, r3DrainerSrc)}, r3Filler(t, 5)...)
	deck1 := r3Filler(t, 7)
	e := rules.New(rules.Config{Seed: 7, Names: []string{"alice", "bob"},
		Decks: [][]*cards.Card{deck0, deck1}})
	// New does not itself run the engine forward to the first decision —
	// rules/turn_test.go's own newSeats helper does the same explicit
	// Advance() immediately after New.
	e.Advance()

	driveThroughPriority(t, e, []string{"R3Gainer", "R3Drainer"})

	d := e.Pending()
	if d == nil || d.Kind != decision.KTriggerOrder {
		t.Fatalf("pending = %+v, want a trigger-order decision", d)
	}
	if d.Player != 0 {
		t.Fatalf("the ordering decision is for player %d, want the controller (0)", d.Player)
	}

	var gainerID, drainerID state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if f := e.G.Obj(id).Face(); f != nil {
			switch f.Name {
			case "R3Gainer":
				gainerID = id
			case "R3Drainer":
				drainerID = id
			}
		}
	}
	if gainerID == 0 || drainerID == 0 {
		t.Fatalf("both enchantments must be on the battlefield: gainer=%d drainer=%d", gainerID, drainerID)
	}

	for viewer := state.PlayerID(0); viewer < 2; viewer++ {
		v := Project(e.G, e, viewer, e.Pending())
		if len(v.Pending) != 2 {
			t.Fatalf("viewer %d: Pending = %d entries, want 2", viewer, len(v.Pending))
		}
		for i, pv := range v.Pending {
			if pv.Controller != 0 {
				t.Errorf("viewer %d: Pending[%d].Controller = %d, want 0 (the actual controller)", viewer, i, pv.Controller)
			}
			if pv.Label == "" {
				t.Errorf("viewer %d: Pending[%d].Label is empty", viewer, i)
			}
		}
		if viewer != 0 {
			if v.Decision != nil {
				t.Fatalf("viewer %d received seat 0's ordering decision", viewer)
			}
			continue
		}
		if v.Decision == nil {
			t.Fatal("the controller did not receive the ordering decision")
		}
		if len(v.Decision.Options) != len(v.Pending) {
			t.Fatalf("Decision has %d options, Pending has %d entries", len(v.Decision.Options), len(v.Pending))
		}
		for i, opt := range v.Decision.Options {
			if opt.Obj != v.Pending[i].Source {
				t.Errorf("Decision.Options[%d].Obj = %d, want Pending[%d].Source = %d",
					i, opt.Obj, i, v.Pending[i].Source)
			}
		}
	}

	// Gainer's option is index 0 (discovery order): choose it first, so it
	// is pushed first, sits at the bottom of the stack, and resolves last.
	submitChoices(t, e, 0, 1)

	if pts := e.PendingTriggers(); len(pts) != 0 {
		t.Fatalf("PendingTriggers = %v, want none after the order was submitted", pts)
	}
	v := Project(e.G, e, 0, e.Pending())
	if len(v.Pending) != 0 {
		t.Fatalf("v.Pending = %v, want none", v.Pending)
	}
	if len(v.Stack) != 2 {
		t.Fatalf("Stack = %d entries, want 2", len(v.Stack))
	}
	want := []struct {
		source state.ObjID
		name   string
	}{{gainerID, "R3Gainer"}, {drainerID, "R3Drainer"}}
	for i, w := range want {
		sv := v.Stack[i]
		if sv.Kind != "ability" {
			t.Errorf("Stack[%d].Kind = %q, want \"ability\"", i, sv.Kind)
		}
		if sv.Source != w.source {
			t.Errorf("Stack[%d].Source = %d, want %d", i, sv.Source, w.source)
		}
		if sv.Name != w.name {
			t.Errorf("Stack[%d].Name = %q, want %q", i, sv.Name, w.name)
		}
	}
}

// TestR3PendingTriggerReportsOptionality is R3's other half: an optional
// trigger (OptionalDecider$ You) must show Optional and a Decider on the
// SAME Pending entry, before its yes/no is even answered.
func TestR3PendingTriggerReportsOptionality(t *testing.T) {
	deck0 := append([]*cards.Card{r3Card(t, r3OptionalSrc)}, r3Filler(t, 6)...)
	deck1 := r3Filler(t, 7)
	e := rules.New(rules.Config{Seed: 3, Names: []string{"alice", "bob"},
		Decks: [][]*cards.Card{deck0, deck1}})
	e.Advance()

	driveThroughPriority(t, e, []string{"R3Almsgiver"})

	d := e.Pending()
	if d == nil || d.Kind != decision.KTriggerOptional {
		t.Fatalf("pending = %+v, want an optional-trigger decision", d)
	}
	v := Project(e.G, e, 0, d)
	if len(v.Pending) != 1 {
		t.Fatalf("Pending = %d entries, want 1", len(v.Pending))
	}
	pv := v.Pending[0]
	if !pv.Optional {
		t.Fatal("Pending[0].Optional = false, want true")
	}
	if pv.Decider == nil || *pv.Decider != 0 {
		t.Fatalf("Pending[0].Decider = %v, want a pointer to the controller (0)", pv.Decider)
	}
	if v.Decision == nil || v.Decision.Player != 0 {
		t.Fatalf("Decision = %+v, want the yes/no question for player 0", v.Decision)
	}
}
