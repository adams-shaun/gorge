package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestMyriadLandscapeActivatePaidByPathOfAncestry pins the farm fshadow
// answer-missing divergence (action:activate, Myriad Landscape, 4-seat
// Commander pod 13102605400): with a B/R commander, an untapped Path of
// Ancestry ("Produced$ Combo ColorIdentity | TriggersWhenSpent$ TrigScry") is
// a real mana source for the payment planner, so Myriad Landscape's
// {2},{T},Sacrifice ability (paid by the Path and a Swamp) is offered at
// priority and Commander's Sphere ({3}) has a payment plan tapping all three
// lands. The scry rider made the planner defer the Path
// ("source:special_production"), which left only the Swamp for the {2}.
func TestMyriadLandscapeActivatePaidByPathOfAncestry(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	strefan := corpusCommander(t, reg, "Strefan, Maurer Progenitor")
	e, cfg := multiSeatIdentityGame(t, 4242, 3, strefan,
		corpusCommander(t, reg, "Path of Ancestry"),
		corpusCommander(t, reg, "Myriad Landscape"),
		corpusCommander(t, reg, "Swamp"),
		corpusCommander(t, reg, "Commander's Sphere"))
	path := moveToBattlefieldByName(t, e, 0, "Path of Ancestry")
	ml := moveToBattlefieldByName(t, e, 0, "Myriad Landscape")
	swamp := moveToBattlefieldByName(t, e, 0, "Swamp")
	for _, id := range []state.ObjID{path, ml, swamp} {
		if e.G.Obj(id).Tapped {
			e.emit(events.Event{Kind: events.Untap, Obj: id})
		}
		if e.G.Obj(id).Tapped {
			t.Fatalf("precondition: object %d is still tapped", id)
		}
	}
	sphere := moveSeeded(t, e, 0, "Name:Commander's Sphere\nManaCost:3\nTypes:Artifact\n", state.ZHand)
	if o := e.G.Obj(sphere); o.Zone != state.ZHand {
		t.Fatalf("precondition: Commander's Sphere is in %s, want hand", o.Zone)
	}
	e.pending = nil
	e.priorityRound()
	d := e.Pending()
	if d == nil {
		t.Fatal("no pending decision")
	}

	// Myriad Landscape's {2},{T},Sac ability is an "ability" option (its {T}
	// mana ability is the separate "activate" one).
	abilities := 0
	for _, o := range d.Options {
		if o.Obj == ml && o.Kind == "ability" {
			abilities++
		}
	}
	if abilities != 1 {
		t.Errorf("Myriad Landscape's ability options = %d, want 1 (Path of Ancestry and the Swamp pay the {2}, the Landscape taps itself)", abilities)
	}

	// Commander's Sphere is cast through a payment plan whose activations
	// tap exactly the three lands.
	want := []state.ObjID{path, ml, swamp}
	slices.Sort(want)
	found := false
	for _, act := range e.EnsurePaymentPlanAlternatives() {
		if act.Cast.Object != sphere {
			continue
		}
		for _, p := range act.Plans {
			got := make([]state.ObjID, 0, len(p.Activations))
			for _, a := range p.Activations {
				got = append(got, a.Source)
			}
			slices.Sort(got)
			if slices.Equal(got, want) {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("no payment plan for Commander's Sphere taps the Path of Ancestry, Myriad Landscape and Swamp")
	}
	replayCheck(t, e, cfg)
}

// multiSeatIdentityGame is colourIdentityGame for n seats: seat 0 holds the
// commander and the named cards over a Mountain deck, every other seat a plain
// Mountain deck. Seat 0 starts.
func multiSeatIdentityGame(t *testing.T, seed uint64, n int, cmdr0 *cards.Card, seat0 ...*cards.Card) (*Engine, Config) {
	t.Helper()
	deck0 := append([]*cards.Card{cmdr0}, seat0...)
	deck0 = append(deck0, mountainDeck(t, 40-len(deck0))...)
	cfg := Config{Seed: seed, Decks: [][]*cards.Card{deck0},
		Commanders: [][]int{{0}}, Format: FormatCommander, StartingLife: 40}
	for i := 1; i < n; i++ {
		cfg.Decks = append(cfg.Decks, mountainDeck(t, 40))
		cfg.Commanders = append(cfg.Commanders, []int{})
	}
	for i := 0; i < n; i++ {
		cfg.Names = append(cfg.Names, string(rune('a'+i)))
	}
	cfg = seatZeroStart(cfg)
	e := New(cfg)
	e.Advance()
	return e, cfg
}

// pathPlanBoard puts an untapped Path of Ancestry on seat 0's battlefield in a
// game of n seats with a Goblin commander (Wort, Boggart Auntie) and a Goblin
// Guide in hand, and returns the planned-cast actions for the Guide.
func pathPlanBoard(t *testing.T, seed uint64, n int) (*Engine, Config, state.ObjID, state.ObjID, []decision.PaymentAction) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e, cfg := multiSeatIdentityGame(t, seed, n, corpusCommander(t, reg, "Wort, Boggart Auntie"),
		corpusCommander(t, reg, "Path of Ancestry"), corpusCommander(t, reg, "Goblin Guide"))
	path := moveToBattlefieldByName(t, e, 0, "Path of Ancestry")
	if !e.G.Obj(path).Tapped {
		t.Fatalf("precondition: Path of Ancestry did not enter tapped")
	}
	e.emit(events.Event{Kind: events.Untap, Obj: path})
	if e.G.Obj(path).Tapped {
		t.Fatalf("precondition: Path of Ancestry is still tapped")
	}
	guide := moveSeededToHand(t, e, 0, "Goblin Guide")
	e.pending = nil
	e.priorityRound()
	var acts []decision.PaymentAction
	for _, a := range e.EnsurePaymentPlanAlternatives() {
		if a.Cast.Object == guide {
			acts = append(acts, a)
		}
	}
	return e, cfg, path, guide, acts
}

// TestPathOfAncestryPlannedCastQueuesScry is the rider half of the
// Path-as-payment-source fix: Goblin Guide ({R}, a Goblin like the Wort
// commander) is cast through a payment plan that taps the Path, and the plan's
// spend still fires the TriggersWhenSpent$ scry (a KArrange ask), as a manual
// tap does.
func TestPathOfAncestryPlannedCastQueuesScry(t *testing.T) {
	t.Parallel()
	e, _, path, guide, acts := pathPlanBoard(t, 4243, 3)
	var plan *decision.PaymentPlan
	var actID string
	for _, a := range acts {
		for i := range a.Plans {
			if slices.Equal(tappedBy(a.Plans[i]), []state.ObjID{path}) {
				plan, actID = &a.Plans[i], a.ID
			}
		}
	}
	if plan == nil {
		t.Fatalf("no payment plan for Goblin Guide taps the Path of Ancestry: %+v", acts)
	}
	d := e.Pending()
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player,
		Payment: &decision.PaymentSelection{ActionID: actID, Plan: *plan}}); err != nil {
		t.Fatalf("Submit plan: %v", err)
	}
	if !e.G.Obj(path).Tapped {
		t.Fatal("the plan did not tap the Path of Ancestry")
	}
	if o := e.G.Obj(guide); o.Zone != state.ZStack {
		t.Fatalf("Goblin Guide is in %s, want the stack", o.Zone)
	}
	if ask := drainUntilArrange(t, e); ask == nil {
		t.Fatal("the planned spend of Path's mana on a Goblin creature spell queued no scry (KArrange) ask")
	}
}
