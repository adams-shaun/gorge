package view

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/replay"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
)

// The round-2 review's defect (merge c7f5485, Gitaxian Probe): a Look$ True
// RevealHand used to emit its look as a PUBLIC Note carrying the target's
// whole hand, so every seat and every spectator — live and on replay —
// learned the hand. These tests pin the fix end to end on the REAL corpus
// card driven through a real 3-seat engine: the activator's own copy of the
// Secret look Note carries the hand, every other seat's and a Public
// spectator's copy is stripped to its shape, an Omniscient spectator keeps
// it (their visibility already includes every hand), and the looker's
// transcript line names the target and the cards while nobody else's does.

// lookSeat is one (viewer, visibility) expectation row.
type lookSeat struct {
	name    string
	viewer  state.PlayerID
	vis     Visibility
	wantIDs bool
}

// driveProbeLook casts the real corpus Gitaxian Probe from seat 0's hand at
// seat 1 (paying its {U/P} with 2 life — the pool is empty) and stops as
// soon as the Secret look Note is in the log. Genesis shuffles every deck,
// so the seed search runs until one deals the probe into seat 0's opening
// hand AND Force of Will + Brainstorm into seat 1's (so the looker's
// transcript line names real, non-filler cards). Returns the engine, the
// look Note, and seat 1's hand ids as they were looked at.
func probeDecks(t *testing.T) [][]*cards.Card {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	probe, ok := reg.Lookup("Gitaxian Probe")
	if !ok {
		t.Skipf("corpus missing card %q", "Gitaxian Probe")
	}
	fow, okF := reg.Lookup("Force of Will")
	brain, okB := reg.Lookup("Brainstorm")
	if !okF || !okB {
		t.Skipf("corpus missing Force of Will / Brainstorm")
	}
	return [][]*cards.Card{
		append([]*cards.Card{probe}, r3Filler(t, 39)...),
		append([]*cards.Card{fow, brain}, r3Filler(t, 38)...),
		r3Filler(t, 40),
	}
}

// probeNames is the fixture roster; the looker's transcript line is asserted
// against these display names.
var probeNames = []string{"Activator", "Target", "Watcher"}

