package decision

import (
	"encoding/json"
	"strings"
	"testing"
)

// A client builds a legal declaration from this field, so it has to reach the
// wire, and an unrestricted option has to serialise as it always did.
func TestAloneRestrictedReachesTheWire(t *testing.T) {
	restricted, err := json.Marshal(Option{Kind: "attacker", AloneRestricted: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(restricted), `"alone_restricted":true`) {
		t.Errorf("restricted option: %s", restricted)
	}
	ordinary, err := json.Marshal(Option{Kind: "attacker"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(ordinary), "alone_restricted") {
		t.Errorf("ordinary option: %s", ordinary)
	}
}
