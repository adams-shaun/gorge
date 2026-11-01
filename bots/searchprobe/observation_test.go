package searchprobe

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

func observationEngine(t testing.TB, seed uint64) *rules.Engine {
	t.Helper()
	c, ds := cards.ParseBytes("observation-fixture", []byte("Name:Mountain\nTypes:Basic Land Mountain\nOracle:Fixture.\n"))
	if len(ds) != 0 {
		t.Fatal(ds)
	}
	c.Link()
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	deck := make([]*cards.Card, 20)
	for i := range deck {
		deck[i] = c
	}
	e, err := rules.NewHypothetical(rules.Config{Seed: seed, Names: []string{"a", "b"}, Decks: [][]*cards.Card{deck, deck}}, []rules.ChanceDraw{{Bound: 2, Value: 0}})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.AdvanceHypothetical(); err != nil {
		t.Fatal(err)
	}
	return e
}

// Two different hidden deals/RNGs have identical legitimate initial views.
// The collector must neither include their shuffle payloads nor encode hidden
// arena positions in supposedly opaque observed-card identities.
func TestObservationIgnoresHiddenSeedShuffleAndArenaIDs(t *testing.T) {
	a, b := observationEngine(t, 17), observationEngine(t, 83)
	if a.L.Head() == b.L.Head() {
		t.Fatal("fixture needs different hidden histories")
	}
	ca, cb := NewCollector(0), NewCollector(0)
	fa, err := ca.Capture(a, a.L.Events)
	if err != nil {
		t.Fatal(err)
	}
	fb, err := cb.Capture(b, b.L.Events)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fa, fb) {
		aa, _ := json.Marshal(fa)
		bb, _ := json.Marshal(fb)
		t.Fatalf("secret-dependent observation\n%s\n%s", aa, bb)
	}
	if len(fa.Identities) != 7 {
		t.Fatalf("learned %d identities, want only own seven cards", len(fa.Identities))
	}
}

func TestCollectorCaptureReusesRedactionStorageWithoutAliasingFrames(t *testing.T) {
	e := observationEngine(t, 17)
	c := NewCollector(0)
	first, err := c.Capture(e, e.L.Events)
	if err != nil {
		t.Fatal(err)
	}
	wantFirst, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	// Precondition: the projected view must actually carry a pending decision
	// to this seat, or the measured body never runs view.copyDecision and the
	// allocation ceiling below guards nothing but the redaction scratch.
	if first.Decision == nil {
		t.Fatal("fixture projects no pending decision; the decision-copy path is not exercised")
	}

	allocs := testing.AllocsPerRun(100, func() {
		if _, err := c.Capture(e, e.L.Events); err != nil {
			t.Fatal(err)
		}
	})
	// The ceiling covers the whole projection, not only the Collector's
	// redaction scratch this test guards: view.Project runs the viewer's
	// potential-action offer walk (PotentialMana followed by the overbound walk
	// in one derivedMemo generation). 51 is the measured cost with scratch reuse
	// intact. view's TestCopyDecisionRejectsDerefedClone pins copyDecision's source
	// shape separately, because the compiler inlines the pointer form away and
	// an allocation count cannot tell them apart.
	if allocs > 51 {
		t.Fatalf("Capture allocations = %.0f, want <= 51 after scratch reuse", allocs)
	}

	if _, err := c.Capture(e, []events.Event{{Kind: events.Note, Player: 0, Text: "later capture"}}); err != nil {
		t.Fatal(err)
	}
	gotFirst, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotFirst) != string(wantFirst) {
		t.Fatal("later capture mutated an earlier returned frame")
	}
}

func BenchmarkCollectorCaptureRepeated(b *testing.B) {
	e := observationEngine(b, 17)
	c := NewCollector(0)
	if _, err := c.Capture(e, e.L.Events); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.Capture(e, e.L.Events); err != nil {
			b.Fatal(err)
		}
	}
}

