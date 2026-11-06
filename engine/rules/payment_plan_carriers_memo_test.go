package rules

// The PaymentPlanInterferenceCarriers memo is incremental: an inert event
// tail (no Mutate, HiddenIdentity or MoveZone fold) extends the built list
// over newly created objects instead of rebuilding it, and anything else
// rebuilds. These tests script a game segment and, after every event batch,
// require the memoised list to equal a forced rebuild (the correctness half
// -- a missed invalidation would flip the len>0 gates at
// paid_mana_census.go and cast_window.go), and require a run of ordinary
// events to REUSE the memo's backing array (the reuse half, which fails on
// the per-event key the memo used before).

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// carriersMutateDork is a creature whose printed face carries a TapsForMana
// trigger, so mutating it UNDER a vanilla creature makes the pile a carrier
// through MergedCards, and moving the pile off the battlefield clears it
// again (CR 702.140e).
const carriersMutateDork = "Name:Tap Carrier\nManaCost:2 R\nTypes:Creature Lizard\nPT:2/2\nK:Mutate\n" +
	"T:Mode$ TapsForMana | ValidCard$ Creature.YouCtrl | Execute$ TrigProbeMana\n" +
	"SVar:TrigProbeMana:DB$ Mana | Produced$ G | Amount$ 1\nOracle:x\n"

func TestPaymentPlanCarriersMemoTracksEvents(t *testing.T) {
	t.Parallel()
	landCard := card(t, interferenceIsland)
	dorkCard := card(t, carriersMutateDork)
	e, _, spell := newFixtureDeck(t, 9952, interferenceOneInstant, interferenceIsland, carriersMutateDork)

	s := asPayer(e).Session()
	memo := func() []state.ObjID { return pay.PaymentPlanInterferenceCarriers(asPayer(e)) }
	sameBacking := func(a, b []state.ObjID) bool {
		return len(a) == len(b) && (len(a) == 0 || &a[0] == &b[0])
	}
	// check requires the memoised list to equal a forced rebuild (a fresh
	// build after invalidating the memo fields) and an eventless re-read to
	// reuse the rebuilt backing.
	check := func(label string) {
		t.Helper()
		got := memo()
		s.PaymentPlanCarriersValid = false
		forced := memo()
		if !slices.Equal(got, forced) {
			t.Fatalf("%s: memoised carriers %v != forced rebuild %v", label, got, forced)
		}
		if again := memo(); !sameBacking(forced, again) {
			t.Fatalf("%s: an eventless re-read rebuilt the memo", label)
		}
	}
	contains := func(label string, want state.ObjID) {
		t.Helper()
		if !slices.Contains(memo(), want) {
			t.Fatalf("%s: precondition: carrier list %v does not name object %d", label, memo(), want)
		}
	}
	lacks := func(label string, gone state.ObjID) {
		t.Helper()
		if slices.Contains(memo(), gone) {
			t.Fatalf("%s: precondition: carrier list %v still names departed object %d", label, memo(), gone)
		}
	}

	// The very first read builds; a re-read reuses it.
	first := memo()
	if again := memo(); !sameBacking(first, again) {
		t.Fatal("an eventless re-read rebuilt the fresh memo")
	}

	// An eventless battlefield placement grows Objs with no event: the
	// extend path must pick the new carrier up.
	probe := onBoard(t, e, 0, tapManaTrigger("DB$ Mana | Produced$ G | Amount$ 1", ""))
	if !slices.Contains(memo(), probe) {
		t.Fatalf("precondition: the TapsForMana enchantment %d is not a carrier", probe)
	}
	check("after the eventless placement")

	// Play a land (a real logged move), then cast the fixture instant.
	land := moveSeededCard(t, e, 0, landCard, state.ZHand)
	check("after the land's move to hand")
	addMana(t, e, 0, "")
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("not at priority to play the land: %+v", d)
	}
	played := false
	for _, o := range d.Options {
		if o.Kind == "play_land" && o.Obj == land {
			submitChoices(t, e, o.Index)
			played = true
			break
		}
	}
	if !played {
		t.Fatalf("precondition: no play_land option for the Island: %+v", d.Options)
	}
	if o := e.G.Obj(land); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the played land is zone %v", o)
	}
	check("after the land drop")
	addMana(t, e, 0, "C")
	castSpellObj(t, e, spell)
	check("after the cast and its resolution")

	// Create a token (TokenCreate, no zone move), then activate a mana rock
	// (Tap + ManaAdd through the real offer path).
	bear := putToken(t, e, 0, mutateBearSrc, state.ZBattlefield)
	lacks("after the token", bear) // the vanilla bear is no carrier yet
	check("after the token")
	rock := onBoard(t, e, 0, manaWatchRockSrc)
	addMana(t, e, 0, "")
	opt, ok := gainedManaOption(e, rock)
	if !ok {
		t.Fatalf("precondition: no activate option for the rock: %+v", e.Pending())
	}
	submitChoices(t, e, opt.Index)
	resolveStack(t, e)
	if !e.G.Obj(rock).Tapped {
		t.Fatal("precondition: the rock was not tapped by the activation")
	}
	check("after the mana activation")

	// The reuse half: a run of ordinary events -- the activation's own tail
	// plus raw ManaAdds and a tap -- must keep the built backing. This is
	// the assertion the per-event key fails: every event rebuilt the list.
	idBefore := memo()
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "C", Amount: 1})
	e.emit(events.Event{Kind: events.Tap, Obj: land})
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "G", Amount: 1})
	if got := memo(); !sameBacking(idBefore, got) {
		t.Fatalf("ordinary events (ManaAdd, Tap) rebuilt the carrier memo; want the built backing %d reused", idBefore)
	}

	// Mutate the TapsForMana dork UNDER the vanilla bear: the pile becomes a
	// carrier through its under-card, so the Mutate fold must rebuild.
	dork := moveSeededCard(t, e, 0, dorkCard, state.ZHand)
	addMana(t, e, 0, "RRR") // the mutate cost {2}{R}
	mutateCastOnto(t, e, mutatedCastOption(t, e, dork), bear, false)
	contains("after the mutate", bear)
	check("after the mutate")

	// Move the mutated pile off the battlefield: the MoveZone fold's
	// merged-pile departure clear (CR 702.140e) must rebuild, and the
	// conservative rule keeps every MoveZone in the invalidation set.
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZGraveyard})
	lacks("after the pile's departure", bear)
	check("after the pile's departure")
}
