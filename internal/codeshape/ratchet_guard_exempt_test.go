package codeshape

import (
	"os"
	"strings"
	"testing"
)

// TestRatchetGuardExemptsTheQueue: a merge-queue branch carries the batch's commits (whose
// ratchets main has already lowered against), so the lint guard must let it through; an ordinary
// branch and a fork's (its ref is withheld, so empty) are still checked.
func TestRatchetGuardExemptsTheQueue(t *testing.T) {
	for ref, want := range map[string]bool{
		"mergify/merge-queue/abc123":   true,
		"":                             false, // a fork's branch, or no ref
		"regen/main":                   false,
		"feature/lower-ratchet":        false,
		"wt/tf-09-ratchets-post-merge": false,
		"xmergify/merge-queue/abc123":  false,
		"mergify/merge-queue":          false,
	} {
		if got := GuardExempt(ref); got != want {
			t.Errorf("GuardExempt(%q) = %v, want %v", ref, got, want)
		}
	}
}

// TestRatchetGuardWiring: the lint step hands the guard the head ref (without it the exemption
// never applies and a merge-queue branch is refused) and the head sha.
func TestRatchetGuardWiring(t *testing.T) {
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
