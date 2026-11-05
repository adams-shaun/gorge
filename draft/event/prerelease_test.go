package event_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
)

func promoCard(seat int, r limited.Rarity) limited.CardRef {
	id := fmt.Sprintf("promo-s%d", seat)
	return limited.CardRef{ID: id, Name: "Promo " + id, Set: "TST", Number: "99", Rarity: r}
}

func prereleaseCreated(mode string, promos int) event.Entry {
	return event.Entry{Kind: event.KindEventCreated, Seed: 42, Set: "TST", Mode: mode, Seats: testSeats(), Promos: promos}
}

// openBoosters opens the six booster rounds for every seat.
func openBoosters(t *testing.T, e *event.Event) {
	t.Helper()
	for p := 1; p <= event.PrereleasePacks; p++ {
		for s := 0; s < 4; s++ {
			mustApply(t, e, event.Entry{Kind: event.KindPackOpened, Round: p, Seat: s, Cards: packCards(p, s)})
		}
	}
}

func newPrerelease(t *testing.T) *event.Event {
	t.Helper()
	e := event.New()
	mustApply(t, e, prereleaseCreated("sealed", 1))
	if e.Promos() != 1 {
		t.Fatalf("Promos() = %d, want 1", e.Promos())
	}
	openBoosters(t, e)
	return e
}

func wantErr(t *testing.T, e *event.Event, en event.Entry, msg string) {
	t.Helper()
	err := e.Apply(en)
	if err == nil || !strings.Contains(err.Error(), msg) {
		t.Errorf("Apply(%s): error %v, want containing %q", en.Kind, err, msg)
	}
}

func TestPrereleaseEventCreatedPromos(t *testing.T) {
	mustApply(t, event.New(), prereleaseCreated("sealed", 1))
	wantErr(t, event.New(), prereleaseCreated("draft", 1), "promos need sealed mode")
	wantErr(t, event.New(), prereleaseCreated("", 1), "promos need sealed mode")
	wantErr(t, event.New(), prereleaseCreated("sealed", 2), "promos 2, want 0..1")
	wantErr(t, event.New(), prereleaseCreated("sealed", -1), "promos -1, want 0..1")
}

func TestPrereleasePromoPackValidated(t *testing.T) {
	e := newPrerelease(t)
	if e.Phase() != event.PhaseDraft {
		t.Fatalf("phase %s after the boosters, want DRAFT", e.Phase())
	}
	round := event.PrereleasePacks + 1
	two := []limited.CardRef{promoCard(0, limited.Rare), promoCard(9, limited.Rare)}
	wantErr(t, e, event.Entry{Kind: event.KindPackOpened, Round: round, Seat: 0, Cards: two}, "promo round 7 holds 2 cards, want 1")
	wantErr(t, e, event.Entry{Kind: event.KindPackOpened, Round: round, Seat: 0, Cards: []limited.CardRef{promoCard(0, limited.Common)}}, "is rarity C, want R or M")
	wantErr(t, e, event.Entry{Kind: event.KindPackOpened, Round: round + 1, Seat: 0, Cards: []limited.CardRef{promoCard(0, limited.Rare)}}, "out of range")
	mustApply(t, e, event.Entry{Kind: event.KindPackOpened, Round: round, Seat: 0, Cards: []limited.CardRef{promoCard(0, limited.Mythic)}})
}

func TestPrereleaseSupplyCompletesAfterPromos(t *testing.T) {
	e := newPrerelease(t)
	round := event.PrereleasePacks + 1
	for s := 0; s < 4; s++ {
		if e.Phase() != event.PhaseDraft {
			t.Fatalf("phase %s before seat %d's promo, want DRAFT", e.Phase(), s)
		}
		mustApply(t, e, event.Entry{Kind: event.KindPackOpened, Round: round, Seat: s, Cards: []limited.CardRef{promoCard(s, limited.Rare)}})
	}
	if e.Phase() != event.PhaseBuild {
		t.Fatalf("phase %s after the last promo, want BUILD", e.Phase())
	}
	for s := 0; s < 4; s++ {
		pool := e.Pool(s)
		if len(pool) != 6*packSize+1 {
			t.Fatalf("seat %d pool has %d cards, want %d", s, len(pool), 6*packSize+1)
		}
		if pool[len(pool)-1] != promoCard(s, limited.Rare) {
			t.Errorf("seat %d pool ends with %+v, want its promo", s, pool[len(pool)-1])
		}
		if got := e.PromoCards(s); len(got) != 1 || got[0] != promoCard(s, limited.Rare) {
			t.Errorf("PromoCards(%d) = %+v", s, got)
		}
	}
	if e.PromoCards(4) != nil || e.PromoCards(-1) != nil {
		t.Error("PromoCards of an out-of-range seat is not nil")
	}
	// A deck maindecking the promo (the pool's last card) validates.
	pool := e.Pool(0)
	d := build.Deck{Basics: [5]int{5, 0, 0, 0, 0}}
	for _, c := range pool[len(pool)-35:] {
		d.Main = append(d.Main, c.ID)
	}
	if d.Main[len(d.Main)-1] != "promo-s0" {
		t.Fatalf("deck does not hold the promo: %v", d.Main[len(d.Main)-1])
	}
	mustApply(t, e, event.Entry{Kind: event.KindDeckSaved, Seat: 0, Rev: 1, Deck: &d})
	if e.PromoCards(0) == nil || newSealedEvent(t, 0).PromoCards(0) != nil {
		t.Error("a promo-less event reports promo cards")
	}
}

func TestPrereleaseStateJSONPromosKey(t *testing.T) {
	plain, err := newSealedEvent(t, 0).StateJSON()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain), `"promos"`) {
		t.Errorf("a promo-less snapshot has a promos key: %.200s", plain)
	}
	with, err := newPrerelease(t).StateJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(with), `"promos":1`) {
		t.Errorf("a promos-1 snapshot lacks \"promos\":1: %.200s", with)
	}
}

func TestPrereleaseLogReplays(t *testing.T) {
	e := newPrerelease(t)
	for s := 0; s < 4; s++ {
		mustApply(t, e, event.Entry{Kind: event.KindPackOpened, Round: event.PrereleasePacks + 1, Seat: s, Cards: []limited.CardRef{promoCard(s, limited.Mythic)}})
	}
	replayMatches(t, e)
}
