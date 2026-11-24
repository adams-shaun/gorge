package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Recover (CR 702.59) end-to-end tests on a REAL corpus card (Grim
// Harvest, `K:Recover:2 B`). The keyword's trigger lives on the card while
// it is in its owner's graveyard: "When a creature is put into your
// graveyard from the battlefield, you may pay [cost]. If you do, return
// this card from your graveyard to your hand. Otherwise, exile this card."
// The cost is offered on resolution (the unless gate, UnlessSwitched$ True:
// paying CAUSES the return); declining or being unable to pay runs the
// otherwise-exile branch instead.

const recoverBearSrc = "Name:Recover Victim\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// recoverFixture builds a two-seat Main1 game where seat 0 has `name` (a
// corpus Recover card) in its graveyard, a creature it owns on the
// battlefield, and floating mana able to cover a {2}{B}-shaped cost. The
// creature is then killed by an emitted battlefield->graveyard move, so the
// returned engine has just resolved the Recover trigger's unless-pay ask
// (the fixture's caller answers it).
func recoverFixture(t *testing.T, seed uint64, name string) (*Engine, state.ObjID) {
	t.Helper()
	return recoverFixtureLife(t, seed, name, 0)
}

// recoverFixtureLife is recoverFixture with seat 0's life set first when
// life > 0 (the PayLife-cost carriers).
func recoverFixtureLife(t *testing.T, seed uint64, name string, life int32) (*Engine, state.ObjID) {
	t.Helper()
	reg := searchTestRegistry(t)
	rc, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("corpus card %q missing", name)
	}
	e := stealEngine(t, seed)
	spell := e.G.AddObject(rc, 0)
	spell.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), spell.ID))
	bear := onBoard(t, e, 0, recoverBearSrc)
	e.G.Players[0].Pool[state.MB] = 1
	e.G.Players[0].Pool[state.MR] = 2
	if life > 0 {
		e.G.Players[0].Life = life
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: spell.ID, From: state.ZHand, To: state.ZGraveyard})
	if z := e.G.Obj(spell.ID).Zone; z != state.ZGraveyard {
		t.Fatalf("fixture precondition: %s zone = %s, want Graveyard", name, z)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZGraveyard})
	e.putTriggersOnStack()
	resolveTopAfterPass(e)
	return e, spell.ID
}

// recoverUnlessPayAsk asserts the fixture reached the unless-pay ask and
// returns it.
func recoverUnlessPayAsk(t *testing.T, e *Engine, wantPayer state.PlayerID) *decision.Decision {
	t.Helper()
	ask := e.Pending()
	if ask == nil || ask.Kind != decision.KModes || ask.ResumeKind != "unless_pay" {
		t.Fatalf("no unless-pay ask after the creature died: %+v", ask)
	}
	if ask.Player != wantPayer {
		t.Fatalf("pay ask player = seat %d, want %d (the Recover card's owner)", ask.Player, wantPayer)
	}
	if ask.ResumeSA == nil || ask.ResumeSA.API != "ChangeZone" {
		t.Fatalf("ask SA = %+v, want the DB$ ChangeZone body", ask.ResumeSA)
	}
	return ask
}

// TestGrimHarvestRecoverPaidReturnsToHand: a creature you own dies while
// Grim Harvest is in your graveyard; you pay {2}{B} and it returns to your
// hand.
func TestGrimHarvestRecoverPaidReturnsToHand(t *testing.T) {
	t.Parallel()
	e, spell := recoverFixture(t, 8113, "Grim Harvest")
	ask := recoverUnlessPayAsk(t, e, 0)
	submitChoices(t, e, ask.Options[0].Index)
	if got := e.G.Players[0].Pool[state.MB] + e.G.Players[0].Pool[state.MR]; got != 0 {
		t.Fatalf("pool after pay = %d mana left, want 0 ({2}{B} spent)", got)
	}
	if z := e.G.Obj(spell).Zone; z != state.ZHand {
		t.Fatalf("paid Recover: Grim Harvest zone = %s, want Hand", z)
	}
	if containsObj(e.G.Zone(state.ZGraveyard, 0), spell) {
		t.Fatalf("paid Recover left Grim Harvest's graveyard entry behind: gy=%v hand=%v", e.G.Zone(state.ZGraveyard, 0), e.G.Zone(state.ZHand, 0))
	}
}

