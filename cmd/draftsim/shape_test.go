package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// TestDraftsimEightSeatSwissVerifies pins the 8-seat Swiss shape end to
// end: -seats 8 -structure swiss -players bot drafts an 8-seat pod, pairs
// each of its ceil(log2 8) = 3 rounds through a logged RoundPaired entry,
// plays 12 Bo1 games and ends COMPLETE with eight standings rows, and
// -verify replays the log and every game's chain (run's own gate, so a
// nil error here is the replay verdict).
func TestDraftsimEightSeatSwissVerifies(t *testing.T) {
	dir := corpusReady(t)
	o := options{
		set: "FDN", seed: 1, events: 1,
		seats: 8, structure: "swiss", players: "bot",
		drafter: "rating-colour", builder: "best23", verify: true, dir: dir,
	}
	var out strings.Builder
	res, err := run(o, &out)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.events) != 1 {
		t.Fatalf("run produced %d events, want 1", len(res.events))
	}
	er := res.events[0]

	// Precondition: the log parses and ends at EventEnded, or every
	// assertion below reads nothing.
	entries, err := event.ParseLog(er.log)
	if err != nil {
		t.Fatalf("event log does not parse: %v", err)
	}
	last := entries[len(entries)-1]
	if last.Kind != event.KindEventEnded || len(last.Standings) != 8 {
		t.Fatalf("last log entry is %s with %d standings rows, want EventEnded with 8", last.Kind, len(last.Standings))
	}

	// The header line names the non-default shape.
	if got := strings.Split(out.String(), "\n")[0]; !strings.Contains(got, ", seats 8") || !strings.Contains(got, ", structure swiss") {
		t.Fatalf("header line %q misses the seats 8 / structure swiss suffixes", got)
	}

	// Three Swiss rounds, each opened by a RoundPaired with 4 pairs.
	paired := 0
	for _, en := range entries {
		if en.Kind != event.KindRoundPaired {
			continue
		}
		paired++
		if len(en.Pairs) != 4 {
			t.Fatalf("RoundPaired round %d carries %d pairs, want 4", en.Round, len(en.Pairs))
		}
	}
	if paired != 3 {
		t.Fatalf("log carries %d RoundPaired entries, want 3 (ceil(log2 8) rounds)", paired)
	}

	// 12 games (3 rounds × 4 pairs), every head non-empty, the whole
	// chain of seats 0..7 covered by the standings.
	if len(er.games) != 12 {
		t.Fatalf("event played %d games, want 12", len(er.games))
	}
	for i, g := range er.games {
		if g.head == "" {
			t.Fatalf("game %d has an empty chain head", i)
		}
	}
	seen := map[int]bool{}
	for _, s := range er.standings {
		seen[s.Seat] = true
	}
	if len(seen) != 8 {
		t.Fatalf("standings cover %d seats, want 8", len(seen))
	}
}

