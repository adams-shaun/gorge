package minihost_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/env"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/minihost"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

func scanManaTaps(t *testing.T, res minihost.Result) (taps, windows int) {
	for _, decisions := range res.SeatDecisions {
		for _, raw := range decisions {
			var sd protocol.SeatDecision
			if err := json.Unmarshal(raw, &sd); err != nil {
				t.Fatal(err)
			}
			if sd.Context.Purpose != nil && *sd.Context.Purpose == "mana_payment" {
				windows++
			}
			for _, c := range sd.Candidates {
				if c.Semantic.Kind == "activate_mana_ability" {
					taps++
				}
			}
		}
	}
	return taps, windows
}

// TestAutoPayNeverOffersManaTaps is split one top-level test per catalog deck
// so each stays inside the repo's one-minute test wall, keeping 10 seeded
// games per deck pair. The exact test name
// is kept for Burn, and -run 'TestAutoPayNeverOffersManaTaps' still selects the
// whole set. The two slowest decks split their seeds across two tests.
func TestAutoPayNeverOffersManaTaps(t *testing.T) {
	testAutoPaySeeds(t, "Burn", 0, 10)
}

func TestAutoPayNeverOffersManaTapsCawGatesFirst(t *testing.T) {
	testAutoPaySeeds(t, "CawGates", 0, 5)
}

func TestAutoPayNeverOffersManaTapsCawGatesSecond(t *testing.T) {
	testAutoPaySeeds(t, "CawGates", 5, 10)
}

func TestAutoPayNeverOffersManaTapsRally(t *testing.T) {
	testAutoPaySeeds(t, "Rally", 0, 10)
}

func TestAutoPayNeverOffersManaTapsSpy(t *testing.T) {
	testAutoPaySeeds(t, "Spy", 0, 10)
}

func TestAutoPayNeverOffersManaTapsWildfireFirst(t *testing.T) {
	testAutoPaySeeds(t, "Wildfire", 0, 5)
}

func TestAutoPayNeverOffersManaTapsWildfireSecond(t *testing.T) {
	testAutoPaySeeds(t, "Wildfire", 5, 10)
}

func testAutoPaySeeds(t *testing.T, id string, from, to uint64) {
	t.Helper()
	d, ok := catalog.ByID(id)
	if !ok {
		t.Fatalf("deck %q not in catalog", id)
	}
	for seed := from; seed < to; seed++ {
		seed := seed
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			t.Parallel()
			h := host(t)
			h.KeepDecisions = true
			a, err := h.Play(seed, d, "london", nil, [2]minihost.Link{&minihost.Uniform{}, &minihost.Uniform{}})
			if err != nil {
				t.Fatalf("seed %d: %v", seed, err)
			}
			if a.Terminal.Classification != "natural" {
				t.Fatalf("seed %d: %s (%s)", seed, a.Terminal.Classification, a.Terminal.Reason)
			}
			if taps, windows := scanManaTaps(t, a); taps != 0 || windows != 0 {
				t.Fatalf("seed %d: %d tap candidates, %d mana windows", seed, taps, windows)
			}
		})
	}
}

func TestManualManaFlagRestoresTapCandidates(t *testing.T) {
	reg := testcorpus.Registry(t)
	manual := env.New(reg, nil)
	hello := manual.Handle([]byte(`{"request_type":"hello","protocol":"spellbench/v2","request_id":"h-1","protocol_minor":0}`))
	var h protocol.HelloOK
	if err := json.Unmarshal(hello, &h); err != nil {
		t.Fatal(err)
	}
	if h.EngineDefaults["mana_payment"] != nil {
		t.Fatalf("manual hello declares mana_payment %v", h.EngineDefaults["mana_payment"])
	}
	d, _ := catalog.ByID("Burn")
	for seed := uint64(0); seed < 10; seed++ {
		mh := hostFor(t, env.New(reg, nil))
		mh.KeepDecisions = true
		a, err := mh.Play(seed, d, "london", []string{"x_gorge_view_v1"}, [2]minihost.Link{&minihost.Uniform{}, &minihost.Uniform{}})
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if taps, _ := scanManaTaps(t, a); taps > 0 {
			return
		}
	}
	t.Fatal("no manual game offered a mana tap")
}
