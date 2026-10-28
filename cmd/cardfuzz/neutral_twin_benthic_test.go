package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/server/spellbench/neutral"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/xview"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestBenthicGiantCostGrantIsInherent pins the one priority disagreement the
// v2 twin still reports at seed 8507278219836860430: gorge casts Benthic Giant
// off Absorb Energy's perpetual {1} cost grant (printed 5U charged as 4U), but
// the v2 observation carries no granted static ability or offer-time spell
// cost, so the neutral rebuild prices the printed 5U and offers no plan for
// it. The neutral world therefore keeps Benthic Giant as a plain cast and pays
// the only plan it can rebuild, Tidal Barracuda.
//
// This is an inherent observation-profile gap, not a neutral-view bug: the
// grant is not in the closed v2 observation at all. The test is the causal
// pin, so a future protocol addition that closes the gap fails here and should
// invert the last assertion to demand the neutral 4U plan.
func TestBenthicGiantCostGrantIsInherent(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	p, err := buildPool(reg)
	if err != nil {
		t.Fatal(err)
	}
	jobs := sbJobs(p, 2, 20)
	var job sbJob
	for _, j := range jobs {
		if j.seed == 8507278219836860430 {
			job = j
			break
		}
	}
	if job.seed == 0 {
		t.Fatal("seed 2 no longer contains the recorded Benthic Giant game")
	}
	o := sbOptions{mode: "twin", policy: "bot", autopay: true, maxSteps: 20000,
		maxTurns: 100, maxIntents: 20000, maxObjects: 20000, games: 20, seed: 2, workers: 1}
	g := playSBTwin(reg, job.decks, job.seed, o)
	if g.err != "" {
		t.Fatalf("game failed: %s", g.err)
	}
	var benthic *failure
	for _, f := range g.fails {
		if f.SB != nil && strings.Contains(string(f.SB.NativeSem), "Benthic Giant") &&
			strings.Contains(string(f.SB.V2Sem), "Tidal Barracuda") {
			benthic = f
			break
		}
	}
	if benthic == nil || benthic.SB.Index != 404 {
		t.Fatalf("record lost: %+v", g.fails)
	}
	if benthic.Sig != "priority: cast_spell -> cast_spell" {
		t.Fatalf("sig = %q", benthic.Sig)
	}

	var req struct {
		Decision protocol.SeatDecision `json:"decision"`
	}
	if err := json.Unmarshal(benthic.SB.Request, &req); err != nil {
		t.Fatal(err)
	}
	var xp xview.Payload
	if err := json.Unmarshal(req.Decision.Extensions["x_gorge_view_v1"], &xp); err != nil {
		t.Fatal(err)
	}
	var nativeBenthic, nativeTidal *decision.PaymentAction
	for i := range xp.Decision.PaymentActions {
		a := &xp.Decision.PaymentActions[i]
		if strings.Contains(a.Label, "Benthic Giant") {
			nativeBenthic = a
		}
		if strings.Contains(a.Label, "Tidal Barracuda") {
			nativeTidal = a
		}
	}
	if nativeBenthic == nil || nativeTidal == nil ||
		len(nativeBenthic.Plans) == 0 || len(nativeTidal.Plans) == 0 {
		t.Fatalf("native plan list: %+v", xp.Decision.PaymentActions)
	}
	// The grant: printed 5U is charged as 4U (generic 4 + one blue).
	if got := nativeBenthic.Plans[0].Cost; got.Generic != 4 || got.Mana != (decision.ManaAmount{0, 1, 0, 0, 0, 0}) {
		t.Fatalf("native Benthic cost = %+v, want 4 + {U}", got)
	}
	// Control: Tidal Barracuda has no grant; printed 3U is charged as 3U.
	if got := nativeTidal.Plans[0].Cost; got.Generic != 3 {
		t.Fatalf("native Tidal cost = %+v, want 3 + {U}", got)
	}
	t.Logf("native Benthic Giant: cost=%+v activations=%d (printed 5U reduced by Absorb Energy's {1} grant)",
		nativeBenthic.Plans[0].Cost, len(nativeBenthic.Plans[0].Activations))

	// The neutral rebuild prices the printed 5U and finds no plan for it.
	ns := neutral.NewSession(reg)
	ns.SetAutoPay(true)
	np, err := ns.Payload(&req.Decision)
	if err != nil {
		t.Fatal(err)
	}
	var neutralPlain, neutralPlan string
	for i := range np.Decision.Options {
		if o := np.Decision.Options[i]; o.Kind == "cast" && o.Mode == "" &&
			strings.Contains(o.Label, "Benthic Giant") {
			neutralPlain = o.Label
		}
	}
	for i := range np.Decision.PaymentActions {
		if strings.Contains(np.Decision.PaymentActions[i].Label, "Benthic Giant") {
			neutralPlan = np.Decision.PaymentActions[i].Label
		}
	}
	if neutralPlain == "" || neutralPlan != "" {
		t.Fatalf("neutral Benthic options/plans: plain=%q plan=%q; want a plain cast and no plan", neutralPlain, neutralPlan)
	}
	var tidalPlan *decision.PaymentAction
	for i := range np.Decision.PaymentActions {
		if strings.Contains(np.Decision.PaymentActions[i].Label, "Tidal Barracuda") {
			tidalPlan = &np.Decision.PaymentActions[i]
		}
	}
	if tidalPlan == nil || len(tidalPlan.Plans) == 0 || tidalPlan.Plans[0].Cost.Generic != 3 {
		t.Fatalf("neutral Tidal plan: %+v; want 3 + {U}", tidalPlan)
	}
	t.Logf("neutral Benthic Giant: plain cast=%q plan=%q; neutral Tidal Barracuda: cost=%+v",
		neutralPlain, neutralPlan, tidalPlan.Plans[0].Cost)
}