func TestObservationDropsChoiceIndicesAndInternalContinuation(t *testing.T) {
	e := observationEngine(t, 17)
	c := NewCollector(0)
	before, err := c.Capture(e, e.L.Events)
	if err != nil {
		t.Fatal(err)
	}
	head := e.L.Head()
	clone := e.Clone()
	clone.Pending().ResumeKind = "unobserved"
	clone.Pending().Rolls = []int32{999}
	d := NewCollector(0)
	after, err := d.Capture(clone, clone.L.Events)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("in-memory continuation reached observation")
	}
	left, err := c.Capture(e, []events.Event{{Kind: events.DecisionMade, Player: 1, Text: "choose:[0]"}})
	if err != nil {
		t.Fatal(err)
	}
	right, err := d.Capture(clone, []events.Event{{Kind: events.DecisionMade, Player: 1, Text: "choose:[19]"}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(left, right) {
		t.Fatal("opponent private option index leaked")
	}
	if e.L.Head() != head {
		t.Fatal("collector mutated source")
	}
}

func TestObservationPrivateLookAndPublicRevealKnowOnlyNamedCards(t *testing.T) {
	e := observationEngine(t, 17)
	lib := e.G.Zone(state.ZLibrary, 1)
	ev := events.Event{Kind: events.Note, Player: 1, From: state.ZLibrary, IDs: []state.ObjID{lib[0], lib[1]}, Text: "looks at the top of the library", Secret: true}
	c := NewCollector(0)
	frame, err := c.Capture(e, []events.Event{ev})
	if err != nil {
		t.Fatal(err)
	}
	if len(frame.Identities) != 7 {
		t.Fatal("opponent private look disclosed cards")
	}
	ev.Secret = false
	frame, err = c.Capture(e, []events.Event{ev})
	if err != nil {
		t.Fatal(err)
	}
	if len(frame.Identities) != 2 || len(frame.Events[0].IDs) != 2 {
		t.Fatal("public reveal failed to introduce exactly two identities")
	}
	// A full-library payload must not add any unseen suffix cards.
	frame, err = c.Capture(e, []events.Event{{Kind: events.LibraryOrder, Player: 0, Secret: true, IDs: e.G.Zone(state.ZLibrary, 0)}})
	if err != nil {
		t.Fatal(err)
	}
	if len(frame.Identities) != 0 || len(frame.Events[0].IDs) != 0 {
		t.Fatal("LibraryOrder disclosed unknown suffix")
	}
}

func TestActionMatchesMeaningNotOptionIndex(t *testing.T) {
	e := observationEngine(t, 17)
	c := NewCollector(0)
	if _, err := c.Capture(e, e.L.Events); err != nil {
		t.Fatal(err)
	}
	id := e.G.Zone(state.ZHand, 0)[0]
	d := &decision.Decision{Player: 0, Kind: decision.KPriority, Min: 1, Max: 1, Options: []decision.Option{
		{Index: 0, Kind: "cast", Obj: id, Mode: "kicked", AltCostIndex: 2},
		{Index: 1, Kind: "pass"},
	}}
	actions, err := c.Actions(d, decision.Intent{Player: 0, Choices: []int{0}})
	if err != nil {
		t.Fatal(err)
	}
	d.Options[0], d.Options[1] = d.Options[1], d.Options[0]
	d.Options[0].Index = 0
	d.Options[1].Index = 1
	in, err := c.Match(d, actions)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in.Choices, []int{1}) {
		t.Fatalf("matched indices %v", in.Choices)
	}
	d.Options[1].Mode = ""
	if _, err := c.Match(d, actions); err == nil {
		t.Fatal("matched wrong cast mode")
	}
	d.Options[1].Mode = "kicked"
	d.Options = append(d.Options, d.Options[1])
	d.Options[2].Index = 2
	if _, err := c.Match(d, actions); err == nil {
		t.Fatal("ambiguous semantic action was silently selected")
	}
}

