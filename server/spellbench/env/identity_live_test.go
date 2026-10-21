package env_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/server/spellbench/env"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
)

// TestEngineIdentityCarriesLiveFingerprint keeps the production identity
// honest while the goldens pin a fixed one.
func TestEngineIdentityCarriesLiveFingerprint(t *testing.T) {
	s := env.New(testcorpus.Registry(t), nil)
	m := resp(t, s.Handle([]byte(`{"request_type":"hello","protocol":"spellbench/v2","request_id":"h-1","protocol_minor":0}`)))
	eng, ok := m["engine"].(map[string]any)
	if !ok {
		t.Fatalf("hello_ok has no engine block: %v", m)
	}
	rules, _ := eng["rules_snapshot_id"].(string)
	want := "/ir-" + cards.CompilerFingerprint
	if !strings.HasSuffix(rules, want) {
		t.Fatalf("rules_snapshot_id %q does not end in %q", rules, want)
	}
	pool, _ := eng["card_pool_identity"].(string)
	if !regexp.MustCompile(`/corpus-[0-9a-f]{16}(/|$)`).MatchString(pool) {
		t.Fatalf("card_pool_identity %q has no /corpus-<16 hex>", pool)
	}
}
