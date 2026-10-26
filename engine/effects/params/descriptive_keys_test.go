package params

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestDescriptiveKeySkipParamsUnread: params' own unreadKeys (DealDamage,
// Mana) drops every caption against an empty table and keeps a real gap.
func TestDescriptiveKeySkipParamsUnread(t *testing.T) {
	ps := map[string]string{"MadeUpUnreadKey": "x"}
	for _, k := range DescriptiveKeys {
		ps[k] = "caption"
	}
	if len(ps) != len(DescriptiveKeys)+1 {
		t.Fatalf("precondition: probe carries %d params, want %d", len(ps), len(DescriptiveKeys)+1)
	}
	want := []string{"MadeUpUnreadKey"}
	if got := unreadKeys(&cards.SA{API: "DealDamage", Params: ps}, nil); !slices.Equal(got, want) {
		t.Fatalf("unreadKeys(all captions, nil) = %v, want %v", got, want)
	}
}
