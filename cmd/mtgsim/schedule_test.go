package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// scheduleCase is one schedule line and the single-game simOptions it maps to.
type scheduleCase struct {
	seed      uint64
	format    string
	seats     int
	decks     []string
	policies  []string
	mulligans int
}

func (c scheduleCase) line() string {
	return fmt.Sprintf("%d\t%s\t%d\t%s\t%s\t%d", c.seed, c.format, c.seats,
		strings.Join(c.decks, ","), strings.Join(c.policies, ","), c.mulligans)
}

func (c scheduleCase) single(dir string) simOptions {
	o := simOptions{Decks: strings.Join(c.decks, ","), Seats: c.seats, Seed: c.seed, Games: 1,
		Verify: true, Dir: dir, Format: c.format, Mulligans: c.mulligans}
	if p := strings.Join(c.policies, ","); p != "" {
		o.Policies = c.policies
	}
	return o
}

func commanderRepoDecks(t *testing.T, n int) []string {
	t.Helper()
	var out []string
	for _, name := range testutil.RepoDeckNames() {
		if len(testutil.RepoDeckFile(t, name).CommanderNames()) > 0 {
			out = append(out, name)
		}
	}
	if len(out) < n {
		t.Fatalf("only %d Commander repo decks, need %d", len(out), n)
	}
	return out[:n]
}

func writeSchedule(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "schedule.tsv")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestScheduleMatchesSingleRuns: a three-line schedule (two constructed
// 2-seat games and a 4-seat commander game, each with its own seed, decks,
// policies and mulligans) prints exactly what three single-game runSim calls
// with the mapped flags print, in order.
func TestScheduleMatchesSingleRuns(t *testing.T) {
	dir := testutil.CorpusDirOrSkip(t)
	legacy := testutil.LegacyDeckNames()
	if len(legacy) < 4 {
		t.Fatalf("only %d legacy decks", len(legacy))
	}
	cases := []scheduleCase{
		{seed: 3, format: "constructed", seats: 2, decks: legacy[0:2], policies: []string{"-", "-"}, mulligans: 0},
		{seed: 11, format: "constructed", seats: 2, decks: []string{legacy[2], legacy[2]}, policies: []string{"lethal-pressure", "-"}, mulligans: 1},
		{seed: 7, format: "commander", seats: 4, decks: commanderRepoDecks(t, 4), policies: []string{"-", "bot", "-", "-"}, mulligans: 0},
	}
	var want bytes.Buffer
	var body strings.Builder
	body.WriteString("# a comment\n\n")
	for _, c := range cases {
		body.WriteString(c.line() + "\n")
		if err := runSim(c.single(dir), &want); err != nil {
			t.Fatalf("single run %s: %v\n%s", c.line(), err, want.String())
		}
	}
	var got bytes.Buffer
	err := runSim(simOptions{Schedule: writeSchedule(t, body.String()), Verify: true, Dir: dir, Format: "constructed", Seats: 4}, &got)
	if err != nil {
		t.Fatalf("schedule: %v\n%s", err, got.String())
	}
	if got.String() != want.String() {
		t.Errorf("schedule output differs from single runs:\n--- schedule\n%s--- single\n%s", got.String(), want.String())
	}
	if n := strings.Count(got.String(), "chain="); n != len(cases) {
		t.Errorf("%d chain= lines, want %d", n, len(cases))
	}
	if n := strings.Count(got.String(), "replay OK"); n != len(cases) {
		t.Errorf("%d replay OK lines, want %d", n, len(cases))
	}
}

// TestScheduleBadLineContinues: a bad line prints SCHEDULE-ERROR under its
// seed, the next line still plays, and the run fails at the end.
func TestScheduleBadLineContinues(t *testing.T) {
	dir := testutil.CorpusDirOrSkip(t)
	legacy := testutil.LegacyDeckNames()
	good := scheduleCase{seed: 5, format: "constructed", seats: 2, decks: legacy[0:2], policies: []string{"-", "-"}}
	body := "9\tconstructed\t2\tno-such-deck,no-other\t-,-\t0\n" +
		"12\tconstructed\t2\t" + legacy[0] + "," + legacy[1] + "\tno-such-policy\t0\n" +
		"x\tconstructed\n" +
		"13\tcommander\t2\t" + legacy[0] + "," + legacy[1] + "\t-,-\t0\n" +
		good.line() + "\n"
	var got bytes.Buffer
	err := runSim(simOptions{Schedule: writeSchedule(t, body), Dir: dir, Format: "constructed", Seats: 4}, &got)
	if err == nil {
		t.Fatalf("schedule with bad lines returned nil:\n%s", got.String())
	}
	out := got.String()
	for _, seed := range []string{"9", "12", "13"} {
		if !strings.Contains(out, "seed "+seed+": SCHEDULE-ERROR ") {
			t.Errorf("no SCHEDULE-ERROR line for seed %s:\n%s", seed, out)
		}
	}
	if !strings.Contains(out, "SCHEDULE-ERROR") || strings.Count(out, "SCHEDULE-ERROR") != 4 {
		t.Errorf("want 4 SCHEDULE-ERROR lines:\n%s", out)
	}
	if !strings.Contains(out, "seed 5: 2 seats,") || !strings.Contains(out, "chain=") {
		t.Errorf("the good line after the bad ones did not play:\n%s", out)
	}
}

// TestScheduleRefusesConflictingFlags: -schedule with a per-game flag is a
// usage error before anything runs.
func TestScheduleRefusesConflictingFlags(t *testing.T) {
	dir := testutil.CorpusDirOrSkip(t)
	p := writeSchedule(t, "")
	for name, o := range map[string]simOptions{
		"decks":    {Decks: "a,b"},
		"seed":     {Seed: 4},
		"games":    {Games: 2},
		"a":        {PolicyA: "bot"},
		"b":        {PolicyB: "bot"},
		"policies": {Policies: []string{"bot"}},
	} {
		o.Schedule, o.Dir, o.Format, o.Seats = p, dir, "constructed", 4
		var buf bytes.Buffer
		var ue usageError
		if err := runSim(o, &buf); !errors.As(err, &ue) {
			t.Errorf("-schedule with %s: err %v, want a usage error", name, err)
		}
	}
}
