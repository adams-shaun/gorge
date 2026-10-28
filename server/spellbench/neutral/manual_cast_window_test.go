package neutral_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/neutral"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// TestManualHostPoolOnlyCastWindowKeepsBothMethods pins the manual-host window
// the twin rebuilds once the host has floated the mana itself: no
// activate_mana_ability candidate, a pool that pays, and Gollum's adventure
// and creature casts both offered. A session's zero value is the manual host,
// so this test never calls SetAutoPay: the host declaration, not the absence
// of mana candidates, decides who pays.
func TestManualHostPoolOnlyCastWindowKeepsBothMethods(t *testing.T) {
	reg := testcorpus.Registry(t)
	b := payBoard{
		lands:  []string{"Mountain", "Island", "Plains", "Swamp"},
		hand:   []string{"Gollum, Silent Slinker"},
		tapped: map[string]bool{"Mountain": true, "Island": true, "Plains": true, "Swamp": true},
	}
	total := 0
	for _, n := range b.pool() {
		total += n
	}
	if total < 4 || b.pool()["B"] < 1 {
		t.Fatalf("precondition: pool %v cannot pay the creature cast", b.pool())
	}
	cands := []string{
		`{"kind":"pass"}`,
		`{"kind":"cast_spell","source":` + ref0("hand", "Gollum, Silent Slinker") + `,"method":"adventure"}`,
		`{"kind":"cast_spell","source":` + ref0("hand", "Gollum, Silent Slinker") + `,"method":"normal"}`,
	}
	var q struct {
		Decision protocol.SeatDecision `json:"decision"`
	}
	if err := json.Unmarshal([]byte(chooseLine(2, b, cands)), &q); err != nil {
		t.Fatal(err)
	}
	p, err := neutral.NewSession(reg).Payload(&q.Decision)
	if err != nil {
		t.Fatal(err)
	}
	var modes []string
	for _, o := range p.Decision.Options {
		modes = append(modes, o.Mode)
	}
	if !slices.Contains(modes, "adventure_alt") || !slices.Contains(modes, "") {
		t.Fatalf("precondition: both cast methods should be in the native options, got %v", modes)
	}
	adv, norm := p.Ops[1].Option, p.Ops[2].Option
	if adv == norm || p.Decision.Options[adv].Mode != "adventure_alt" || p.Decision.Options[norm].Mode != "" {
		t.Fatalf("precondition: the two cast candidates must map to distinct native options, got %+v", p.Ops)
	}
	if p.Decision.Options[adv].Obj != p.Decision.Options[norm].Obj {
		t.Fatalf("precondition: both casts must be the same source object, got %d and %d", p.Decision.Options[adv].Obj, p.Decision.Options[norm].Obj)
	}
	if p.Ops[1].Op != "choose" || p.Ops[2].Op != "choose" {
		t.Fatalf("a manual host's pool-only cast window must offer both casts as chooses, got ops %+v", p.Ops)
	}
	if p.Ops[1].Payment != nil || p.Ops[2].Payment != nil {
		t.Fatalf("a manual host's cast window must not tag a plan, got ops %+v", p.Ops)
	}
	if len(p.Decision.PaymentActions) != 0 {
		t.Fatalf("a manual host's pool-only window must not re-apply a plan: %d payment actions", len(p.Decision.PaymentActions))
	}
}
