package env_test

import (
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/env"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
)

func TestHelloDoesNotOfferSearch(t *testing.T) {
	s := env.New(testcorpus.Registry(t), nil)
	s.EnableAutoPay()
	m := resp(t, s.Handle([]byte(`{"request_type":"hello","protocol":"spellbench/v2","request_id":"h-1","protocol_minor":0}`)))
	exts := m["extensions"].([]any)
	if len(exts) != 1 || exts[0].(map[string]any)["name"] != "x_gorge_view_v1" {
		t.Fatalf("extensions %v", exts)
	}
	if got := m["engine_defaults"].(map[string]any)["mana_payment"]; got != "engine_autopay" {
		t.Fatalf("mana_payment %v", got)
	}
	manual := env.New(testcorpus.Registry(t), nil)
	if got := resp(t, manual.Handle([]byte(`{"request_type":"hello","protocol":"spellbench/v2","request_id":"h-1","protocol_minor":0}`)))["engine_defaults"].(map[string]any)["mana_payment"]; got != nil {
		t.Fatalf("manual mana_payment %v", got)
	}
}
