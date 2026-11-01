package draftlab

import (
	"errors"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// flakyDrafter answers like the wrapped drafter until its failAt-th pick,
// then errors (an external agent that dies mid-draft).
type flakyDrafter struct {
	pickbot.Drafter
	failAt int
	seen   int
}

func (d *flakyDrafter) PickErr(v pickbot.PickView, r *rand.Rand) (string, error) {
	d.seen++
	if d.seen == d.failAt {
		return "", errors.New("external agent died")
	}
	return d.Drafter.Pick(v, r), nil
}

// pickErrOnly answers PickErr and panics on Pick: only the fallible path may
// drive it.
type pickErrOnly struct {
	pickbot.Drafter
}

func (d *pickErrOnly) Pick(pickbot.PickView, *rand.Rand) string {
	panic("Pick must not be called on a FallibleDrafter")
}

func (d *pickErrOnly) PickErr(v pickbot.PickView, r *rand.Rand) (string, error) {
	return d.Drafter.Pick(v, r), nil
}

// flakyBuilder errors on every BuildErr (an external agent that died).
type flakyBuilder struct {
	pickbot.Builder
}

func (b *flakyBuilder) BuildErr(string, []limited.CardRef) (build.Deck, error) {
	return build.Deck{}, errors.New("external builder died")
}

// TestFallibleDrafterErrorFailsDraft: a FallibleDrafter whose PickErr errors
// at its 4th pick fails the draft, naming the seat, round and pick and
// carrying the agent's own error.
func TestFallibleDrafterErrorFailsDraft(t *testing.T) {
	l := testLab(t)
	packs, err := l.OpenPod("FDN", setdata.Arena, 42, 4, 3)
	if err != nil {
		t.Fatalf("OpenPod: %v", err)
	}
	base := testDrafter(t, l, "rating-colour")
	drafters := make([]NamedDrafter, 4)
	for s := range drafters {
		drafters[s] = NamedDrafter{
			Ref:     base.Ref,
			Drafter: &flakyDrafter{Drafter: base.Drafter, failAt: 4},
		}
	}
	_, err = l.Draft(packs, drafters, 7)
	if err == nil {
		t.Fatal("Draft succeeded despite the drafter's error")
	}
	if !strings.Contains(err.Error(), "draftlab: draft: pick seat 0 round 1 pick 4") {
		t.Errorf("error %q does not name the seat, round and pick", err)
	}
	if !strings.Contains(err.Error(), "external agent died") {
		t.Errorf("error %q does not carry the agent's own failure", err)
	}
}

// TestFallibleDrafterPreferred: a FallibleDrafter's Pick is never called;
// PickErr answers the whole draft.
func TestFallibleDrafterPreferred(t *testing.T) {
	l := testLab(t)
	packs, err := l.OpenPod("FDN", setdata.Arena, 42, 4, 3)
	if err != nil {
		t.Fatalf("OpenPod: %v", err)
	}
	base := testDrafter(t, l, "rating-colour")
	drafters := make([]NamedDrafter, 4)
	for s := range drafters {
		drafters[s] = NamedDrafter{
			Ref:     base.Ref,
			Drafter: &pickErrOnly{Drafter: base.Drafter},
		}
	}
	res, err := l.Draft(packs, drafters, 7)
	if err != nil {
		t.Fatalf("Draft: %v", err)
	}
	if got, want := len(res.Pools), 4; got != want {
		t.Fatalf("%d pools, want %d", got, want)
	}
	for s, pool := range res.Pools {
		if len(pool) == 0 {
			t.Errorf("seat %d picked nothing; the PickErr path did not answer", s)
		}
	}
}

// TestFallibleBuilderErrorFailsBuild: a FallibleBuilder whose BuildErr
// errors fails the build, carrying the agent's own error.
func TestFallibleBuilderErrorFailsBuild(t *testing.T) {
	l := testLab(t)
	pool, err := l.SealedPool("FDN", setdata.Arena, 2001, 6)
	if err != nil {
		t.Fatalf("SealedPool: %v", err)
	}
	base := testBuilder(t, l, "best23")
	_, err = l.Build(pool, NamedBuilder{Ref: base.Ref, Builder: &flakyBuilder{Builder: base.Builder}})
	if err == nil {
		t.Fatal("Build succeeded despite the builder's error")
	}
	if !strings.Contains(err.Error(), "draftlab: build") || !strings.Contains(err.Error(), "external builder died") {
		t.Errorf("error %q does not name the build and carry the agent's failure", err)
	}
}
