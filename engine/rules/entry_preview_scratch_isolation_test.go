package rules

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// previewIsolationEngine is a two-seat board with one Effect-created
// continuous effect, one pending trigger and a filled option buffer: the
// live state an entry preview (a shallow struct copy of the engine) could
// write into.
func previewIsolationEngine(t *testing.T) (*Engine, state.ObjID) {
	t.Helper()
	e := handEngine(t)
	e.G.Tokens = testutil.CorpusRegistry(t).Tokens
	src := stashBoard(t, e, 0, corpusAlternativeCard(t, "Grizzly Bears"))
	other := stashBoard(t, e, 1, corpusAlternativeCard(t, "Grizzly Bears"))
	e.AddContinuous(ContinuousEffect{Source: src, Controller: 0, FromEffect: true, UntilEOT: true})
	e.AddContinuous(ContinuousEffect{Source: other, Controller: 1, FromEffect: true, UntilEOT: true})
	e.continuous = append(make([]ContinuousEffect, 0, 8), e.continuous...)
	e.pendingTriggers = append(make([]pendingTrigger, 0, 8), pendingTrigger{Source: src, Controller: 0})
	e.legalOptBuf = append(make([]decision.Option, 0, 8), decision.Option{Kind: "pass", Label: "live"})
	return e, src
}

// TestEntryPreviewMutationsNeverReachTheLiveEngine extends layer-cache
// isolation to the engine's other shared slices: an entry
// preview that ends an Effect (the in-place filter of the continuous
// registry), rewrites the pending-trigger list or refills a scratch buffer
// must leave the live engine's registry, queue and buffer untouched (CR
// 611.3a: the live game's continuous effects are the live game's alone).
func TestEntryPreviewMutationsNeverReachTheLiveEngine(t *testing.T) {
	t.Parallel()
	e, src := previewIsolationEngine(t)
	preview, _ := e.entryPreview(events.Event{Kind: events.TokenCreate, Player: 0, Text: "w_1_1_human"})
	preview.EndEffectSource(src, 0)
	preview.pendingTriggers = append(preview.pendingTriggers[:0], pendingTrigger{Source: 9999})
	preview.legalOptBuf = append(preview.legalOptBuf[:0], decision.Option{Kind: "pass", Label: "preview"})
	e.releaseEntryPreview(preview)

	if len(e.continuous) != 2 || e.continuous[0].Source != src {
		t.Fatalf("live continuous registry after the preview ended its Effect: %d entries %+v", len(e.continuous), e.continuous)
	}
	if len(e.pendingTriggers) != 1 || e.pendingTriggers[0].Source != src {
		t.Fatalf("live pending triggers after the preview rewrote its queue: %+v", e.pendingTriggers)
	}
	if len(e.legalOptBuf) != 1 || e.legalOptBuf[0].Label != "live" {
		t.Fatalf("live option buffer after the preview refilled its own: %+v", e.legalOptBuf)
	}
}

// TestEntryPreviewSharesNoAppendableSlice is the structural half: no slice
// field of the preview (Engine and its embedded parts) may share the live
// engine's backing array with spare capacity, or a preview append would
// write into the live array. It is checked on a real mid-game engine (an
// acceptance game's checkpoint) whose every non-nil slice field is first
// given spare capacity, so a field the isolation list misses fails here. A
// field may be private (another array), nil or empty; the "share" fields the
// live engine never writes (deck manifests, land type words, attack offers)
// are exempt.
func TestEntryPreviewSharesNoAppendableSlice(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("plays an acceptance game")
	}
	reg := testutil.CorpusRegistry(t)
	names, decks, err := testutil.AcceptanceDecks(reg, 2)
	if err != nil {
		t.Fatalf("%v", err)
	}
	exempt := map[string]bool{"deckManifests": true, "landTypeWords": true, "atkOffers": true}
	type field struct {
		name string
		l, p reflect.Value
	}
	var fields func(lv, pv reflect.Value, path string, out []field) []field
	fields = func(lv, pv reflect.Value, path string, out []field) []field {
		for i := 0; i < lv.NumField(); i++ {
			f := lv.Type().Field(i)
			name := path + f.Name
			l := reflect.NewAt(f.Type, unsafe.Pointer(lv.Field(i).UnsafeAddr())).Elem()
			var p reflect.Value
			if pv.IsValid() {
				p = reflect.NewAt(f.Type, unsafe.Pointer(pv.Field(i).UnsafeAddr())).Elem()
			}
			if f.Anonymous && f.Type.Kind() == reflect.Struct {
				out = fields(l, p, name+".", out)
				continue
			}
			if f.Type.Kind() == reflect.Slice && !exempt[f.Name] {
				out = append(out, field{name, l, p})
			}
		}
		return out
	}
	checked := false
	cfg := AcceptanceConfig(reg, names, decks)
	_, _, err = PlayAcceptance(cfg, acceptanceTestBot, func(e *Engine, n int) {
		if checked || n < 150 || e.G.Tokens == nil {
			return
		}
		checked = true
		promoted := 0
		for _, f := range fields(reflect.ValueOf(e).Elem(), reflect.Value{}, "", nil) {
			if f.l.IsNil() {
				continue
			}
			grown := reflect.MakeSlice(f.l.Type(), f.l.Len(), f.l.Len()+4)
			reflect.Copy(grown, f.l)
			f.l.Set(grown)
			promoted++
		}
		preview, _ := e.entryPreview(events.Event{Kind: events.TokenCreate, Player: e.G.Active, Text: "w_1_1_human"})
		for _, f := range fields(reflect.ValueOf(e).Elem(), reflect.ValueOf(preview).Elem(), "", nil) {
			if f.p.IsNil() || f.l.IsNil() || f.p.Cap() == 0 {
				continue
			}
			if f.p.Pointer() == f.l.Pointer() && f.p.Cap() > f.p.Len() {
				t.Errorf("preview field %s shares the live backing array with spare capacity (len %d cap %d)", f.name, f.p.Len(), f.p.Cap())
			}
		}
		e.releaseEntryPreview(preview)
		if promoted < 10 {
			t.Errorf("only %d non-nil slice fields on the mid-game engine: the check is too shallow", promoted)
		}
	})
	if err != nil {
		t.Fatalf("acceptance game: %v", err)
	}
	if !checked {
		t.Fatal("the acceptance game never reached the checkpoint")
	}
}
