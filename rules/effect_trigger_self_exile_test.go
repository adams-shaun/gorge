package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/state"
)

// TestEffectTriggerSelfExileEndsRealCorpusRegistration exercises Palace
// Jailer, whose real DB$ Effect | Triggers$ ComeBack registration resolves
// TrigReturn and chains ExileSelf. The carrier is one of the corpus's
// ChangeZone/Command self-exile sites; this also proves the trigger body is
// not merely registered but reaches the frame-aware move.
func TestEffectTriggerSelfExileEndsRealCorpusRegistration(t *testing.T) {
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Palace Jailer", "Grizzly Bears")
	source := searchMoveByName(t, e, "Palace Jailer", state.ZBattlefield)
	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)
	face := e.G.Obj(source).Face()
	effect := cards.ResolveSVar(face.SVars, "DBEffect")
	if effect == nil {
		t.Fatal("Palace Jailer DBEffect missing from compiled corpus face")
	}
	ctx := &effects.Ctx{Source: source, Controller: e.G.Obj(source).Controller,
		Targets: []state.Target{{Obj: bear}}, Remembered: []state.Target{{Obj: bear}},
		Captured: []state.Target{{Obj: bear}}}
	effects.SetSVars(ctx, face.SVars)
	effects.Resolve(e, ctx, effect)
	found := false
	for _, ce := range e.continuous {
		if ce.Source == source && ce.AddTrigger != nil {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("precondition: Palace Jailer ComeBack trigger was not registered")
	}

	// BecomeMonarch is not a registered trigger matcher in this build, so
	// drive the compiled real Execute body directly after capturing the exact
	// registration frame. This still proves the Effect-created trigger body
	// reaches the self-exile, rather than only proving registration.
	trigger := cards.ResolveSVar(face.SVars, "TrigReturn")
	if trigger == nil {
		t.Fatal("precondition: Palace Jailer TrigReturn missing from corpus face")
	}
	var frame effects.EffectFrame
	for _, ce := range e.continuous {
		if ce.Source == source && ce.AddTrigger != nil {
			frame = effects.EffectFrame{Source: source, Stamp: ce.Timestamp}
			break
		}
	}
	if frame.Source == 0 {
		t.Fatal("precondition: no frame for Palace Jailer trigger")
	}
	ctx = &effects.Ctx{Source: source, Controller: 0,
		Remembered: []state.Target{{Obj: bear}}, EffectFrame: frame}
	effects.SetSVars(ctx, face.SVars)
	effects.Resolve(e, ctx, trigger)
	for _, ce := range e.continuous {
		if ce.Source == source && ce.AddTrigger != nil {
			t.Fatalf("one-shot Effect trigger registration survived self-exile: %+v", ce)
		}
	}
}
