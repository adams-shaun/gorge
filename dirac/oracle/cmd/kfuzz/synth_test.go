package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/oracle"
)

// TestFuzzSynthRuns plays a dealt pool from memory: the verdict and the reach line
// print, and two runs print the same.
func TestFuzzSynthRuns(t *testing.T) {
	corpus, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("gorge corpus: %v", err)
	}
	if _, err := os.Stat(filepath.Join(corpus, "cardsfolder")); err != nil {
		t.Skipf("gorge corpus has no cardsfolder: %v", err)
	}
	cardsettest.Synth(t, "std-fdn")
	reach := filepath.Join(t.TempDir(), "reach.tsv")
	args := []string{"-synth", "std-fdn", "-play-refused", "-first-pair", "0", "-pairs", "12", "-seed", "1", "-keep-going", "-surface", "autopay", "-shrink", "0", "-workers", "2", "-reach", reach}
	code, stdout, stderr := runKFuzz(t, args...)
	if code > 1 {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	for _, want := range []string{"std-fdn: 12 games (", "reach std-fdn: ", "cards in the list"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout)
		}
	}
	b, err := os.ReadFile(reach)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(b), "card\tstatus\tdecks\tgames\tplays\n") {
		t.Errorf("-reach: %.80q", b)
	}
	_, again, _ := runKFuzz(t, args...)
	if again != stdout {
		t.Errorf("two runs differ:\n%s\n---\n%s", stdout, again)
	}
}

func TestFuzzSynthFlags(t *testing.T) {
	for _, args := range [][]string{
		{"-synth", "std-dsk", "-kernel", "blb"},
		{"-synth", "std-dsk", "-set", "fdn"},
		{"-synth", "std-dsk", "-decks", "/x"},
		{"-synth-decks", "10"},
		{"-reach", "/x"},
	} {
		if code, _, stderr := runKFuzz(t, args...); code != 2 {
			t.Errorf("%v: exit %d, want 2 (stderr %q)", args, code, stderr)
		}
	}
	cardsettest.Synth(t, "std-dsk")
	if code, _, stderr := runKFuzz(t, "-synth", "no-such-pool"); code != 2 || !strings.Contains(stderr, "no-such-pool") {
		t.Errorf("unknown pool: exit %d, stderr %q", code, stderr)
	}
}

// TestFuzzAutopayXCastPaysThroughTheWindow replays the std-sos game whose
// Traumatic Critique, offered because the floating restricted {U}{R} pays its
// pips at X = 0, is cast with X = 6. gorge's planner prices no {X} spell, so
// the seat's cast pays the six through the manual window in battlefield order;
// dirac used to tap a witness plan (three Mountains, three Islands).
//
// The game is a recording (pinnedReplay): std-sos pair 1104 as dealt when the
// cast was fixed.
func TestFuzzAutopayXCastPaysThroughTheWindow(t *testing.T) {
	corpus, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("gorge corpus: %v", err)
	}
	if _, err := os.Stat(filepath.Join(corpus, "cardsfolder")); err != nil {
		t.Skipf("gorge corpus has no cardsfolder: %v", err)
	}
	cardsettest.Synth(t, "std-sos")
	out := pinnedReplay(t, "std-sos", "testdata/autopay_x_cast_pays_through_window.repro.json", "-surface", "autopay")
	m, rest := eventAfter(t, out, `stack_push (p\d) obj=Traumatic Critique#(\d+) zone 1>5`)
	p, id := m[1], m[2]
	m, _ = eventAfter(t, rest, `(?s)^(.*?)cast_info `+p+` obj=Traumatic Critique#`+id+` n=(\d+)\n`)
	window, x := m[1], m[2]
	if x != "6" {
		t.Fatalf("Traumatic Critique cast with X = %s, want 6", x)
	}
	// the six through the manual window: one land tapped for one mana each,
	// beside the restricted floating {U}{R} spent on the pips
	if n := len(regexp.MustCompile(`(?m)^\s*ev \d+ mana_add `+p+` n=1 ctr=[WUBRGC]$`).FindAllString(window, -1)); n != 6 {
		t.Errorf("%d mana tapped in the cast's window, want 6:\n%s", n, window)
	}
	if n := strings.Count(window, `"mana-restriction:`); n != 2 {
		t.Errorf("%d restricted floating mana spent, want 2 ({U}{R}):\n%s", n, window)
	}
}

// TestFuzzFullSpreeCastPaysThroughAFilter replays the std-otj game whose
// Unfortunate Accident is cast with both modes, {3}{B}{B} in all, when the
// second {B} can come only from Conduit Pylons' {1}, {T} any-colour ability.
// gorge's target menu keeps the destroy mode's target (the window can pay),
// where dirac used to reverse the cast at that ask for lack of the filter.
//
// The game is a recording (pinnedReplay): std-otj pair 707 as dealt when the
// cast was fixed.
func TestFuzzFullSpreeCastPaysThroughAFilter(t *testing.T) {
	corpus, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("gorge corpus: %v", err)
	}
	if _, err := os.Stat(filepath.Join(corpus, "cardsfolder")); err != nil {
		t.Skipf("gorge corpus has no cardsfolder: %v", err)
	}
	cardsettest.Synth(t, "std-otj")
	out := pinnedReplay(t, "std-otj", "testdata/spree_full_cast_pays_through_filter.repro.json")
	m, rest := eventAfter(t, out, `stack_push (p\d) obj=Unfortunate Accident#(\d+) zone 1>5`)
	p, id := m[1], m[2]
	// both modes, then the destroy mode's target, then the window taps the
	// caster's Conduit Pylons
	_, rest = eventAfter(t, rest, `mode_chosen `+p+` obj=Unfortunate Accident#`+id+` "Create a 1/1 red Mercenary[^\n]*,Destroy target creature\."`)
	_, rest = eventAfter(t, rest, `targets_chosen p\d obj=Unfortunate Accident#`+id+` `)
	eventAfter(t, rest, `^(?s:.*?)tap p\d obj=Conduit Pylons#\d+\n\s*ev \d+ mana_add `+p+` n=1 `)
}

