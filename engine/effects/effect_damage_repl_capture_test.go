package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

func effectDamageReplCaptureRun(t *testing.T, validSource string) state.ContinuousEffect {
	t.Helper()
	g := state.NewGame([]string{"A"})
	host := g.AddObject(nil, 0)
	chosen := g.AddObject(nil, 0)
	g.Obj(host.ID).Chosen = []state.Target{{Obj: chosen.ID}}
	h := &fakeHost{g: g}
	sa := &cards.SA{API: "Effect", Params: map[string]string{"ReplacementEffects": "RPrevent"}}
	c := &Ctx{Source: host.ID, Controller: 0, SVars: map[string]string{
		"RPrevent": "Event$ DamageDone | Prevent$ True | ValidSource$ " + validSource + " | Description$ x",
	}}
	effEffect(h, c, sa)
	for _, ce := range h.continuous {
		if ce.ReplacementEvent == "DamageDone" {
			return ce
		}
	}
	t.Fatalf("no DamageDone replacement registered (continuous=%v)", h.continuous)
	return state.ContinuousEffect{}
}

// TestEffectDamageReplChosenSnapshotTaken: a replacement naming ChosenCard
// binds the creation-time choice.
func TestEffectDamageReplChosenSnapshotTaken(t *testing.T) {
	ce := effectDamageReplCaptureRun(t, "Card.ChosenCard")
	if !ce.ChosenBound || len(ce.Chosen) != 1 {
		t.Fatalf("Chosen=%v bound=%v, want the one chosen card bound", ce.Chosen, ce.ChosenBound)
	}
}

// TestEffectDamageReplNoChosenSpecNoSnapshot: specs that do not name
// ChosenCard take no snapshot, so every other registration is unchanged.
func TestEffectDamageReplNoChosenSpecNoSnapshot(t *testing.T) {
	ce := effectDamageReplCaptureRun(t, "Creature")
	if ce.ChosenBound || ce.Chosen != nil {
		t.Fatalf("Chosen=%v bound=%v, want no snapshot", ce.Chosen, ce.ChosenBound)
	}
}