// TestDraftsimSealedRoundRobinVerifies pins the sealed shape: -mode sealed
// opens 6 packs per seat straight into its pool (no Picked entries at
// all), builds every seat's deck and plays the 4-seat round robin (6
// games) with -verify semantics.
func TestDraftsimSealedRoundRobinVerifies(t *testing.T) {
	dir := corpusReady(t)
	spec, err := setdata.Load("FDN")
	if err != nil {
		t.Fatalf("setdata FDN: %v", err)
	}
	if spec.PackSize <= 0 {
		t.Fatalf("precondition: FDN pack size is %d, want > 0", spec.PackSize)
	}
	o := options{
		set: "FDN", seed: 1, events: 1,
		mode: "sealed", seats: 4, players: "bot,bot,bot,bot",
		drafter: "rating-colour", builder: "best23", verify: true, dir: dir,
	}
	var out strings.Builder
	res, err := run(o, &out)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	er := res.events[0]

	// Precondition: the log parses; the pack openings really ran.
	entries, err := event.ParseLog(er.log)
	if err != nil {
		t.Fatalf("event log does not parse: %v", err)
	}
	opened := 0
	for _, en := range entries {
		switch en.Kind {
		case event.KindPicked:
			t.Fatal("a sealed event logged a Picked entry; it has no draft")
		case event.KindPackOpened:
			opened++
		}
	}
	if opened != 4*event.DefaultSealedPacks {
		t.Fatalf("sealed event opened %d packs, want %d (4 seats × %d)", opened, 4*event.DefaultSealedPacks, event.DefaultSealedPacks)
	}

	// Every pool is 6 packs of the collation's pack size, and the header
	// line names the mode.
	ev, err := event.ReplayLog(er.log)
	if err != nil {
		t.Fatalf("event log replay: %v", err)
	}
	if !ev.Done() {
		t.Fatalf("replayed event is in phase %s, want COMPLETE", ev.Phase())
	}
	for s := 0; s < ev.NumSeats(); s++ {
		if got := len(ev.Pool(s)); got != event.DefaultSealedPacks*spec.PackSize {
			t.Fatalf("seat %d's pool holds %d cards, want %d (%d packs × %d)", s, got, event.DefaultSealedPacks*spec.PackSize, event.DefaultSealedPacks, spec.PackSize)
		}
	}
	if got := strings.Split(out.String(), "\n")[0]; !strings.Contains(got, ", mode sealed") {
		t.Fatalf("header line %q misses the mode sealed suffix", got)
	}

	// The round robin still plays 6 games with every chain head verified.
	if len(er.games) != 6 {
		t.Fatalf("event played %d games, want 6", len(er.games))
	}
	for i, g := range er.games {
		if g.head == "" {
			t.Fatalf("game %d has an empty chain head", i)
		}
	}
}

// TestDraftsimStructureNoneBuildsOnly pins the self-play-data shape:
// -structure none drafts and builds only — no series are played, both
// events end COMPLETE from BUILD, and the -draftlog export still carries
// every seat's picks under one header.
func TestDraftsimStructureNoneBuildsOnly(t *testing.T) {
	dir := corpusReady(t)
	spec, err := setdata.Load("FDN")
	if err != nil {
		t.Fatalf("setdata FDN: %v", err)
	}
	log := filepath.Join(t.TempDir(), "draft.csv")
	o := options{
		set: "FDN", seed: 1, events: 2,
		seats: 8, structure: "none", players: "bot",
		drafter: "rating-colour", builder: "best23", draftlog: log, dir: dir,
	}
	var out strings.Builder
	res, err := run(o, &out)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(res.events) != 2 {
		t.Fatalf("run produced %d events, want 2", len(res.events))
	}
	for i, er := range res.events {
		if len(er.games) != 0 {
			t.Fatalf("event %d played %d games, want 0 (structure none)", i+1, len(er.games))
		}
		if len(er.decks) != 8 {
			t.Fatalf("event %d built %d decks, want 8", i+1, len(er.decks))
		}
		entries, err := event.ParseLog(er.log)
		if err != nil {
			t.Fatalf("event %d log does not parse: %v", i+1, err)
		}
		last := entries[len(entries)-1]
		if last.Kind != event.KindEventEnded {
			t.Fatalf("event %d ends with %s, want EventEnded", i+1, last.Kind)
		}
		ev, err := event.ReplayLog(er.log)
		if err != nil {
			t.Fatalf("event %d log replay: %v", i+1, err)
		}
		if !ev.Done() {
			t.Fatalf("event %d is in phase %s, want COMPLETE", i+1, ev.Phase())
		}
	}
	if strings.Contains(out.String(), " vs seat ") {
		t.Fatalf("structure none printed game lines:\n%s", out.String())
	}

	// The draftlog: one header, then both events' picks — 8 seats × 3
	// packs × the collation's pack size rows per event.
	raw, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("read draftlog: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if !strings.HasPrefix(lines[0], "expansion,event_type") {
		t.Fatalf("draftlog first line %q is not the header", lines[0])
	}
	headers := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "expansion,event_type") {
			headers++
		}
	}
	if headers != 1 {
		t.Fatalf("draftlog carries %d header lines, want 1", headers)
	}
	want := 2 * 8 * event.DraftPacks * spec.PackSize
	if got := len(lines) - 1; got != want {
		t.Fatalf("draftlog carries %d pick rows, want %d (2 events × 8 seats × 3 packs × %d)", got, want, spec.PackSize)
	}
}

