package draftlab

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
)

// testEventSpec is the 4-seat FDN draft the event tests replay: one
// drafter, one builder, one bot player, round robin by default.
func testEventSpec(seed uint64) EventSpec {
	return EventSpec{
		Set:   "FDN",
		Seats: 4,
		Seed:  seed,
	}
}

// resolveEventAgents fills a spec's agent slices with fresh instances.
func resolveEventAgents(t *testing.T, l *Lab, s EventSpec) EventSpec {
	t.Helper()
	s.Drafters = []NamedDrafter{testDrafter(t, l, "rating-colour")}
	s.Builders = []NamedBuilder{testBuilder(t, l, "best23")}
	s.Players = []NamedPlayer{testPlayer(t, l, "bot")}
	return s
}

// countEntries counts a log's entries of one kind.
func countEntries(t *testing.T, log []byte, kind event.Kind) int {
	t.Helper()
	entries, err := event.ParseLog(log)
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	n := 0
	for _, en := range entries {
		if en.Kind == kind {
			n++
		}
	}
	return n
}

// TestRunEventDraftFourSeat: a default 4-seat draft event replays to
// COMPLETE round robin, six games, four standings, per-seat draft ids, and
// the log names no default.
func TestRunEventDraftFourSeat(t *testing.T) {
	l := testLabOpts(t, Options{Workers: 4})
	s := resolveEventAgents(t, l, testEventSpec(42))
	res, err := l.RunEvent(s)
	if err != nil {
		t.Fatalf("RunEvent: %v", err)
	}
	ev, err := event.ReplayLog(res.Log)
	if err != nil {
		t.Fatalf("ReplayLog: %v", err)
	}
	if !ev.Done() {
		t.Fatalf("event stands in phase %s, want COMPLETE", ev.Phase())
	}
	if got := len(res.Games); got != 6 {
		t.Fatalf("%d games, want 6 (round robin)", got)
	}
	if got := countEntries(t, res.Log, event.KindGameEnded); got != 6 {
		t.Errorf("log has %d GameEnded entries, want 6", got)
	}
	if got := len(res.Standings); got != 4 {
		t.Errorf("%d standings rows, want 4", got)
	}
	if got := res.Decks[2].DraftID; got != draftID(42, 2) {
		t.Errorf("seat 2 draft id %q, want %q", got, draftID(42, 2))
	}
	log := string(res.Log)
	for _, key := range []string{`"mode":`, `"structure":`, `"packs":`, `"rounds":`, `"collation":`} {
		if strings.Contains(log, key) {
			t.Errorf("log carries the default %s key", key)
		}
	}
	if got := res.Manifest.Kind; got != "event" {
		t.Errorf("manifest kind %q, want event", got)
	}
	if res.Manifest.Set != "FDN" || res.Manifest.Mode != "draft" || res.Manifest.Structure != "roundrobin" || res.Manifest.Seats != 4 {
		t.Errorf("manifest event fields %+v", res.Manifest)
	}
}

// TestRunEventSealedEightSwiss: a default 8-seat sealed event runs Swiss
// (the 8-seat default), 3 rounds of 4 series, 6 packs per seat, and the
// structure default is named in the log.
func TestRunEventSealedEightSwiss(t *testing.T) {
	l := testLabOpts(t, Options{Workers: 4})
	s := EventSpec{Set: "FDN", Seats: 8, Mode: event.ModeSealed, Seed: 43}
	s.Builders = []NamedBuilder{testBuilder(t, l, "best23")}
	s.Players = []NamedPlayer{testPlayer(t, l, "bot")}
	res, err := l.RunEvent(s)
	if err != nil {
		t.Fatalf("RunEvent: %v", err)
	}
	spec, err := l.loadSpec("FDN", "")
	if err != nil {
		t.Fatalf("loadSpec: %v", err)
	}
	if got, want := len(res.Games), 12; got != want {
		t.Fatalf("%d games, want %d (3 swiss rounds of 4)", got, want)
	}
	if got := countEntries(t, res.Log, event.KindRoundPaired); got != 3 {
		t.Errorf("log has %d RoundPaired entries, want 3", got)
	}
	for seat, d := range res.Decks {
		if got, want := len(d.Pool), 6*spec.PackSize; got != want {
			t.Errorf("seat %d pool has %d cards, want %d", seat, got, want)
		}
	}
	if got := res.Manifest.Structure; got != "swiss" {
		t.Errorf("manifest structure %q, want swiss (the 8-seat default, still named)", got)
	}
	ev, err := event.ReplayLog(res.Log)
	if err != nil {
		t.Fatalf("ReplayLog: %v", err)
	}
	if !ev.Done() {
		t.Errorf("event stands in phase %s, want COMPLETE", ev.Phase())
	}
	if got := ev.Rounds(); got != 3 {
		t.Errorf("event rounds %d, want 3", got)
	}
}

// TestRunEventStructureNone: a draft event with the none structure ends
// after BUILD with no games.
func TestRunEventStructureNone(t *testing.T) {
	l := testLab(t)
	s := testEventSpec(44)
	s.Seats = 8
	s.Structure = NoGames
	s = resolveEventAgents(t, l, s)
	res, err := l.RunEvent(s)
	if err != nil {
		t.Fatalf("RunEvent: %v", err)
	}
	if got := len(res.Games); got != 0 {
		t.Errorf("%d games, want 0", got)
	}
	if got := len(res.Decks); got != 8 {
		t.Errorf("%d decks, want 8", got)
	}
	ev, err := event.ReplayLog(res.Log)
	if err != nil {
		t.Fatalf("ReplayLog: %v", err)
	}
	if !ev.Done() {
		t.Errorf("event stands in phase %s, want COMPLETE", ev.Phase())
	}
	if got := countEntries(t, res.Log, event.KindPicked); got != 8*3*packSize(t, l, "FDN") {
		t.Errorf("log has %d Picked entries, want %d", got, 8*3*packSize(t, l, "FDN"))
	}
}

