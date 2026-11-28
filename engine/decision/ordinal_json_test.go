package decision

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestDecisionOrdinalNotOnTheWire: Decision.Ordinal (the search seed's
// decisions-answered count) is absent from the JSON encoding, so no wire
// shape moves, and a decoded Decision carries none.
func TestDecisionOrdinalNotOnTheWire(t *testing.T) {
	d := Decision{Seq: 7, Ordinal: 12345, Kind: KPriority}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"seq":7`) {
		t.Fatalf("precondition: Seq is encoded: %s", b)
	}
	if strings.Contains(string(b), "12345") || strings.Contains(strings.ToLower(string(b)), "ordinal") {
		t.Fatalf("Ordinal is on the wire: %s", b)
	}
	var back Decision
	if err := json.Unmarshal([]byte(`{"seq":7,"Ordinal":9,"ordinal":9}`), &back); err != nil {
		t.Fatal(err)
	}
	if back.Ordinal != 0 {
		t.Fatalf("decoded Ordinal %d, want 0", back.Ordinal)
	}
}
