package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
)

// adNauseamRepeatSA returns Ad Nauseam's real corpus Repeat ability -- the
// brief's named carrier for RepeatOptional$ -- after asserting the shape the
// feature reads is still present. Tying the no-host leaf to the real corpus
// card (rather than an invented SA) is what keeps this test honest if the
// script shape moves.
func adNauseamRepeatSA(t *testing.T) *cards.SA {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Ad Nauseam")
	if !ok {
		t.Fatal("precondition failed: real corpus card Ad Nauseam missing")
	}
	var repeat *cards.SA
	for i := range card.Faces[0].Abilities {
		if card.Faces[0].Abilities[i].API == "Repeat" {
			repeat = card.Faces[0].Abilities[i]
			break
		}
	}
	if repeat == nil {
		t.Fatal("precondition failed: Ad Nauseam carries no Repeat ability")
	}
	if repeat.Params["RepeatOptional"] != "True" {
		t.Fatalf("precondition failed: Ad Nauseam RepeatOptional$ = %q, want True", repeat.Params["RepeatOptional"])
	}
	if repeat.Params["RepeatSubAbility"] == "" {
		t.Fatal("precondition failed: Ad Nauseam Repeat has no RepeatSubAbility$")
	}
	return repeat
}

// TestAdNauseamRepeatOptionalNoHostRunsOneIterationThenStops is the brief's
// deterministic no-host half: on a host whose Ask reports no channel (the
// engine's R-9 degradation), a RepeatOptional$ do/while runs the body EXACTLY
// once and stops -- it neither spins to the 1000-iteration guard nor skips
// the body entirely.
//
// The count is the observable: the do/while body runs before the first
// election (CR 608.2c), and only the election can grant a second run, so a
// host that cannot answer stops after one pass. The test asserts BOTH halves
// so it cannot pass vacuously: the body ran (run == 1, not 0) and it did not
// iterate (run == 1, not 2+), and the election was actually posed (askCount
// == 1 with the repeat_optional resume kind), which proves the feature's
// handler ran rather than the Repeat falling through unregistered.
func TestAdNauseamRepeatOptionalNoHostRunsOneIterationThenStops(t *testing.T) {
	sa := adNauseamRepeatSA(t)

	run := 0
	Register("TestAdNauseamRepeatOptionalNoHostSpy", func(Host, *Ctx, *cards.SA) { run++ })
	t.Cleanup(func() { unregister("TestAdNauseamRepeatOptionalNoHostSpy") })

	h, c := fixtureHost(t)
	// Point the real carrier's RepeatSubAbility name at the spy, the same
	// substitution effects/misc_test.go uses, so the body's iteration count
	// is observable without a dig.
	c.SVars = map[string]string{sa.Params["RepeatSubAbility"]: "DB$ TestAdNauseamRepeatOptionalNoHostSpy"}
	h.askResult = false // the no-decision-channel host

	Resolve(h, c, sa)

	if run != 1 {
		t.Fatalf("no-host RepeatOptional ran the body %d times, want exactly 1 (one pass, then the unanswerable election stops the do/while)", run)
	}
	if h.askCount != 1 {
		t.Fatalf("no-host RepeatOptional posed %d elections, want 1 (the first election is always offered)", h.askCount)
	}
	if h.lastAsk == nil || h.lastAsk.ResumeKind != "repeat_optional" {
		t.Fatalf("no-host election = %+v, want a repeat_optional ask (the feature's handler must have run)", h.lastAsk)
	}
}
