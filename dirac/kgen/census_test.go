package kgen

import (
	"io"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestCensusSelfReferencingSVar pins the census walk's bound: an SVar that
// names itself (directly, or through another SVar) through Triggers$ or
// Execute$ once recursed until the goroutine stack overflowed on the full
// corpus, because cards.ResolveSVar returns a fresh *SA per call.
func TestCensusSelfReferencingSVar(t *testing.T) {
	svars := map[string]string{
		"Self": "DB$ Pump | Triggers$ Self",
		"A":    "DB$ Pump | Execute$ B",
		"B":    "DB$ Pump | Triggers$ A",
	}
	for _, root := range []string{"Self", "A"} {
		sa := cards.ResolveSVar(svars, root)
		if sa == nil {
			t.Fatalf("ResolveSVar(%s) = nil", root)
		}
		f := &cards.Face{SVars: svars, Abilities: []*cards.SA{sa}}
		var c census
		c.face("Test Card", f)
		if len(c.use["api:Pump Triggers"]) != 1 {
			t.Fatalf("root %s: census missed api:Pump Triggers: %v", root, c.use)
		}
	}
	runCensus(&pool{}, io.Discard)
}
