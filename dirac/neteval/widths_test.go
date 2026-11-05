package neteval

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/feat"
)

// TestLoadWeightsRefusesWiderNet: a net reading more entity lanes than the
// encoder writes is refused (netsimd would zero-fill them silently); the
// legacy widths and the encoder's own load; the bag and card widths must
// agree, at any value.
func TestLoadWeightsRefusesWiderNet(t *testing.T) {
	for _, raw := range []int{68, 84, feat.EntityRawWidth} {
		if err := checkWidths("n", 32, 32, raw); err != nil {
			t.Errorf("raw %d refused: %v", raw, err)
		}
	}
	err := checkWidths("n", 32, 32, feat.EntityRawWidth+16)
	if err == nil || !strings.Contains(err.Error(), "this encoder writes") {
		t.Fatalf("wider net: err = %v", err)
	}
	if err := checkWidths("n", 64, 64, feat.EntityRawWidth); err != nil {
		t.Errorf("64-wide vectors refused: %v", err)
	}
	if err := checkWidths("n", 64, 32, feat.EntityRawWidth); err == nil {
		t.Error("mismatched bag/card widths accepted")
	}
}