func driveProbeLook(t *testing.T) (*rules.Engine, events.Event, []state.ObjID) {
	t.Helper()
	decks := probeDecks(t)

	var e *rules.Engine
	for seed := uint64(1); seed < 5000; seed++ {
		e = rules.New(rules.Config{Seed: seed, Names: probeNames, Decks: decks})
		if !handHas(e, 0, "Gitaxian Probe") || !handHas(e, 1, "Force of Will") || !handHas(e, 1, "Brainstorm") {
			e = nil
			continue
		}
		break
	}
	if e == nil {
		t.Fatal("no seed in 1..5000 deals the fixture hands")
	}
	e.Advance()

	findLook := func() *events.Event {
		for i := range e.L.Events {
			ev := &e.L.Events[i]
			if ev.Kind == events.Note && ev.Secret && ev.From == state.ZHand {
				return ev
			}
		}
		return nil
	}
	for i := 0; i < 500; i++ {
		if look := findLook(); look != nil {
			hand := append([]state.ObjID(nil), e.G.Zone(state.ZHand, 1)...)
			return e, *look, hand
		}
		d := e.Pending()
		if d == nil {
			t.Fatal("no pending decision and no look Note in the log")
		}
		idx := -1
		switch d.Kind {
		case decision.KPriority:
			if d.Player == 0 {
				for _, o := range d.Options {
					if o.Kind == "cast" && strings.Contains(o.Label, "Gitaxian Probe") {
						idx = o.Index
						break
					}
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
		case decision.KTarget:
			for _, o := range d.Options {
				if o.Kind == "player" && o.Player == 1 {
					idx = o.Index
					break
				}
			}
		case decision.KChoose:
			if strings.Contains(d.Prompt, "how to pay") {
				// the probe's {U/P}: an empty pool offers only the life half
				for _, o := range d.Options {
					if o.Kind == "pay_life" {
						idx = o.Index
						break
					}
				}
			} else if strings.HasPrefix(d.Prompt, "You look at") {
				// the bare look's look_ack: the looker clicks Continue on the
				// "You look at ..." modal (d8047da4 made the bare look an ask;
				// the driver predates it and stalled on the unhandled kind)
				for _, o := range d.Options {
					if o.Kind == "yes" && o.Label == "Continue" {
						idx = o.Index
						break
					}
				}
			} else if strings.Contains(d.Prompt, "hand-size limit") {
				// a later seat's cleanup discard: discard anything
				idx = 0
			}
		}
		if idx < 0 {
			t.Fatalf("driver stuck on an unexpected decision: kind=%v prompt=%q options=%+v", d.Kind, d.Prompt, d.Options)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}
	t.Fatal("driver never reached the look")
	return nil, events.Event{}, nil
}

// handHas reports whether seat p's hand holds a card whose face name is name.
func handHas(e *rules.Engine, p state.PlayerID, name string) bool {
	for _, id := range e.G.Zone(state.ZHand, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return true
		}
	}
	return false
}

// TestGitaxianProbeLookStaysPrivateFromEveryOtherViewer is the brief's own
// redaction test: 3 seats, the real probe. The activator sees the names;
// the other seat, a public spectator (and a plain seat redaction of either)
// get no ids/names; an Omniscient spectator keeps the payload their
// visibility already allows (they see every hand).
func TestGitaxianProbeLookStaysPrivateFromEveryOtherViewer(t *testing.T) {
	e, look, hand := driveProbeLook(t)

	if look.Player != 0 || !look.Secret || look.From != state.ZHand || look.Text != "" {
		t.Fatalf("look Note = %+v, want a Secret, text-free hand look owned by the activator", look)
	}
	if len(hand) < 3 || !slices.Contains(hand, look.IDs[0]) {
		t.Fatalf("fixture moved: seat 1's hand %v does not cover the looked ids %v", hand, look.IDs)
	}
	if !slices.Equal(look.IDs, hand) {
		t.Fatalf("look ids = %v, want the WHOLE hand %v", look.IDs, hand)
	}
	// The leak's exact shape must be gone: no PUBLIC Note anywhere in the
	// log carries any of the target's hand ids.
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && !ev.Secret {
			for _, id := range ev.IDs {
				if slices.Contains(hand, id) {
					t.Fatalf("a public Note still names a hand card: %+v", ev)
				}
			}
		}
	}

	seats := []lookSeat{
		{"the activator", 0, Seat, true},
		{"the target", 1, Seat, false},
		{"the third seat", 2, Seat, false},
		{"a public spectator", NoSeat, Public, false},
		{"an omniscient spectator", 2, Omniscient, true},
	}
	for _, s := range seats {
		r := RedactEventFor(e.G, look, s.viewer, s.vis)
		if got := len(r.IDs) > 0; got != s.wantIDs {
			t.Fatalf("%s: look payload ids = %v, wantIDs=%v", s.name, r.IDs, s.wantIDs)
		}
		if !r.Secret || r.Kind != events.Note || r.Player != 0 || r.From != state.ZHand {
			t.Fatalf("%s: redacted look = %+v, want the Secret Note's shape preserved", s.name, r)
		}
		if s.wantIDs && !slices.Equal(r.IDs, hand) {
			t.Fatalf("%s: look ids = %v, want the whole hand", s.name, r.IDs)
		}
		if !s.wantIDs && (r.Text != "" || r.Obj != 0) {
			t.Fatalf("%s: a stripped look kept payload fields: %+v", s.name, r)
		}
	}
}

// TestGitaxianProbeLookDescribeLines pins the transcript: the looker's line
// reads "<activator> looks at <target>'s hand" and names the cards (the
// transcript is the client's only data path for hidden-zone ids in a Note);
// every other viewer's line names the look but no card.
func TestGitaxianProbeLookDescribeLines(t *testing.T) {
	e, look, _ := driveProbeLook(t)

	own := Describe(e.G, look)
	wantPrefix := "Activator looks at Target's hand: "
	if !strings.HasPrefix(own, wantPrefix) {
		t.Fatalf("looker's line = %q, want prefix %q", own, wantPrefix)
	}
	if !strings.Contains(own, "Force of Will #") || !strings.Contains(own, "Brainstorm #") {
		t.Fatalf("looker's line %q does not name the looked-at cards", own)
	}
	for _, s := range []lookSeat{
		{"the target", 1, Seat, false},
		{"the third seat", 2, Seat, false},
		{"a public spectator", NoSeat, Public, false},
	} {
		r := RedactEventFor(e.G, look, s.viewer, s.vis)
		if got := Describe(e.G, r); got != "Activator looks at hidden cards" {
			t.Fatalf("%s's line = %q, want the generic look line", s.name, got)
		}
	}
	omni := Describe(e.G, RedactEventFor(e.G, look, 2, Omniscient))
	if omni != own {
		t.Fatalf("an Omniscient spectator's line = %q, want the looker's own line %q", omni, own)
	}
}

// TestGitaxianProbeGameReplaysAndDescribesIdentically is the replay half of
// the brief's Done-means: the game containing the new Secret look event
// replays byte-identically (replay.Replay's chain compare), and the
// transcript lines the replay produces match the original run's — the look
// line included.
func TestGitaxianProbeGameReplaysAndDescribesIdentically(t *testing.T) {
	e, _, _ := driveProbeLook(t)
	original := make([]string, 0, len(e.L.Events))
	for _, ev := range e.L.Events {
		original = append(original, Describe(e.G, ev))
	}
	r2, err := replay.Replay(e.L, rules.Config{Seed: e.L.Seed, Names: probeNames, Decks: probeDecks(t)})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if r2.L.Head() != e.L.Head() {
		t.Fatalf("replayed chain %s, original %s", r2.L.Head(), e.L.Head())
	}
	for i, ev := range r2.L.Events {
		if got := Describe(r2.G, ev); got != original[i] {
			t.Fatalf("replayed line %d = %q, original %q", i, got, original[i])
		}
	}
}
