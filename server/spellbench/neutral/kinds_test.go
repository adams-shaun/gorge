package neutral_test

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/neutral"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// This file covers the v2 candidate kinds gorge's engine never emits:
// distribute, choose_cost_option, choose_cast_method, choose_option and
// choose_pile. A gorget-hosted game never asks them; an agent playing
// another engine's v2 game must still answer them from a gorge decision
// instead of falling to noOp.

func newKindSession(t *testing.T) *neutral.Session {
	t.Helper()
	return neutral.NewSession(testcorpus.Registry(t))
}

// TestNeutralDistributeKind answers a combat-damage distribution with the
// engine-order split: a 1/1 blocker takes 1 (its lethal), not all 3.
func TestNeutralDistributeKind(t *testing.T) {
	s := newKindSession(t)
	epi := ref("o-epi", "Voldaren Epicure", "battlefield")
	var cands []string
	for amount := 0; amount <= 3; amount++ {
		cands = append(cands, fmt.Sprintf(`{"kind":"distribute","source":null,"purpose":"combat_damage","recipient":{"object":%s},"amount":%d,"remaining":3}`, epi, amount))
	}
	first := seatDecision(t, choiceCtx("null", "combat_damage"), `{"group_id":9,"substep_index":0,"substep_count":2}`, cands...)
	p, err := s.Payload(&first)
	if err != nil {
		t.Fatal(err)
	}
	if p.Decision.Kind != decision.KChoose || len(p.Decision.Options) != 4 || len(p.Ops) != 4 {
		t.Fatalf("decision %+v ops %+v, want a four-amount KChoose", p.Decision, p.Ops)
	}
	for i, op := range p.Ops {
		if op.Op != "choose" {
			t.Fatalf("candidate %d translated to %+v, want a choose op", i, op)
		}
	}
	if got := p.Decision.Options[0].Amount; got != 1 {
		t.Fatalf("first option gives %d, want the 1/1's lethal 1", got)
	}
	if got := p.Ops[1].Option; got != 0 {
		t.Fatalf("amount 1 maps to option %d, want the engine-order first option 0", got)
	}
	last := seatDecision(t, choiceCtx("null", "combat_damage"), `{"group_id":9,"substep_index":1,"substep_count":2}`, cands...)
	p, err = s.Payload(&last)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Decision.Options[0].Amount; got != 3 {
		t.Fatalf("last recipient's first option gives %d, want the remaining 3", got)
	}
}

// TestNeutralCostOptionKind maps a cost choice to the spell's printed
// GenericChoice modes, in printed order.
func TestNeutralCostOptionKind(t *testing.T) {
	s := newKindSession(t)
	src := ref("o-hw", "Highway Robbery", "hand")
	sd := seatDecision(t, choiceCtx(src, ""), `{"group_id":10,"substep_index":0,"substep_count":1}`,
		`{"kind":"choose_cost_option","source":`+src+`,"choice":"sacrifice_land"}`,
		`{"kind":"choose_cost_option","source":`+src+`,"choice":"discard_card"}`)
	p, err := s.Payload(&sd)
	if err != nil {
		t.Fatal(err)
	}
	if p.Decision.Kind != decision.KModes || len(p.Decision.Options) != 2 {
		t.Fatalf("decision %+v, want two printed cost modes", p.Decision)
	}
	if p.Ops[0].Op != "choose" || p.Ops[1].Op != "choose" {
		t.Fatalf("ops %+v, want both cost choices translated", p.Ops)
	}
	if p.Ops[0].Option != 1 || p.Ops[1].Option != 0 {
		t.Fatalf("cost options %+v, want discard printed first and sacrifice second", p.Ops)
	}
}

// TestNeutralChooseOptionKind maps an effect option to gorge's modal ask.
func TestNeutralChooseOptionKind(t *testing.T) {
	s := newKindSession(t)
	sd := seatDecision(t, choiceCtx("null", "effect_option"), `{"group_id":11,"substep_index":0,"substep_count":1}`,
		`{"kind":"choose_option","source":null,"purpose":"effect_option","option_index":0,"option_count":2,"option_label":null}`,
		`{"kind":"choose_option","source":null,"purpose":"effect_option","option_index":1,"option_count":2,"option_label":null}`)
	p, err := s.Payload(&sd)
	if err != nil {
		t.Fatal(err)
	}
	if p.Decision.Kind != decision.KModes || len(p.Decision.Options) != 2 {
		t.Fatalf("decision %+v, want a two-mode ask", p.Decision)
	}
	for i, op := range p.Ops {
		if op.Op != "choose" {
			t.Fatalf("option %d translated to %+v, want a choose op", i, op)
		}
	}
	if p.Ops[0].Option != 0 || p.Ops[1].Option != 1 {
		t.Fatalf("option ops %+v, want printed-order indices", p.Ops)
	}
}

// TestNeutralChoosePileKind translates a two-pile choice and counts the
// first-pile stand-in as an approximation, not an untranslated decision.
func TestNeutralChoosePileKind(t *testing.T) {
	s := newKindSession(t)
	bolt := ref("o-bolt", "Lightning Bolt", "hand")
	mountain := ref("o-mtn1", "Mountain", "battlefield")
	piles := fmt.Sprintf(`[%s],[%s]`, bolt, mountain)
	sd := seatDecision(t, choiceCtx("null", "effect"), `{"group_id":12,"substep_index":0,"substep_count":1}`,
		`{"kind":"choose_pile","source":null,"purpose":"effect","pile_index":0,"piles":[`+piles+`]}`,
		`{"kind":"choose_pile","source":null,"purpose":"effect","pile_index":1,"piles":[`+piles+`]}`)
	p, err := s.Payload(&sd)
	if err != nil {
		t.Fatal(err)
	}
	if p.Decision.Kind != decision.KChoose || len(p.Decision.Options) != 2 {
		t.Fatalf("decision %+v, want a two-pile KChoose", p.Decision)
	}
	if p.Ops[0].Op != "choose" || p.Ops[0].Option != 0 || p.Ops[1].Op != "choose" || p.Ops[1].Option != 1 {
		t.Fatalf("pile ops %+v, want pile 0 first", p.Ops)
	}
	if s.Counts["choose_pile"] != 0 || s.Approx["choose_pile"] != 1 {
		t.Fatalf("counts %v approx %v, want the first-pile stand-in counted as an approximation", s.Counts, s.Approx)
	}
}