// TestDraftsimShapeFlagRefusals pins the shape validation: every bad
// -seats/-mode/-structure/-players/-draftlog value is refused with an
// error naming the flag, before the corpus opens (dir "." holds no
// ir.gob.gz, so a validation that reached the corpus would fail with a
// different message).
func TestDraftsimShapeFlagRefusals(t *testing.T) {
	base := options{
		set: "FDN", seed: 1, events: 1,
		players: "bot", drafter: "rating-colour", builder: "best23", dir: ".",
	}
	cases := []struct {
		name string
		mut  func(o *options)
		want string
	}{
		{"six seats", func(o *options) { o.seats = 6 }, "-seats must be 4 or 8"},
		{"cube mode", func(o *options) { o.mode = "cube" }, "unknown -mode"},
		{"league structure", func(o *options) { o.structure = "league" }, "unknown -structure"},
		{"two names at 8 seats", func(o *options) { o.seats = 8; o.players = "a,b" }, "want 8"},
		{"sealed draftlog", func(o *options) { o.mode = "sealed"; o.draftlog = "x.csv" }, "-draftlog needs -mode draft"},
	}
	for _, tc := range cases {
		o := base
		tc.mut(&o)
		_, err := run(o, &strings.Builder{})
		if err == nil {
			t.Fatalf("%s: run accepted the shape, want a refusal", tc.name)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: refusal %q does not name %q", tc.name, err, tc.want)
		}
		if strings.Contains(err.Error(), "opening corpus") {
			t.Fatalf("%s: refusal came after the corpus opened: %v", tc.name, err)
		}
	}
}

// TestDraftsimDefaultOutputUnchanged pins the byte-identical contract: a
// run with every shape flag at its default logs an EventCreated with no
// mode/structure/rounds/packs/collation key at all and prints the header
// line exactly as it has always been.
func TestDraftsimDefaultOutputUnchanged(t *testing.T) {
	dir := corpusReady(t)
	o := options{
		set: "FDN", seed: 1, events: 1, players: "bot,bot,bot,bot",
		drafter: "rating-colour", builder: "best23", dir: dir,
	}
	var out strings.Builder
	res, err := run(o, &out)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	logBytes := res.events[0].log
	for _, key := range []string{`"mode":`, `"structure":`, `"rounds":`, `"pairs":`, `"packs":`, `"collation":`} {
		if bytes.Contains(logBytes, []byte(key)) {
			t.Fatalf("default-flag log carries the %s key; the default output moved", key)
		}
	}
	// Precondition: the log really parses and its EventCreated is the
	// entry the assertions above are about.
	entries, err := event.ParseLog(logBytes)
	if err != nil {
		t.Fatalf("event log does not parse: %v", err)
	}
	if entries[0].Kind != event.KindEventCreated {
		t.Fatalf("first entry is %s, want EventCreated", entries[0].Kind)
	}
	first := entries[0]
	if first.Mode != "" || first.Structure != "" || first.Rounds != 0 || first.Packs != 0 || first.Collation != "" {
		t.Fatalf("default EventCreated carries shape %+v, want the zero values", first)
	}
	want := "event 1: seed 1, set FDN (play-arena), drafter rating-colour, builder best23"
	if got := strings.Split(out.String(), "\n")[0]; got != want {
		t.Fatalf("header line = %q, want %q", got, want)
	}
}
