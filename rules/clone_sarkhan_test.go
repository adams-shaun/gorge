package rules

// Sarkhan, Soul Aflame is the corpus-pinned carrier for api:Clone's
// trigger-driven, RESOLVING shape (ticket api-clone-trigger-copy): a Dragon
// entering under your control fires the ChangesZone trigger whose Execute$
// is `DB$ Clone | Defined$ TriggeredCardLKICopy | NewName$ Sarkhan, Soul
// Aflame | AddTypes$ Legendary | Duration$ UntilEndOfTurn | Optional$ True`.
// The election is a real yes/no ask; the accepted copy overrides the name,
// keeps the copied characteristics, and expires at end-of-turn cleanup.

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// sarkhanFixture moves Sarkhan Soul Aflame and a Dragon Hatchling onto seat
// 0's battlefield (the Dragon's entry fires Sarkhan's copy trigger) and
// drives to the may-copy election the trigger's resolution poses. Returns
// the engine, the replay config, both object ids and the pending election.
func sarkhanFixture(t *testing.T) (*Engine, Config, state.ObjID, state.ObjID, *decision.Decision) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e, cfg := searchEngine(t, reg, "Sarkhan, Soul Aflame", "Dragon Hatchling")
	sark := searchMoveByName(t, e, "Sarkhan, Soul Aflame", state.ZBattlefield)
	// Preconditions the assertions below stand on: Sarkhan on the battlefield
	// as its printed 2/4 Human Shaman self (so a copy is distinguishable by
	// its 0/1 Flying body), and the Dragon on the battlefield (the trigger's
	// ValidCard$ Dragon.YouCtrl capture).
	if o := e.G.Obj(sark); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Sarkhan fixture missing off the battlefield: %+v", o)
	}
	if f := e.G.Obj(sark).Face(); f == nil || f.Name != "Sarkhan, Soul Aflame" {
		t.Fatalf("Sarkhan fixture wrong face: %+v", f)
	}
	if d := e.Derived(sark); d.Power != 2 || d.Toughness != 4 {
		t.Fatalf("pre-copy Sarkhan P/T %d/%d, want 2/4", d.Power, d.Toughness)
	}
	drag := searchMoveByName(t, e, "Dragon Hatchling", state.ZBattlefield)
	if o := e.G.Obj(drag); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Dragon fixture missing off the battlefield: %+v", o)
	}
	d := passUntilNonPriority(t, e, 40)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "clone" {
		t.Fatalf("the Dragon entry did not pose a clone election: %+v", d)
	}
	if d.Player != 0 || d.Min != 1 || d.Max != 1 {
		t.Fatalf("election = %+v, want a Min==Max==1 KChoose for Sarkhan's controller", d)
	}
	if len(d.Options) != 2 || d.Options[0].Kind != "yes" || d.Options[1].Kind != "no" {
		t.Fatalf("election options = %+v, want a yes/no pair with yes first", d.Options)
	}
	return e, cfg, sark, drag, d
}

// optionIndexOf returns the option whose Kind is kind.
func optionIndexOf(t *testing.T, d *decision.Decision, kind string) int {
	t.Helper()
	for _, o := range d.Options {
		if o.Kind == kind {
			return o.Index
		}
	}
	t.Fatalf("no %q option in %+v", kind, d.Options)
	return -1
}

// TestSarkhanDragonEntryPosesTheMayCopyElectionAndDeclineKeepsSarkhan is the
// election half of the pin: the Dragon entry poses the real may-copy ask,
// and the decline leaves Sarkhan exactly as it printed (no copy event).
func TestSarkhanDragonEntryPosesTheMayCopyElectionAndDeclineKeepsSarkhan(t *testing.T) {
	e, cfg, sark, _, d := sarkhanFixture(t)
	no := optionIndexOf(t, d, "no")
	submitChoices(t, e, no)
	passUntilStackEmpty(t, e, 40)

	// The decline: Sarkhan keeps its printed 2/4 Human Shaman body and no
	// ClonePermanent event named it.
	if d := e.Derived(sark); d.Power != 2 || d.Toughness != 4 {
		t.Fatalf("declined Sarkhan P/T %d/%d, want the printed 2/4", d.Power, d.Toughness)
	}
	if hasEvent(e, events.ClonePermanent, sark) {
		t.Fatal("the decline still cloned")
	}
	if slices.Contains(e.Derived(sark).Types, "Dragon") {
		t.Fatal("declined Sarkhan reads as a Dragon")
	}
	replayCheck(t, e, cfg)
}

// TestSarkhanAcceptedCopyIsNamedLegendaryAndExpiresAtCleanup is the accepted
// half: the copy takes the Dragon's characteristics with the overridden
// name and the added Legendary type, and end-of-turn cleanup reverts it to
// the printed 2/4 self.
func TestSarkhanAcceptedCopyIsNamedLegendaryAndExpiresAtCleanup(t *testing.T) {
	e, cfg, sark, drag, d := sarkhanFixture(t)
	yes := optionIndexOf(t, d, "yes")
	submitChoices(t, e, yes)
	passUntilStackEmpty(t, e, 40)

	// The copy: the Dragon's body (0/1 Flying, red) under the overridden
	// name, Legendary added on top, and Sarkhan now reads as a Dragon.
	f := e.G.Obj(sark).Face()
	if f == nil || f.Name != "Sarkhan, Soul Aflame" {
		t.Fatalf("copy name %v, want the overridden Sarkhan, Soul Aflame", f)
	}
	dv := e.Derived(sark)
	if dv.Power != 0 || dv.Toughness != 1 {
		t.Fatalf("copy P/T %d/%d, want Dragon Hatchling's 0/1", dv.Power, dv.Toughness)
	}
	if !slices.Contains(dv.Keywords, "Flying") {
		t.Fatalf("copy keywords %v, want Flying from the copied Dragon", dv.Keywords)
	}
	if !slices.Contains(dv.Types, "Legendary") || !slices.Contains(dv.Types, "Dragon") {
		t.Fatalf("copy types %v, want Legendary (the AddTypes$ rider) and Dragon (the copied body)", dv.Types)
	}
	if slices.Contains(dv.Types, "Shaman") {
		t.Fatalf("copy types %v: the copied Dragon must not carry the printed Shaman", dv.Types)
	}
	// The copied Dragon itself never moved.
	if o := e.G.Obj(drag); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("the copied Dragon left the battlefield: %+v", o)
	}
	replayCheck(t, e, cfg)

	// Expiry: the copy's Duration$ UntilEndOfTurn ends at this turn's
	// cleanup; Sarkhan reverts to its printed 2/4 Human Shaman.
	e.pending = nil
	e.setStep(state.StepCleanup)
	e.priorityRound()
	f = e.G.Obj(sark).Face()
	if f == nil || f.Name != "Sarkhan, Soul Aflame" {
		t.Fatalf("after cleanup the object is %v, want Sarkhan, Soul Aflame", f)
	}
	dv = e.Derived(sark)
	if dv.Power != 2 || dv.Toughness != 4 {
		t.Fatalf("after cleanup P/T %d/%d, want the printed 2/4", dv.Power, dv.Toughness)
	}
	if slices.Contains(dv.Keywords, "Flying") || slices.Contains(dv.Types, "Dragon") {
		t.Fatalf("after cleanup the copy did not expire: keywords %v types %v", dv.Keywords, dv.Types)
	}
	if !slices.Contains(dv.Types, "Legendary") || !slices.Contains(dv.Types, "Human") {
		t.Fatalf("after cleanup types %v, want the printed Legendary Human Shaman", dv.Types)
	}
}
