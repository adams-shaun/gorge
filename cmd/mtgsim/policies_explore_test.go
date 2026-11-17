package main

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestPoliciesExplore: "explore" is an mtgsim-local policy name (the
// coverage-exploration bot, heuristic.NewExploreBot), not a bots registry
// entry: seatPolicies accepts it, a seeded 2-seat game plays and replays
// with it, and the hosted registry still does not offer it.
func TestPoliciesExplore(t *testing.T) {
	if _, ok := bots.Lookup(explorePolicy); ok {
		t.Fatalf("%q is in the bots registry; it must stay mtgsim-local", explorePolicy)
	}
	got, err := seatPolicies([]string{"explore", "bot"}, "", "", 2)
	if err != nil || !slices.Equal(got, []string{"explore", "bot"}) {
		t.Fatalf("seatPolicies = %q, %v; want [explore bot]", got, err)
	}
	_, err = seatPolicies([]string{"no-such-policy"}, "", "", 2)
	var ue usageError
	if !errors.As(err, &ue) || !strings.HasSuffix(err.Error(), ", "+explorePolicy+")") {
		t.Fatalf("unknown policy error %v; want a usage error listing %q last", err, explorePolicy)
	}

	dir := testutil.CorpusDirOrSkip(t)
	o := simOptions{Seats: 2, Seed: 7, Games: 1, Verify: true, Dir: dir, Format: "constructed", Policies: []string{"explore", "bot"}}
	var buf bytes.Buffer
	if err := runSim(o, &buf); err != nil {
		t.Fatalf("runSim: %v\n%s", err, buf.String())
	}
	out := buf.String()
	for _, want := range []string{"policies=explore,bot\n", "replay OK"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}
