package rules

// A budgeted Charm's mode bodies carry the mode's own Pawprint$ price (CR
// 700.2i, CR 107.18). The parent Charm reads it -- CharmPawprints through
// effects/charm_modes.go's modePawprint -- as part of the mode CHOICE (CR
// 601.2b), so the mode body's own API compiler must treat the key as known
// and must not loud-degrade it with a replay-visible unread-parameter Note.
// Season of Gathering is the corpus carrier: its {P}{P}{P} Draw mode and
// {P} PutCounter mode both spell out Pawprint$, and the card's root carries
// the budget.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestPawprintModeResolvesWithoutUnreadNote casts the real Season of
// Gathering choosing its 3-{P} Draw mode once and its 1-{P} PutCounter mode
// twice (exactly the 5-{P} budget). Both chosen modes must resolve -- the
// bear bears both counters and the Draw fills the hand -- and no resolution
// may emit the `Draw ignores unread parameter(s) Pawprint$` family of Notes
// for a key the parent Charm already read.
func TestPawprintModeResolvesWithoutUnreadNote(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	season := lookup(t, reg, "Season of Gathering")
	bear := lookup(t, reg, "Grizzly Bears")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{season, bear}, nil)

	bearID := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	if o := e.G.Obj(bearID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Grizzly Bears zone = %+v, want battlefield", o)
	}
	seasonID := state.ObjID(0)
	for _, cand := range e.G.Zone(state.ZHand, 0) {
		if o := e.G.Obj(cand); o != nil && o.Face() != nil && o.Face().Name == "Season of Gathering" {
			seasonID = cand
		}
	}
	if seasonID == 0 {
		seasonID = moveByName(t, e, 0, "Season of Gathering", state.ZHand)
	}
	addMana(t, e, 0, "GGGGGG") // 4 G G

	d := castFixture(t, e, seasonID, -1)
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("expected the cast-time KModes announcement, got %+v", d)
	}
	if d.MaxSum != 5 {
		t.Fatalf("cast-mode ask budget = %d, want 5 (root Pawprint$ 5) -- the budget must stay enforced", d.MaxSum)
	}
	draw, counter := -1, -1
	for _, o := range d.Options {
		switch o.Value {
		case 3:
			draw = o.Index
		case 1:
			counter = o.Index
		}
	}
	if draw < 0 || counter < 0 {
		t.Fatalf("Season of Gathering modes not priced 1 and 3: %+v", d.Options)
	}
	handBefore := len(e.G.Zone(state.ZHand, 0))
	// 3 {P} + 1 {P} + 1 {P} = the whole budget.
	submitChoices(t, e, draw, counter, counter)
	passUntilStackEmpty(t, e, 20)

	if got := e.G.Obj(bearID).Counter("P1P1"); got != 2 {
		t.Fatalf("Grizzly Bears +1/+1 counters = %d, want 2 (the repeated 1-{P} mode must resolve)", got)
	}
	if got, want := len(e.G.Zone(state.ZHand, 0)), handBefore+4; got != want {
		t.Fatalf("seat 0 hand = %d, want %d (the 3-{P} Draw mode draws the bear's power, 4)", got, want)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "Pawprint$") {
			t.Fatalf("resolution emitted a false unread-parameter Note: %q", ev.Text)
		}
	}
	replayCheck(t, e, cfg)
}

// TestPawprintCharmRootStillMeasured guards the census attribution the fix
// depends on: modePawprint's read stays attributed to api:Charm (the root
// budget's shared closure), so the mode APIs' ignored-key entries cannot
// hide a lost Charm read.
func TestPawprintCharmRootStillMeasured(t *testing.T) {
	t.Parallel()
	_, d := measureParamCensus(t, nil)
	if !d.api["Charm"]["Pawprint"] {
		t.Fatal("api:Charm no longer reads Pawprint$ -- the census's attribution of the root budget is lost")
	}
}
