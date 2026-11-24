package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 701.61a: forage is "exile three cards from your graveyard or sacrifice a
// Food", and which three cards is the payer's choice (CR 601.2h / 118.3).
// A triggered forage (Bushy Bodyguard) and a mana-ability forage (Thornvault
// Forager) both pose a pick of exactly three graveyard cards when the
// graveyard holds more than three, and exile the three picked.

// forageChoiceGraveyard moves five distinct graveyard cards into seat 0's
// graveyard and returns the graveyard in zone order.
func forageChoiceGraveyard(t *testing.T, e *Engine) []state.ObjID {
	t.Helper()
	for _, name := range []string{"Grizzly Bears", "Forest", "Mountain", "Llanowar Elves", "Shock"} {
		searchMoveByName(t, e, name, state.ZGraveyard)
	}
	got := slices.Clone(e.G.Zone(state.ZGraveyard, 0))
	if len(got) != 5 {
		t.Fatalf("fixture: graveyard = %v, want five cards", got)
	}
	return got
}

// forageExilePick asserts d is the exactly-three graveyard pick over grave and
// answers it with the three cards NOT at the top of the graveyard order (the
// last three), returning them.
func forageExilePick(t *testing.T, e *Engine, d *decision.Decision, grave []state.ObjID) []state.ObjID {
	t.Helper()
	if d == nil || d.Kind != decision.KChoose || d.Min != 3 || d.Max != 3 || len(d.Options) != len(grave) {
		t.Fatalf("pending %+v, want a 3..3 pick over the %d graveyard cards", d, len(grave))
	}
	want := slices.Clone(grave[len(grave)-3:])
	var idx []int
	for _, o := range d.Options {
		if !slices.Contains(grave, o.Obj) {
			t.Fatalf("pick option %+v is not a graveyard card (%v)", o, grave)
		}
		if slices.Contains(want, o.Obj) {
			idx = append(idx, o.Index)
		}
	}
	submitChoices(t, e, idx...)
	return want
}

func forageExiledSince(e *Engine, start int) []state.ObjID {
	var out []state.ObjID
	for _, ev := range e.L.Events[start:] {
		if ev.Kind == events.MoveZone && ev.To == state.ZExile && ev.Text == "exiled as a forage cost" {
			out = append(out, ev.Obj)
		}
	}
	return out
}

func TestBushyBodyguardForagePicksThree(t *testing.T) {
	reg := searchTestRegistry(t)
	e, cfg := searchEngine(t, reg, "Bushy Bodyguard", "Llanowar Elves", "Shock")
	grave := forageChoiceGraveyard(t, e)
	searchMoveByName(t, e, "Bushy Bodyguard", state.ZBattlefield)
	d := forageCostAsk(t, e)
	payIdx := optionIndexForKind(d, "trigger_cost_pay")
	if payIdx < 0 {
		t.Fatalf("Forage cost with five graveyard cards offers no pay: %+v", d.Options)
	}
	start := len(e.L.Events)
	submitChoices(t, e, payIdx)
	if got := forageExiledSince(e, start); len(got) != 0 {
		t.Fatalf("paying exiled %v before any pick was posed, want a 3-card pick first", got)
	}
	picked := forageExilePick(t, e, e.Pending(), grave)
	exiled := forageExiledSince(e, start)
	slices.Sort(exiled)
	slices.Sort(picked)
	if !slices.Equal(exiled, picked) || forageActionCount(e.L.Events, start) != 1 {
		t.Fatalf("exiled %v (%d ForageAction), want the picked %v and 1", exiled, forageActionCount(e.L.Events, start), picked)
	}
	if left := e.G.Zone(state.ZGraveyard, 0); len(left) != 2 || !slices.Equal(left, grave[:2]) {
		t.Fatalf("graveyard after forage = %v, want the unpicked %v", left, grave[:2])
	}
	replayCheck(t, e, cfg)
}
