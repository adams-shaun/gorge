package rules

import (
	"fmt"
	"reflect"
	"testing"
	"unsafe"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEntryPreviewNeverWritesALiveMap is the map half of
// TestEntryPreviewSharesNoAppendableSlice. The entry preview is a shallow
// struct copy of the engine, so every map field is the live engine's map by
// reference (the slices have private copies). Rather than pay a
// per-entry clone of ~50 maps, the preview's paths are held READ-ONLY on them:
// across a real acceptance game, at every checkpoint, each non-nil map field
// of the live engine is fingerprinted, the preview paths run (the preview of
// a token mint and of the top card of each library entering the battlefield,
// the grant plan and the grant settlement the emit pre-pass runs), and every
// fingerprint must be unchanged. A map the preview starts writing through
// fails here, and the fix is a private copy in isolatePreviewScratch.
func TestEntryPreviewNeverWritesALiveMap(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("plays an acceptance game")
	}
	reg := testutil.CorpusRegistry(t)
	names, decks, err := testutil.AcceptanceDecks(reg, 2)
	if err != nil {
		t.Fatalf("%v", err)
	}
	var maps func(lv reflect.Value, path string, out map[string]reflect.Value)
	maps = func(lv reflect.Value, path string, out map[string]reflect.Value) {
		for i := 0; i < lv.NumField(); i++ {
			f := lv.Type().Field(i)
			v := reflect.NewAt(f.Type, unsafe.Pointer(lv.Field(i).UnsafeAddr())).Elem()
			if f.Anonymous && f.Type.Kind() == reflect.Struct {
				maps(v, path+f.Name+".", out)
				continue
			}
			if f.Type.Kind() == reflect.Map {
				out[path+f.Name] = v
			}
		}
	}
	fingerprint := func(e *Engine) map[string]string {
		live := map[string]reflect.Value{}
		maps(reflect.ValueOf(e).Elem(), "", live)
		out := make(map[string]string, len(live))
		for name, v := range live {
			// fmt prints map entries in sorted key order, so equal contents
			// give equal text.
			out[name] = fmt.Sprintf("%d:%v", v.Len(), v.Interface())
		}
		return out
	}
	checkpoints, seen := 0, 0
	check := func(e *Engine, n int) {
		checkpoints++
		// Most map fields are allocated lazily and are nil on a real engine
		// (a preview write to a nil map lands in the preview's own copy of
		// the field). Give every nil one a live, empty map so a write that
		// would otherwise be hidden by the laziness is caught.
		all := map[string]reflect.Value{}
		maps(reflect.ValueOf(e).Elem(), "", all)
		for _, v := range all {
			if v.IsNil() {
				v.Set(reflect.MakeMap(v.Type()))
			}
		}
		before := fingerprint(e)
		evs := []events.Event{{Kind: events.TokenCreate, Player: e.G.Active, Text: "w_1_1_human"}}
		for p := range e.G.Players {
			if lib := e.G.Zone(state.ZLibrary, state.PlayerID(p)); len(lib) > 0 {
				evs = append(evs, events.Event{Kind: events.MoveZone, Obj: lib[0], From: state.ZLibrary, To: state.ZBattlefield})
			}
		}
		for _, ev := range evs {
			preview, entrant := e.entryPreview(ev)
			if o := preview.G.Obj(entrant); o != nil && o.Zone == state.ZBattlefield {
				grants, _ := e.entryGrantPlan(ev, preview, entrant)
				preview.settleEntryGrants(entrant, grants)
			}
			e.releaseEntryPreview(preview)
		}
		after := fingerprint(e)
		if len(before) < 40 {
			t.Errorf("only %d non-nil map fields on the mid-game engine: the check is too shallow", len(before))
		}
		for name, b := range before {
			if after[name] != b {
				t.Errorf("checkpoint %d: live map %s changed under the entry preview:\n before %.200s\n after  %.200s", n, name, b, after[name])
			}
		}
	}
	// The acceptance loop's own step hook fires every 997 intents; wrap the
	// bot instead so the check runs at a denser, deterministic cadence.
	cfg := AcceptanceConfig(reg, names, decks)
	_, _, err = PlayAcceptance(cfg, func(seed uint64) Answerer {
		inner := acceptanceTestBot(seed)
		return func(e *Engine, d *decision.Decision) decision.Intent {
			seen++
			if seen%20 == 0 && seen >= 100 && checkpoints < 60 && e.G.Tokens != nil {
				check(e, seen)
			}
			return inner(e, d)
		}
	}, nil)
	if err != nil {
		t.Fatalf("acceptance game: %v", err)
	}
	if checkpoints < 3 {
		t.Fatalf("only %d checkpoints reached", checkpoints)
	}
}
