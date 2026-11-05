package draftlab

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// TestBalancedAgentResolves covers the balanced agent end to end at the lab
// boundary: the version table lists both halves at 1, the concrete types are
// the committed drafter and the balanced builder, an explicit wrong version
// is refused, and a full FDN pod drafted with balanced builds eight decks
// that both build.Validate and resolve against the lab's registry.
func TestBalancedAgentResolves(t *testing.T) {
	l := testLab(t)

	d, err := l.Drafter(AgentRef{Name: "balanced"})
	if err != nil {
		t.Fatalf("Drafter(balanced): %v", err)
	}
	if d.Ref.Version != "1" {
		t.Errorf("drafter version = %q, want 1", d.Ref.Version)
	}
	if _, ok := d.Drafter.(*pickbot.Committed); !ok {
		t.Errorf("drafter is %T, want *pickbot.Committed", d.Drafter)
	}

	b, err := l.Builder(AgentRef{Name: "balanced"})
	if err != nil {
		t.Fatalf("Builder(balanced): %v", err)
	}
	if b.Ref.Version != "1" {
		t.Errorf("builder version = %q, want 1", b.Ref.Version)
	}
	if _, ok := b.Builder.(*pickbot.Balanced); !ok {
		t.Errorf("builder is %T, want *pickbot.Balanced", b.Builder)
	}

	if _, err := l.Builder(AgentRef{Name: "balanced", Version: "2"}); !errors.Is(err, ErrVersion) {
		t.Errorf("builder balanced@2 err = %v, want ErrVersion", err)
	}

	packs, err := l.OpenPod("FDN", setdata.Arena, 42, 8, 3)
	if err != nil {
		t.Fatalf("OpenPod: %v", err)
	}
	drafters := newDrafters(t, l, "balanced", 8)
	res, err := l.Draft(packs, drafters, 7)
	if err != nil {
		t.Fatalf("Draft: %v", err)
	}
	checkDraft(t, res, packs, 8, 3)
	for s, pool := range res.Pools {
		deck, err := l.Build(pool, b)
		if err != nil {
			t.Fatalf("seat %d Build: %v", s, err)
		}
		if deck.Build == nil {
			t.Fatalf("seat %d deck has no build", s)
		}
		if len(deck.File.Cards) == 0 {
			t.Fatalf("seat %d deck file has no cards", s)
		}
	}
}