func TestActionRejectsDifferentDecisionContext(t *testing.T) {
	e := observationEngine(t, 17)
	c := NewCollector(0)
	if _, err := c.Capture(e, e.L.Events); err != nil {
		t.Fatal(err)
	}
	d := &decision.Decision{Player: 0, Kind: decision.KChoose, Min: 1, Max: 1, Options: []decision.Option{{Kind: "yes", Label: "yes"}}}
	a, err := c.Actions(d, decision.Intent{Player: 0, Choices: []int{0}})
	if err != nil {
		t.Fatal(err)
	}
	d.Kind = decision.KModes
	if _, err := c.Match(d, a); err == nil {
		t.Fatal("matched an answer to a different decision kind")
	}
	d.Kind = decision.KChoose
	d.Source = e.G.Zone(state.ZHand, 0)[0]
	if _, err := c.Match(d, a); err == nil {
		t.Fatal("matched an answer to a different decision source")
	}
}

func TestActionMatchesAcrossWorldsAndDistinguishesCopies(t *testing.T) {
	a, b := observationEngine(t, 17), observationEngine(t, 83)
	ca, cb := NewCollector(0), NewCollector(0)
	fa, _ := ca.Capture(a, a.L.Events)
	fb, _ := cb.Capture(b, b.L.Events)
	if !reflect.DeepEqual(fa, fb) {
		t.Fatal("fixture observations differ")
	}
	aa, bb := a.G.Zone(state.ZHand, 0), b.G.Zone(state.ZHand, 0)
	if aa[0] == bb[0] {
		t.Fatal("fixture needs different raw IDs")
	}
	da := &decision.Decision{Player: 0, Source: aa[0], Kind: decision.KChoose, Min: 1, Max: 1, Options: []decision.Option{{Obj: aa[0], Kind: "card", Ability: 3, AltCostIndex: 2}, {Index: 1, Obj: aa[1], Kind: "card", Ability: 3, AltCostIndex: 2}}}
	db := *da
	db.Source = bb[0]
	db.Options = append([]decision.Option(nil), da.Options...)
	db.Options[0].Obj = bb[1]
	db.Options[1].Obj = bb[0]
	actions, err := ca.Actions(da, decision.Intent{Player: 0, Choices: []int{0}})
	if err != nil {
		t.Fatal(err)
	}
	in, err := cb.Match(&db, actions)
	if err != nil || !reflect.DeepEqual(in.Choices, []int{1}) {
		t.Fatalf("cross-world match: %v %v", in, err)
	}
	db.Options[1].Ability++
	if _, err := cb.Match(&db, actions); err == nil {
		t.Fatal("matched wrong ability")
	}
	db.Options[1].Ability--
	db.Options[1].AltCostIndex++
	if _, err := cb.Match(&db, actions); err == nil {
		t.Fatal("matched wrong alternate cost")
	}
}

// A Note's Secret bit decides who learns a card, whatever its text says. Each
// text is one the engine emits with cards in hidden zones.
func TestObservationNoteIdentityFollowsSecret(t *testing.T) {
	for _, tc := range []struct {
		name   string
		secret bool
		text   string
	}{
		{"miracle reveal", false, "reveals Terminus (miracle)"},
		{"cascade exile", false, "cascades, exiling cards from the top of the library"},
		{"beheld hand card", false, "beheld Dragon as a cost"},
		{"id-only reveal", false, ""},
		{"private look", true, "looks at the top two cards of the library"},
		{"private bottom look", true, "looks at the bottom of the library"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := observationEngine(t, 17)
			hidden := e.G.Zone(state.ZLibrary, 1)[0]
			note := events.Event{Kind: events.Note, Player: 1, IDs: []state.ObjID{hidden}, Text: tc.text, Secret: tc.secret}
			for _, seat := range []state.PlayerID{0, 1} {
				frame, err := NewCollector(seat).Capture(e, []events.Event{note})
				if err != nil {
					t.Fatalf("seat %d: the feed stopped: %v", seat, err)
				}
				told := !tc.secret || seat == note.Player
				if len(frame.Events) != 1 {
					t.Fatalf("seat %d: events %+v", seat, frame.Events)
				}
				ev := frame.Events[0]
				named := len(ev.IDs) == 1 && ev.Text == tc.text
				if named != told {
					t.Fatalf("seat %d: told=%v, event %+v", seat, told, ev)
				}
				if !told && (ev.Text != "" || ev.IDs != nil || !ev.Secret) {
					t.Fatalf("seat %d: a look by another seat kept its payload: %+v", seat, ev)
				}
				introduced := false
				for _, identity := range frame.Identities {
					if len(ev.IDs) == 1 && identity.ID == ev.IDs[0] {
						introduced = true
					}
				}
				if introduced != told {
					t.Fatalf("seat %d: told=%v, identities %+v", seat, told, frame.Identities)
				}
			}
		})
	}
}

