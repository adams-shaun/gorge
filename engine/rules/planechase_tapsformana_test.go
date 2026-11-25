package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The face-up plane's printed TapsForMana static (CR 901): a
// `T:Mode$ TapsForMana | Static$ True | TriggerZones$ Command` line on a plane
// in ZPlanarDeck, Eloren Wilds' shape. The plane is not a battlefield
// permanent, so the ordinary trigger walk never reaches it: the visit path
// admits it and the zone gate admits its Command spelling, or the planner
// prices mana the tap never brings.
const tapsForManaPlaneScript = "Name:Tap Probe Plane\nManaCost:no cost\nTypes:Plane Probe\n" +
	"T:Mode$ TapsForMana | ValidCard$ Permanent | Execute$ TapReflect | TriggerZones$ Command | Static$ True | TriggerDescription$ Whenever a player taps a permanent for mana, they add one extra mana of a type it made.\n" +
	"SVar:TapReflect:DB$ ManaReflected | ColorOrType$ Type | ReflectProperty$ Produced | Defined$ TriggeredActivator | SpellDescription$ an extra mana of a produced type\n" +
	"Oracle:Whenever a player taps a permanent for mana, they add one extra mana of a type it made.\n"

// landTapsForManaPlaneScript is The Fertile Lands of Saulvinia's filter:
// only lands reflect.
const landTapsForManaPlaneScript = "Name:Land Tap Probe Plane\nManaCost:no cost\nTypes:Plane Probe\n" +
	"T:Mode$ TapsForMana | ValidCard$ Land | Execute$ LandReflect | TriggerZones$ Command | Static$ True | TriggerDescription$ Whenever a player taps a land for mana, they add one extra mana of a type it made.\n" +
	"SVar:LandReflect:DB$ ManaReflected | ColorOrType$ Type | ReflectProperty$ Produced | Defined$ TriggeredActivator | SpellDescription$ an extra mana of a produced type\n" +
	"Oracle:Whenever a player taps a land for mana, they add one extra mana of a type it made.\n"

// tapsPlaneConfig builds a two-seat Config whose seat 0 has a one-card planar
// deck (so its top plane is the face-up current plane at genesis) and whose
// first deck card, first, is a card the test moves onto the battlefield.
func tapsPlaneConfig(t *testing.T, seed uint64, first *cards.Card, planes ...*cards.Card) Config {
	t.Helper()
	build := func(s uint64) Config {
		return Config{Seed: s, Names: []string{"a", "b"},
			Decks: [][]*cards.Card{
				append([]*cards.Card{first}, mountainDeck(t, 39)...),
				mountainDeck(t, 40),
			},
			PlanarDecks: [][]*cards.Card{planes},
		}
	}
	return seatZeroStart(build(seed))
}

// TestPlanechaseTapsForManaExtra pins both halves of a plane's reflected
// TapsForMana extra: the payment plan PRICES one mana of the produced type
// (paymentPlanTapExtraMana) and the real activation BRINGS it (CR 605.3b, the
// immediate triggered mana ability), for the ValidCard$ Permanent and
// ValidCard$ Land spellings both corpus carriers print. Each subtest asserts
// its precondition (the plane is the face-up current plane, the tapped source
// is on the battlefield) so reverting the zone admission turns the pool
// assertion red rather than silently passing.
func TestPlanechaseTapsForManaExtra(t *testing.T) {
	t.Parallel()
	t.Run("PermanentFilterPricesAndBrings", func(t *testing.T) {
		plane := card(t, tapsForManaPlaneScript)
		forest := card(t, forestScript())
		cfg := tapsPlaneConfig(t, 91, forest, plane)
		e := New(cfg)
		e.Advance()
		fid := moveSeededCard(t, e, 0, forest, state.ZBattlefield)
		// Precondition: the probe plane really is seat 0's face-up current
		// plane, and the Forest is on the battlefield.
		if planeID := currentPlaneOfTest(t, e, 0); planeID == 0 || e.G.Obj(planeID).Face().Name != "Tap Probe Plane" {
			t.Fatalf("precondition: no face-up probe plane (got %d)", planeID)
		}
		if o := e.G.Obj(fid); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: Forest not on the battlefield (%v)", o)
		}
		// The priced plan: one mana of the produced type on top of the
		// Forest's own {G}.
		extra, _ := e.paymentPlanTapExtraMana(fid, "G")
		if extra[state.MG] != 1 {
			t.Fatalf("plane's reflected extra in the priced plan = %+v, want one {G}", extra)
		}
		// The real mana: the activation brings the Forest's {G} plus the
		// reflected {G}.
		e.priorityRound()
		activateMana(t, e, fid)
		if got := e.G.Players[0].Pool[state.MG]; got != 2 {
			t.Fatalf("plane + Forest green pool = %d, want 2 (the Forest's {G} + the reflected {G})", got)
		}
		if got := e.G.Players[0].Pool.Total(); got != 2 {
			t.Fatalf("total pool = %d, want 2", got)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("LandFilterExcludesTheRock", func(t *testing.T) {
		plane := card(t, landTapsForManaPlaneScript)
		rock := card(t, "Name:Mana Rock\nTypes:Artifact\n"+
			"A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ tap for colourless mana.\nOracle:x\n")
		cfg := tapsPlaneConfig(t, 92, rock, plane)
		e := New(cfg)
		e.Advance()
		rid := moveSeededCard(t, e, 0, rock, state.ZBattlefield)
		if planeID := currentPlaneOfTest(t, e, 0); planeID == 0 || e.G.Obj(planeID).Face().Name != "Land Tap Probe Plane" {
			t.Fatalf("precondition: no face-up land plane (got %d)", planeID)
		}
		if o := e.G.Obj(rid); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: rock not on the battlefield (%v)", o)
		}
		// ValidCard$ Land does not match the artifact, so nothing is priced.
		extra, _ := e.paymentPlanTapExtraMana(rid, "C")
		if extra != (state.Mana{}) {
			t.Fatalf("land plane priced the artifact's tap: %+v", extra)
		}
		e.priorityRound()
		activateMana(t, e, rid)
		if got := e.G.Players[0].Pool[state.MC]; got != 1 {
			t.Fatalf("rock pool = %d, want 1 only (the plane's ValidCard$ Land excludes the artifact)", got)
		}
		replayCheck(t, e, cfg)
	})
}