// TestRecoverDeclinedExilesTheCard: the same shape, declined — Grim
// Harvest is exiled instead and no mana was spent.
func TestRecoverDeclinedExilesTheCard(t *testing.T) {
	t.Parallel()
	e, spell := recoverFixture(t, 8114, "Grim Harvest")
	ask := recoverUnlessPayAsk(t, e, 0)
	submitChoices(t, e, ask.Options[1].Index)
	if z := e.G.Obj(spell).Zone; z != state.ZExile {
		t.Fatalf("declined Recover: Grim Harvest zone = %s, want Exile", z)
	}
	if got := e.G.Players[0].Pool[state.MB] + e.G.Players[0].Pool[state.MR]; got != 3 {
		t.Fatalf("pool after decline = %d, want 3 (nothing was spent)", got)
	}
	if len(e.G.Zone(state.ZExile, 0)) != 1 {
		t.Fatalf("seat 0 exile = %v, want exactly the declined Recover card", e.G.Zone(state.ZExile, 0))
	}
}

// TestRecoverIgnoresOpponentsCreature: an OPPONENT's creature dying goes to
// the opponent's graveyard — not "your" graveyard — so the trigger never
// fires and no ask is posed.
func TestRecoverIgnoresOpponentsCreature(t *testing.T) {
	t.Parallel()
	e, spell := recoverFixtureOpponentDeath(t, 8115)
	if ask := e.Pending(); ask != nil && ask.ResumeKind == "unless_pay" {
		t.Fatalf("opponent creature dying posed %+v, want no Recover trigger", ask.ResumeSA)
	}
	if z := e.G.Obj(spell).Zone; z != state.ZGraveyard {
		t.Fatalf("Grim Harvest moved = %s, want still in the graveyard", z)
	}
}

// recoverFixtureOpponentDeath is recoverFixture with the dying creature on
// seat 1's battlefield: its corpse goes to seat 1's graveyard.
func recoverFixtureOpponentDeath(t *testing.T, seed uint64) (*Engine, state.ObjID) {
	t.Helper()
	reg := searchTestRegistry(t)
	rc, ok := reg.Lookup("Grim Harvest")
	if !ok {
		t.Fatal("corpus card Grim Harvest missing")
	}
	e := stealEngine(t, seed)
	spell := e.G.AddObject(rc, 0)
	spell.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), spell.ID))
	bear := onBoard(t, e, 1, recoverBearSrc)
	e.emit(events.Event{Kind: events.MoveZone, Obj: spell.ID, From: state.ZHand, To: state.ZGraveyard})
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZGraveyard})
	e.putTriggersOnStack()
	return e, spell.ID
}

// TestRecoverCardLeftGraveyardDoesNothing: the card leaves the graveyard
// after the trigger fired but before resolution; nothing happens (the
// resolution's ChangeZones find the object in its Origin$ Graveyard no
// longer, CR 400.7) — the card is neither returned nor exiled.
func TestRecoverCardLeftGraveyardDoesNothing(t *testing.T) {
	t.Parallel()
	e, spell := recoverFixture(t, 8116, "Grim Harvest")
	recoverUnlessPayAsk(t, e, 0)
	// The card leaves the graveyard before the resolution completes.
	e.emit(events.Event{Kind: events.MoveZone, Obj: spell, From: state.ZGraveyard, To: state.ZHand})
	submitChoices(t, e, e.Pending().Options[1].Index)
	if z := e.G.Obj(spell).Zone; z != state.ZHand {
		t.Fatalf("departed Recover card zone = %s, want Hand (nothing happened)", z)
	}
	if len(e.G.Zone(state.ZExile, 0)) != 0 {
		t.Fatalf("seat 0 exile = %v, want empty", e.G.Zone(state.ZExile, 0))
	}
}

// TestEveryRecoverCardIsPlayable is the class census: the seven printed
// corpus carriers of `K:Recover` are supported now.
func TestEveryRecoverCardIsPlayable(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	for _, name := range []string{
		"Controvert", "Garza's Assassin", "Grim Harvest", "Icefall",
		"Krovikan Rot", "Resize", "Sun's Bounty",
	} {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("corpus card %q missing", name)
		}
		if miss := reg.Unsupported(c, sup); len(miss) != 0 {
			t.Errorf("%s is still unsupported: %v", name, miss)
		}
	}
}