func TestCollectorReverseReferenceCloneIsIndependent(t *testing.T) {
	e := observationEngine(t, 17)
	c := NewCollector(0)
	frame, err := c.Capture(e, e.L.Events)
	if err != nil {
		t.Fatal(err)
	}
	if len(frame.Identities) == 0 {
		t.Fatal("fixture introduced no identities")
	}
	ref := frame.Identities[0].ID
	id := c.object(ref)
	if id == 0 {
		t.Fatalf("reference %d did not resolve", ref)
	}
	clone := c.clone()
	c.byRef[ref] = 0
	if clone.object(ref) != id {
		t.Fatal("clone aliases reverse reference storage")
	}
	if clone.object(0) != 0 || clone.object(9999) != 0 {
		t.Fatal("invalid observer reference resolved")
	}
}

func TestActionKeepsScalarModeMeaningWithHighlightedSource(t *testing.T) {
	e := observationEngine(t, 17)
	c := NewCollector(0)
	if _, err := c.Capture(e, e.L.Events); err != nil {
		t.Fatal(err)
	}
	id := e.G.Zone(state.ZHand, 0)[0]
	d := &decision.Decision{Player: 0, Kind: decision.KModes, Source: id, Min: 1, Max: 1, Options: []decision.Option{{Kind: "mode", Obj: id, Label: "Pay 1"}, {Index: 1, Kind: "mode", Obj: id, Label: "Don't pay"}}}
	a, err := c.Actions(d, decision.Intent{Player: 0, Choices: []int{1}})
	if err != nil {
		t.Fatal(err)
	}
	d.Options[0], d.Options[1] = d.Options[1], d.Options[0]
	d.Options[0].Index = 0
	d.Options[1].Index = 1
	in, err := c.Match(d, a)
	if err != nil || !reflect.DeepEqual(in.Choices, []int{0}) {
		t.Fatalf("scalar mode match: %v %v", in, err)
	}
}

// An attack on a planeswalker (or battle) and the same creature's attack on
// that permanent's controller are different actions (sb-v1 test-0434: a
// Helpful Hunter could attack B or B's Kaito, and the two keys collided).
func TestActionTellsAPermanentAttackFromAPlayerAttack(t *testing.T) {
	e := observationEngine(t, 17)
	c := NewCollector(0)
	if _, err := c.Capture(e, e.L.Events); err != nil {
		t.Fatal(err)
	}
	hand := e.G.Zone(state.ZHand, 0)
	creature, walker := hand[0], hand[1]
	d := &decision.Decision{Player: 0, Kind: decision.KAttackers, Min: 0, Max: 1, Options: []decision.Option{
		{Index: 0, Kind: "attacker", Obj: creature, Player: 1, Label: "Attack with X at B"},
		{Index: 1, Kind: "attacker", Obj: creature, Player: 1, Battle: walker, Label: "Attack with X at B (planeswalker: W)"},
	}}
	for i := range d.Options {
		actions, err := c.Actions(d, decision.Intent{Player: 0, Choices: []int{i}})
		if err != nil {
			t.Fatal(err)
		}
		in, err := c.Match(d, actions)
		if err != nil {
			t.Fatalf("option %d: %v", i, err)
		}
		if !reflect.DeepEqual(in.Choices, []int{i}) {
			t.Fatalf("option %d matched %v", i, in.Choices)
		}
	}
}
