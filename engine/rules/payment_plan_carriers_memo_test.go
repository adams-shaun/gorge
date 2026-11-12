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

// TestPaymentPlanCarriersMemoRebuildsOnMyriadCleanup pins the second fold
// that runs the merged-pile departure clear: foldMyriadCleanup exiles every
// IsMyriad battlefield object through a DIRECT move() -- no MoveZone event
// in the tail -- and that move drops a mutated pile's under-cards (CR
// 702.140e). A myriad token mutated under during combat therefore loses its
// carrier membership under a MyriadCleanup fold alone, so the tail scan
// must treat that kind as rescanning; a memo that extends over it keeps the
// departed pile live through the len>0 gates.
func TestPaymentPlanCarriersMemoRebuildsOnMyriadCleanup(t *testing.T) {
	t.Parallel()
	dorkCard := card(t, carriersMutateDork)
	e, _, _ := newFixtureDeck(t, 9953, interferenceOneInstant, interferenceIsland, carriersMutateDork)
	s := asPayer(e).Session()
	memo := func() []state.ObjID { return pay.PaymentPlanInterferenceCarriers(asPayer(e)) }
	check := func(label string) {
		t.Helper()
		got := memo()
		s.PaymentPlanCarriersValid = false
		if forced := memo(); !slices.Equal(got, forced) {
			t.Fatalf("%s: memoised carriers %v != forced rebuild %v", label, got, forced)
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

	// A vanilla bearer on the battlefield, then a Myriad token minted from
	// it through the real fold pair (MyriadCopy mints in the untracked
	// ZLibrary state; the follow-up MoveZone brings it in, exactly as
	// engine/effects/myriad.go emits them).
	bear := putToken(t, e, 0, mutateBearSrc, state.ZBattlefield)
	e.emit(events.Event{Kind: events.MyriadCopy, Player: 0, Obj: bear, IDs: []state.ObjID{1}})
	tok := e.G.Objs[len(e.G.Objs)-1].ID
	e.emit(events.Event{Kind: events.MoveZone, Obj: tok, From: state.ZLibrary, To: state.ZBattlefield})
	if o := e.G.Obj(tok); o == nil || o.Zone != state.ZBattlefield || !o.IsMyriad {
		t.Fatalf("precondition: the myriad token %+v is not a battlefield IsMyriad object", o)
	}
	lacks("after the myriad token", tok) // a copy of the vanilla bear carries nothing
	check("after the myriad token")

	// Mutate the TapsForMana dork under the myriad token mid-combat (mutate
	// targets any non-Human creature you control, tokens included): the
	// Mutate fold makes the pile a carrier through its under-card.
	dork := moveSeededCard(t, e, 0, dorkCard, state.ZHand)
	addMana(t, e, 0, "RRR") // the mutate cost {2}{R}
	mutateCastOnto(t, e, mutatedCastOption(t, e, dork), tok, false)
	contains("after the mutate under the token", tok)
	check("after the mutate under the token")

	// End of combat: the MyriadCleanup fold's direct move() runs the
	// merged-pile departure clear under Kind MyriadCleanup -- no MoveZone
	// event reaches the tail -- so this kind alone must rebuild the memo.
	if o := e.G.Obj(tok); o.Zone != state.ZBattlefield || len(o.MergedCards) == 0 || !o.IsMyriad {
		t.Fatalf("precondition: the pile %+v is not a mutated battlefield myriad token", o)
	}
	e.emit(events.Event{Kind: events.MyriadCleanup})
	if o := e.G.Obj(tok); o.Zone != state.ZExile || len(o.MergedCards) != 0 {
		t.Fatalf("precondition: the fold did not exile the mutated pile clear: zone %v, merged %v", o.Zone, o.MergedCards)
	}
	lacks("after the myriad cleanup", tok)
	check("after the myriad cleanup")
}

// TestPaymentPlanCarriersMemoTracksCopyEffects pins that a layer-1 copy of a
// TapsForMana carrier (CR 707.2) is listed, and that the copy fold is in the
// tail rescan set: a ClonePermanent onto an existing non-carrier must list it
// without a forced rebuild, and a clearing ClonePermanent must drop it.
func TestPaymentPlanCarriersMemoTracksCopyEffects(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9954, interferenceOneInstant)
	memo := func() []state.ObjID { return pay.PaymentPlanInterferenceCarriers(asPayer(e)) }
	check := func(label string) {
		t.Helper()
		got := slices.Clone(memo())
		asPayer(e).Session().PaymentPlanCarriersValid = false
		if forced := memo(); !slices.Equal(got, forced) {
			t.Fatalf("%s: memoised carriers %v != forced rebuild %v", label, got, forced)
		}
	}
	bear := onBoard(t, e, 0, blankBearSrc)
	model := onBoardCard(t, e, 1, corpusCard(t, "Forsaken Monument"))
	memo() // build the memo before the copy
	if slices.Contains(memo(), bear) {
		t.Fatalf("precondition: the vanilla bear %d is already a carrier", bear)
	}
	e.emit(events.Event{Kind: events.ClonePermanent, Obj: bear, IDs: []state.ObjID{model}})
	if o := e.G.Obj(bear); o.CopyFace == nil {
		t.Fatal("precondition: the clone left no copy face")
	}
	if !slices.Contains(memo(), bear) {
		t.Fatalf("a copy of Forsaken Monument %d is not listed: %v", bear, memo())
	}
	check("after the copy")
	e.emit(events.Event{Kind: events.ClonePermanent, Obj: bear})
	if o := e.G.Obj(bear); o.CopyFace != nil {
		t.Fatal("precondition: the clearing clone left a copy face")
	}
	if slices.Contains(memo(), bear) {
		t.Fatalf("the cleared copy %d is still listed: %v", bear, memo())
	}
	check("after the clearing copy")
}