// packSize loads a set's pack size at the default collation.
func packSize(t *testing.T, l *Lab, set string) int {
	t.Helper()
	spec, err := l.loadSpec(set, "")
	if err != nil {
		t.Fatalf("loadSpec: %v", err)
	}
	return spec.PackSize
}

// TestRunEventWorkerCountIndependent: the same spec under different
// worker counts gives identical results.
func TestRunEventWorkerCountIndependent(t *testing.T) {
	var want EventResult
	for _, workers := range []int{1, 4} {
		l := testLabOpts(t, Options{Workers: workers})
		s := resolveEventAgents(t, l, testEventSpec(45))
		res, err := l.RunEvent(s)
		if err != nil {
			t.Fatalf("RunEvent: %v", err)
		}
		if workers == 1 {
			want = res
		} else if !reflect.DeepEqual(res, want) {
			t.Fatalf("workers 4 result differs from workers 1")
		}
	}
}

// TestRunEventConcurrentCallsAgree: two concurrent RunEvent calls with
// fresh agent instances agree with each other and a serial baseline.
func TestRunEventConcurrentCallsAgree(t *testing.T) {
	l := testLabOpts(t, Options{Workers: 4})
	s := resolveEventAgents(t, l, testEventSpec(46))
	base, err := l.RunEvent(s)
	if err != nil {
		t.Fatalf("RunEvent: %v", err)
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	results := make([]EventResult, 2)
	// Fresh agent instances per goroutine, resolved on the test goroutine
	// (the agents' own rule; resolve helpers fail via t.Fatalf, which is
	// only legal here).
	agents := make([]EventSpec, 2)
	for i := range agents {
		agents[i] = resolveEventAgents(t, l, testEventSpec(46))
	}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = l.RunEvent(agents[i])
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent RunEvent %d: %v", i, err)
		}
		if !reflect.DeepEqual(results[i], base) {
			t.Errorf("concurrent RunEvent %d differs from the serial baseline", i)
		}
	}
}

// TestVerifyEvent: a RunEvent result verifies; a tampered GameEnded head
// fails naming that game.
func TestVerifyEvent(t *testing.T) {
	l := testLabOpts(t, Options{Workers: 4})
	s := resolveEventAgents(t, l, testEventSpec(42))
	res, err := l.RunEvent(s)
	if err != nil {
		t.Fatalf("RunEvent: %v", err)
	}
	player := testPlayer(t, l, "bot")
	if err := l.VerifyEvent(res, []NamedPlayer{player}); err != nil {
		t.Fatalf("VerifyEvent: %v", err)
	}
	// Tamper with the first GameEnded's chain head in the log bytes.
	alt := alterFirstGameHead(t, res.Log)
	tampered := res
	tampered.Log = alt
	err = l.VerifyEvent(tampered, []NamedPlayer{player})
	if err == nil {
		t.Fatal("VerifyEvent accepted a tampered GameEnded head")
	}
	if !strings.Contains(err.Error(), "seats") || !strings.Contains(err.Error(), "chain head") {
		t.Errorf("error %q does not name the game and the head mismatch", err)
	}
	if !strings.Contains(err.Error(), "round 1 seats") {
		t.Errorf("error %q does not name the tampered game (round 1)", err)
	}
}

// alterFirstGameHead rewrites the first GameEnded log line's head to an
// impossible value.
func alterFirstGameHead(t *testing.T, log []byte) []byte {
	t.Helper()
	lines := strings.Split(string(log), "\n")
	for i, line := range lines {
		if !strings.Contains(line, `"kind":"GameEnded"`) {
			continue
		}
		idx := strings.Index(line, `"head":"`)
		if idx < 0 {
			t.Fatalf("GameEnded line %d has no head field: %s", i, line)
		}
		rest := line[idx+len(`"head":"`):]
		end := strings.Index(rest, `"`)
		if end < 0 {
			t.Fatalf("GameEnded line %d has an unterminated head: %s", i, line)
		}
		lines[i] = line[:idx] + `"head":"deadbeef"` + rest[end+1:]
		return []byte(strings.Join(lines, "\n"))
	}
	t.Fatal("log has no GameEnded line")
	return nil
}

// TestRunEventManifestIsJSON: an event manifest marshals and round-trips
// through encoding/json with its event fields.
func TestRunEventManifestIsJSON(t *testing.T) {
	l := testLab(t)
	s := resolveEventAgents(t, l, testEventSpec(47))
	res, err := l.RunEvent(s)
	if err != nil {
		t.Fatalf("RunEvent: %v", err)
	}
	b, err := json.Marshal(res.Manifest)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var back Manifest
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(back, res.Manifest) {
		t.Errorf("manifest does not round-trip:\n got %+v\nwant %+v", back, res.Manifest)
	}
	if got := res.Manifest.Seed; got != 47 {
		t.Errorf("manifest seed %d, want 47", got)
	}
	if got := len(res.Manifest.Agents); got != 3 {
		t.Errorf("%d agents in the manifest, want drafter+builder+player", got)
	}
}
