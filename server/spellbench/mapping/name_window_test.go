package mapping_test

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
)

// A windowed human transport posts the first MaxCandidates names while the
// native decision keeps all of them, so any offered name stays answerable.
func TestNameChoiceWindowThroughTheBuilder(t *testing.T) {
	names := testcorpus.Registry(t).Universe().Names()
	const n = 5000
	if len(names) < n {
		t.Fatalf("precondition: corpus universe has %d names, want >= %d", len(names), n)
	}
	domain := map[string]bool{}
	for _, name := range names {
		domain[name] = true
	}
	g := untilPending(t, "Wildfire", 1, func(*decision.Decision, *rules.Engine) bool { return true })
	env := envFor(t, g)
	env.Domain = domain
	env.NameWindow = mapping.MaxCandidates

	d := &decision.Decision{Kind: decision.KChoose, Player: 0, Seq: 7}
	for i := 0; i < n; i++ {
		d.Options = append(d.Options, decision.Option{Index: i, Kind: "name", Label: names[i]})
	}
	tx, err := mapping.Begin(env, d)
	if err != nil {
		t.Fatal(err)
	}
	pose, err := tx.Pose()
	if err != nil {
		t.Fatal(err)
	}
	if len(pose.Candidates) == len(pose.Native.Options) {
		t.Fatalf("precondition: window did not truncate (%d candidates)", len(pose.Candidates))
	}
	if len(pose.Candidates) != mapping.MaxCandidates || len(pose.Native.Options) != n {
		t.Fatalf("candidates %d (want %d), native options %d (want %d)", len(pose.Candidates), mapping.MaxCandidates, len(pose.Native.Options), n)
	}
	commits, done, err := tx.Answer(mapping.MaxCandidates - 1)
	if err != nil || !done {
		t.Fatalf("Answer: done=%v err=%v", done, err)
	}
	want := []decision.Intent{{Seq: d.Seq, Player: 0, Choices: []int{mapping.MaxCandidates - 1}}}
	if !reflect.DeepEqual(commits, want) {
		t.Fatalf("commits = %+v, want %+v", commits, want)
	}
}
