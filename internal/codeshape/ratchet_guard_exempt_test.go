package codeshape

import (
	"os"
	"strings"
	"testing"
)

// TestRatchetGuardExemptsTheBotAndTheQueue: the daily bot's pull request lowers ceilings by design and
// a merge-queue branch carries it, so the lint guard must let both through; an ordinary branch and a
// fork's (its ref is withheld, so empty) are still checked.
func TestRatchetGuardExemptsTheBotAndTheQueue(t *testing.T) {
	for ref, want := range map[string]bool{
		"regen/ceilings":               true,
		"mergify/merge-queue/abc123":   true,
		"":                             false, // a fork's branch, or no ref
		"regen/ceilings-copy":          false,
		"feature/lower-ceilings":       false,
		"wt/tf-09-ratchets-post-merge": false,
		"xmergify/merge-queue/abc123":  false,
		"mergify/merge-queue":          false,
	} {
		if got := GuardExempt(ref); got != want {
			t.Errorf("GuardExempt(%q) = %v, want %v", ref, got, want)
		}
	}
}

// TestRatchetGuardWiring: the bot script's branch is the exempt one, and the lint step hands the
// guard the head ref (without it the exemption never applies and the bot's pull request is refused).
func TestRatchetGuardWiring(t *testing.T) {
	script, err := os.ReadFile("../../scripts/lower-ceilings.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script), "branch="+LoweringBranch+"\n") {
		t.Errorf("scripts/lower-ceilings.sh does not push %s", LoweringBranch)
	}
	// An open bot PR that went red or conflicting is rebuilt, or one transient red wedges the job.
	for _, want := range []string{"mergeable,statusCheckRollup", `"CONFLICTING "*`, "rebuilding it over main"} {
		if !strings.Contains(string(script), want) {
			t.Errorf("scripts/lower-ceilings.sh does not rebuild a red or conflicting bot PR (missing %q)", want)
		}
	}
	ci, err := os.ReadFile("../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ci), `go run ./cmd/checkratchet "${{ github.event.pull_request.base.sha }}" "$HEAD_REF" "$HEAD_SHA"`) ||
		!strings.Contains(string(ci), "HEAD_REF: ${{ github.event.pull_request.head.repo.full_name == github.repository && github.head_ref || '' }}") ||
		!strings.Contains(string(ci), "HEAD_SHA: ${{ github.event.pull_request.head.sha }}") {
		t.Error("ci.yml's lint step does not pass the head ref (same-repo only) and head sha to cmd/checkratchet")
	}
}