// TestFuzzCleanupLegendRuleGivesPriority replays the std-otj pair where an
// Oko's copy of an Elk ends at cleanup beside a second Oko: the legend rule's
// keep choice, answered in the cleanup step, is a state-based action that
// performed, so the active player gets priority there (CR 514.3a, 704.5j)
// and the turn does not pass at once.
func TestFuzzCleanupLegendRuleGivesPriority(t *testing.T) {
	corpus, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("gorge corpus: %v", err)
	}
	if _, err := os.Stat(filepath.Join(corpus, "cardsfolder")); err != nil {
		t.Skipf("gorge corpus has no cardsfolder: %v", err)
	}
	cardsettest.Synth(t, "std-otj")
	code, stdout, stderr := runKFuzz(t, "-synth", "std-otj", "-first-pair", "231", "-pairs", "1", "-seed", "1", "-surface", "full", "-shrink", "0", "-absolute")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q\n%s", code, stderr, stdout)
	}
	if !strings.Contains(stdout, "1 clean, 0 divergent") {
		t.Errorf("pair 231 diverged:\n%s", stdout)
	}
}

// TestFuzzPlottedCastSpendIsNotAResponseCastsPayment replays the game where
// a plotted Slickshot Lockpicker is cast for no mana and the same player then
// casts This Town Ain't Big Enough in response, paying with floating mana: no
// mana was spent to cast the Lockpicker, so Satoru, the Infiltrator's second
// alternative holds when it enters and Satoru triggers (CR 601.2h).
//
// The game is a recording (std-otj pair 806 as dealt when the rule was fixed),
// replayed with its own decks and actions, so a re-deal of the synthetic pool
// cannot move the scenario away; the test asserts the event sequence, not
// only a clean verdict.
func TestFuzzPlottedCastSpendIsNotAResponseCastsPayment(t *testing.T) {
	corpus, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("gorge corpus: %v", err)
	}
	if _, err := os.Stat(filepath.Join(corpus, "cardsfolder")); err != nil {
		t.Skipf("gorge corpus has no cardsfolder: %v", err)
	}
	cardsettest.Synth(t, "std-otj")
	const repro = "testdata/plotted_cast_response_payment.repro.json"
	rep, err := oracle.ReadRepro(repro)
	if err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runKFuzz(t, "-synth", "std-otj", "-repro", repro, "-events", "100000")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q\n%s", code, stderr, stdout)
	}
	// the whole recording played, in lockstep
	if want := fmt.Sprintf("repro: no divergence in %d decisions", len(rep.Actions)); !strings.Contains(stdout, want) {
		t.Fatalf("want %q:\n%s", want, stdout)
	}
	// the scenario, in order, in gorge's event log: the plotted Lockpicker
	// cast from exile, This Town cast in response by the same player and paid
	// from the pool, the Lockpicker resolving, and Satoru's trigger on it
	lock := regexp.MustCompile(`stack_push (p\d) obj=Slickshot Lockpicker#\d+ zone 4>5`).FindStringSubmatchIndex(stdout)
	if lock == nil {
		t.Fatalf("no plotted Slickshot Lockpicker cast from exile:\n%s", stdout)
	}
	p, rest := stdout[lock[2]:lock[3]], stdout[lock[1]:]
	for _, step := range []string{
		`stack_push ` + p + ` obj=This Town Ain't Big Enough#\d+ zone 1>5`,
		`mana_add ` + p + ` n=-\d`,
		`stack_resolve p\d obj=Slickshot Lockpicker#\d+`,
		`trigger_push ` + p + ` obj=Satoru, the Infiltrator#\d+ "triggered ability" Slickshot Lockpicker#\d+`,
	} {
		loc := regexp.MustCompile(step).FindStringIndex(rest)
		if loc == nil {
			t.Fatalf("after the plotted cast, no %q:\n%s", step, stdout)
		}
		rest = rest[loc[1]:]
	}
}

// pinnedReplay replays a recorded game (a kfuzz repro in testdata: decks,
// seed and actions) against the synthetic pool's kernel with gorge's events,
// so a re-deal of the pool cannot move a pinned scenario away. It fails
// unless the whole recording plays in lockstep, and returns kfuzz's output.
func pinnedReplay(t *testing.T, pool, path string, args ...string) string {
	t.Helper()
	rep, err := oracle.ReadRepro(path)
	if err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runKFuzz(t, append([]string{"-synth", pool, "-repro", path, "-events", "100000"}, args...)...)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q\n%s", code, stderr, stdout)
	}
	if want := fmt.Sprintf("repro: no divergence in %d decisions", len(rep.Actions)); !strings.Contains(stdout, want) {
		t.Fatalf("want %q:\n%s", want, stdout)
	}
	return stdout
}

// eventAfter finds the first match of re in out and returns its submatches
// and the text after it; it fails the test when there is none.
func eventAfter(t *testing.T, out, re string) ([]string, string) {
	t.Helper()
	loc := regexp.MustCompile(re).FindStringSubmatchIndex(out)
	if loc == nil {
		t.Fatalf("no event matching %q in:\n%s", re, out)
	}
	m := make([]string, len(loc)/2)
	for i := range m {
		if loc[2*i] >= 0 {
			m[i] = out[loc[2*i]:loc[2*i+1]]
		}
	}
	return m, out[loc[1]:]
}
