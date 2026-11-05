package draftlab

import (
	"bytes"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
)

// packsBySeat returns each seat's PackOpened card lists in log order.
func packsBySeat(t *testing.T, log []byte, seats int) [][][]limited.CardRef {
	t.Helper()
	entries, err := event.ParseLog(log)
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	out := make([][][]limited.CardRef, seats)
	for _, en := range entries {
		if en.Kind == event.KindPackOpened {
			out[en.Seat] = append(out[en.Seat], en.Cards)
		}
	}
	return out
}

// TestRunEventPrerelease: a sealed event with one promo opens six boosters
// then one rare or mythic promo per seat, every deck builds from the pool
// of all seven packs, and the log replays.
func TestRunEventPrerelease(t *testing.T) {
	l := testLabOpts(t, Options{Workers: 4})
	spec := testEventSpec(7)
	spec.Mode = event.ModeSealed
	spec.Promos = 1
	spec.Structure = NoGames
	res, err := l.RunEvent(resolveEventAgents(t, l, spec))
	if err != nil {
		t.Fatalf("RunEvent: %v", err)
	}
	if got := countEntries(t, res.Log, event.KindPackOpened); got != 4*7 {
		t.Fatalf("%d PackOpened entries, want %d", got, 4*7)
	}
	entries, err := event.ParseLog(res.Log)
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	if entries[0].Promos != 1 {
		t.Fatalf("EventCreated promos = %d, want 1", entries[0].Promos)
	}
	ev, err := event.ReplayLog(res.Log)
	if err != nil {
		t.Fatalf("ReplayLog: %v", err)
	}
	packs := packsBySeat(t, res.Log, 4)
	for st, seatPacks := range packs {
		if len(seatPacks) != 7 {
			t.Fatalf("seat %d opened %d packs, want 7", st, len(seatPacks))
		}
		promo := seatPacks[6]
		if len(promo) != 1 {
			t.Fatalf("seat %d promo pack has %d cards, want 1", st, len(promo))
		}
		if r := promo[0].Rarity; r != limited.Rare && r != limited.Mythic {
			t.Fatalf("seat %d promo %s has rarity %c, want R or M", st, promo[0].Name, r)
		}
		want := 0
		for _, p := range seatPacks {
			want += len(p)
		}
		if got := len(ev.Pool(st)); got != want {
			t.Fatalf("seat %d pool has %d cards, want all %d", st, got, want)
		}
	}
	if len(res.Decks) != 4 {
		t.Fatalf("%d decks, want 4", len(res.Decks))
	}
	for i, d := range res.Decks {
		if d.Build == nil {
			t.Fatalf("deck %d has no build", i)
		}
		if err := build.Validate(ev.Pool(i), *d.Build); err != nil {
			t.Fatalf("deck %d does not validate against its pool: %v", i, err)
		}
	}
}

// TestRunEventPrereleaseRefusals: a promo count outside 0..MaxPromos, or
// promos outside sealed mode, is refused naming the value, before any
// pack is opened (the refusals precede the spec load, so an unknown set
// would otherwise be the error).
func TestRunEventPrereleaseRefusals(t *testing.T) {
	l := testLabOpts(t, Options{Workers: 1})
	cases := []struct {
		name   string
		mode   string
		promos int
		want   string
	}{
		{"draft mode", event.ModeDraft, 1, `promos need sealed mode, got "draft"`},
		{"default mode", "", 1, `promos need sealed mode, got "draft"`},
		{"two promos", event.ModeSealed, 2, "2 promos"},
		{"negative promos", event.ModeSealed, -1, "-1 promos"},
	}
	for _, c := range cases {
		s := resolveEventAgents(t, l, testEventSpec(1))
		s.Set = "NOSUCHSET"
		s.Mode, s.Promos = c.mode, c.promos
		_, err := l.RunEvent(s)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s: err = %v, want it to contain %q", c.name, err, c.want)
		}
	}
}

// TestRunEventSealedUnchangedByPromosField: Promos 0 logs no promos key,
// six PackOpened per seat, and the pools of OpenPod with the same seed.
func TestRunEventSealedUnchangedByPromosField(t *testing.T) {
	l := testLabOpts(t, Options{Workers: 4})
	spec := testEventSpec(11)
	spec.Mode = event.ModeSealed
	spec.Promos = 0
	spec.Structure = NoGames
	res, err := l.RunEvent(resolveEventAgents(t, l, spec))
	if err != nil {
		t.Fatalf("RunEvent: %v", err)
	}
	first, _, _ := bytes.Cut(res.Log, []byte("\n"))
	if !bytes.Contains(first, []byte(event.KindEventCreated)) {
		t.Fatalf("first log line is not EventCreated: %s", first)
	}
	if bytes.Contains(first, []byte(`"promos"`)) {
		t.Fatalf("EventCreated carries a promos key: %s", first)
	}
	want, err := l.OpenPod("FDN", "", 11, 4, 6)
	if err != nil {
		t.Fatalf("OpenPod: %v", err)
	}
	got := packsBySeat(t, res.Log, 4)
	for st := range got {
		if len(got[st]) != 6 {
			t.Fatalf("seat %d opened %d packs, want 6", st, len(got[st]))
		}
		for p := range got[st] {
			assertSameCards(t, st, p, got[st][p], want[st][p])
		}
	}
}

// TestLabOpenPrereleasePod: promos 1 appends one one-card pack to each
// seat's OpenPod packs, which come out identical, and IDs stay unique.
func TestLabOpenPrereleasePod(t *testing.T) {
	l := testLabOpts(t, Options{Workers: 1})
	base, err := l.OpenPod("FDN", "", 5, 4, 6)
	if err != nil {
		t.Fatalf("OpenPod: %v", err)
	}
	pod, err := l.OpenPrereleasePod("FDN", "", 5, 4, 6, 1)
	if err != nil {
		t.Fatalf("OpenPrereleasePod: %v", err)
	}
	if len(pod) != 4 {
		t.Fatalf("%d seats, want 4", len(pod))
	}
	seen := map[string]bool{}
	for st := range pod {
		if len(pod[st]) != 7 {
			t.Fatalf("seat %d has %d packs, want 7", st, len(pod[st]))
		}
		if len(pod[st][6]) != 1 {
			t.Fatalf("seat %d promo pack has %d cards, want 1", st, len(pod[st][6]))
		}
		for p := range pod[st] {
			if p < 6 {
				assertSameCards(t, st, p, pod[st][p], base[st][p])
			}
			for _, c := range pod[st][p] {
				if seen[c.ID] {
					t.Fatalf("duplicate card ID %s", c.ID)
				}
				seen[c.ID] = true
			}
		}
	}
}

func assertSameCards(t *testing.T, seat, pack int, got, want []limited.CardRef) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("seat %d pack %d: %d cards, want %d", seat, pack, len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("seat %d pack %d card %d: %+v, want %+v", seat, pack, i, got[i], want[i])
		}
	}
}
