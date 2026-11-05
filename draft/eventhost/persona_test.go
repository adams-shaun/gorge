package eventhost

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited/pickbot"
)

// TestPersonaBalanced covers the event host's persona switch for the
// balanced pair: it resolves to the committed drafter and the balanced
// builder with the caller's lookup injected, the default rating-colour pair
// still resolves to its own types, and an unknown name errors listing the
// registry's names.
func TestPersonaBalanced(t *testing.T) {
	h := &Host{}
	d, b, err := h.newPersona("balanced", nil)
	if err != nil {
		t.Fatalf("newPersona(balanced): %v", err)
	}
	if _, ok := d.(*pickbot.Committed); !ok {
		t.Errorf("balanced drafter is %T, want *pickbot.Committed", d)
	}
	if _, ok := b.(*pickbot.Balanced); !ok {
		t.Errorf("balanced builder is %T, want *pickbot.Balanced", b)
	}

	d, b, err = h.newPersona("rating-colour", nil)
	if err != nil {
		t.Fatalf("newPersona(rating-colour): %v", err)
	}
	if _, ok := d.(*pickbot.RatingColour); !ok {
		t.Errorf("rating-colour drafter is %T, want *pickbot.RatingColour", d)
	}
	if _, ok := b.(*pickbot.Best23); !ok {
		t.Errorf("rating-colour builder is %T, want *pickbot.Best23", b)
	}

	_, _, err = h.newPersona("nope", nil)
	if err == nil {
		t.Fatal("newPersona(nope) succeeded, want an error")
	}
	for _, name := range []string{"balanced", "best23", "rating-colour"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not list %q", err, name)
		}
	}
}