// TestNeutralUntranslatedBudget runs a kernel-shaped transcript through the
// neutral session and holds the untranslated share to 0.5%. The transcript
// includes every translated family and exactly one unmappable kind, so a
// regression in any translator fails the budget long before the bound.
func TestNeutralUntranslatedBudget(t *testing.T) {
	s := newKindSession(t)
	var transcript []protocol.SeatDecision
	add := func(sd protocol.SeatDecision) { transcript = append(transcript, sd) }
	one := func() string { return `{"group_id":1,"substep_index":0,"substep_count":1}` }

	bolt := ref("o-bolt", "Lightning Bolt", "hand")
	epi := ref("o-epi", "Voldaren Epicure", "battlefield")
	highway := ref("o-hw", "Highway Robbery", "hand")
	piles := fmt.Sprintf(`[%s],[%s]`, bolt, ref("o-mtn1", "Mountain", "battlefield"))

	for i := 0; i < 20; i++ {
		// Priority pass; then a cast and its choose_cast_method follow-up
		// (the method inherits the priority plan).
		add(seatDecision(t, priority, one(), `{"kind":"pass"}`))
		add(seatDecision(t, priority, one(), `{"kind":"cast_spell","source":`+bolt+`,"method":null}`))
		add(seatDecision(t, choiceCtx(bolt, ""), one(), `{"kind":"choose_cast_method","source":`+bolt+`,"method":"normal"}`))
		add(seatDecision(t, choiceCtx("null", "effect_option"), one(),
			`{"kind":"choose_option","source":null,"purpose":"effect_option","option_index":0,"option_count":2,"option_label":null}`,
			`{"kind":"choose_option","source":null,"purpose":"effect_option","option_index":1,"option_count":2,"option_label":null}`))
		add(seatDecision(t, choiceCtx(highway, ""), one(),
			`{"kind":"choose_cost_option","source":`+highway+`,"choice":"sacrifice_land"}`,
			`{"kind":"choose_cost_option","source":`+highway+`,"choice":"discard_card"}`))
		add(seatDecision(t, choiceCtx("null", "combat_damage"), one(),
			fmt.Sprintf(`{"kind":"distribute","source":null,"purpose":"combat_damage","recipient":{"object":%s},"amount":0,"remaining":3}`, epi),
			fmt.Sprintf(`{"kind":"distribute","source":null,"purpose":"combat_damage","recipient":{"object":%s},"amount":1,"remaining":3}`, epi),
			fmt.Sprintf(`{"kind":"distribute","source":null,"purpose":"combat_damage","recipient":{"object":%s},"amount":2,"remaining":3}`, epi),
			fmt.Sprintf(`{"kind":"distribute","source":null,"purpose":"combat_damage","recipient":{"object":%s},"amount":3,"remaining":3}`, epi)))
		add(seatDecision(t, choiceCtx("null", "effect"), one(),
			`{"kind":"choose_pile","source":null,"purpose":"effect","pile_index":0,"piles":[`+piles+`]}`,
			`{"kind":"choose_pile","source":null,"purpose":"effect","pile_index":1,"piles":[`+piles+`]}`))
		add(seatDecision(t, choiceCtx(bolt, ""), one(),
			`{"kind":"choose_spell_mode","source":`+bolt+`,"mode_index":0,"mode_count":2,"selected_count":0,"minimum":1,"maximum":1}`,
			`{"kind":"choose_spell_mode","source":`+bolt+`,"mode_index":1,"mode_count":2,"selected_count":0,"minimum":1,"maximum":1}`))
		add(seatDecision(t, choiceCtx("null", "optional_trigger"), one(),
			`{"kind":"choose_boolean","source":null,"purpose":"optional_trigger","value":true}`,
			`{"kind":"choose_boolean","source":null,"purpose":"optional_trigger","value":false}`))
		add(seatDecision(t, choiceCtx("null", "amount"), one(),
			`{"kind":"choose_number","source":null,"purpose":"amount","value":2,"minimum":0,"maximum":3}`))
	}
	// The one decision no translator covers: a name choice. It spends the
	// whole 0.5% budget by itself (1/201), so a second regression fails.
	add(seatDecision(t, choiceCtx("null", "card_name"), one(),
		`{"kind":"choose_name","source":null,"purpose":"card_name","value":"Bear"}`))

	for i := range transcript {
		if _, err := s.Payload(&transcript[i]); err != nil {
			t.Fatalf("decision %d (%s): %v", i, transcript[i].Candidates[0].Semantic.Kind, err)
		}
	}
	untranslated := 0
	for _, n := range s.Counts {
		untranslated += n
	}
	if len(transcript) < 200 {
		t.Fatalf("transcript has %d decisions, want at least 200 for a meaningful 0.5%% budget", len(transcript))
	}
	if got := float64(untranslated) / float64(len(transcript)); got > 0.005 {
		t.Fatalf("untranslated %d/%d = %.2f%%, want <= 0.5%% (counts %v)", untranslated, len(transcript), got*100, s.Counts)
	}
}
