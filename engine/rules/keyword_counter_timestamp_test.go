package rules

// CR 122.1b + CR 613.7: a keyword counter grants its keyword in layer 6 with
// a timestamp from when the counter was put on the permanent, so a
// "loses all abilities" effect (CR 613.1f) removes only what it is older
// than. Abigale, Eloquent First-Year's trigger removes the target's abilities
// and THEN puts flying, first strike and lifelink counters on it: the counters
// are newer than the removal and keep applying. A counter put on BEFORE a
// later lose-all effect (Final Showdown) is still cleared by it.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestAbigaleKeywordCountersSurviveTheirOwnLoseAllAbilities: Darksteel Myr
// (printed indestructible) is Abigale's target. After the ETB resolves it must
// have lost Indestructible and read Flying, First Strike and Lifelink from the
// counters Abigale put on after the removal.
func TestAbigaleKeywordCountersSurviveTheirOwnLoseAllAbilities(t *testing.T) {
	t.Parallel()
	e, cfg, ids := animateAllEngine(t, 1401, "Abigale, Eloquent First-Year", "Darksteel Myr")
	myr := ids["perm"]
	if !e.HasKeyword(myr, "Indestructible") {
		t.Fatal("precondition: Darksteel Myr does not read Indestructible")
	}
	for range 2 { // {W/B}{W/B}
		e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "W", Amount: 1})
	}
	submitOption(t, e, "", "Cast Abigale, Eloquent First-Year")
	// Each hybrid {W/B} symbol asks how to pay it; answer with the first
	// offered way (the W already in the pool).
	for d := e.Pending(); d != nil && d.Kind == decision.KChoose && strings.HasPrefix(d.Prompt, "Choose how to pay"); d = e.Pending() {
		submitChoices(t, e, d.Options[0].Index)
	}
	passPriorityUntil(t, e, decision.KTarget)
	submitTarget(t, e, myr)
	passUntilStackEmpty(t, e, 30)
	if e.G.Obj(myr).Zone != state.ZBattlefield {
		t.Fatalf("Darksteel Myr zone = %s, want battlefield", e.G.Obj(myr).Zone)
	}
	for _, k := range []string{"Flying", "First Strike", "Lifelink"} {
		if e.G.Obj(myr).Counter(k) != 1 {
			t.Fatalf("precondition: Darksteel Myr has no %s counter (counters %+v)", k, e.G.Obj(myr).Counters)
		}
		if !e.HasKeyword(myr, k) {
			t.Errorf("Darksteel Myr does not read %s: its counter is newer than the loss of abilities (CR 613.7)", k)
		}
	}
	if e.HasKeyword(myr, "Indestructible") {
		t.Error("Darksteel Myr still reads its printed Indestructible after losing all abilities")
	}
	replayCheck(t, e, cfg)
}

// TestKeywordCounterOlderThanLoseAllAbilitiesIsCleared: the other side of the
// same timestamp rule. A flying counter put on the Myr BEFORE Final Showdown's
// lose-all-abilities effect is older than it and is removed with the rest.
func TestKeywordCounterOlderThanLoseAllAbilitiesIsCleared(t *testing.T) {
	t.Parallel()
	e, cfg, ids := animateAllEngine(t, 1310, "Final Showdown", "Darksteel Myr")
	myr := ids["perm"]
	e.emit(events.Event{Kind: events.CounterChange, Obj: myr, Counter: "Flying", Amount: 1})
	if !e.HasKeyword(myr, "Flying") {
		t.Fatal("precondition: the flying counter does not grant Flying")
	}
	for range 2 {
		e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "W", Amount: 1})
	}
	submitOption(t, e, "", "Cast Final Showdown")
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("after cast, next decision = %+v, want modes", d)
	}
	submitChoices(t, e, modeOptionContaining(t, d, "lose all abilities"))
	passUntilStackEmpty(t, e, 30)
	if e.HasKeyword(myr, "Indestructible") {
		t.Fatal("precondition: the lose-all-abilities mode did not resolve")
	}
	if e.HasKeyword(myr, "Flying") {
		t.Error("Darksteel Myr reads Flying from a counter older than the lose-all-abilities effect")
	}
	replayCheck(t, e, cfg)
}
