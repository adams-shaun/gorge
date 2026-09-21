package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/events"
)

// TestFlipCoinRememberFlagsAllLoud pins the structural fix for the review's
// MAJOR finding: Forge's per-flip memory parameters (RememberResult$,
// RememberNumber$, RememberLoser$) are not persisted by this build, so each
// present one must produce its own loud Note — an earlier revision named only
// RememberResult$/RememberNumber$ and dropped RememberLoser$ SILENTLY, which
// made Unleash the Flux's "if you lose, repeat" loop never repeat with nothing
// in the log to say why. rememberFlags is the ONE list the gate iterates, so a
// future memory flag cannot be added without appearing here (and thus in the
// note) or failing this test.
func TestFlipCoinRememberFlagsAllLoud(t *testing.T) {
	// Unit: the classifier names every memory flag, in a fixed order.
	saAll := sa(t, "DB$ FlipCoin | RememberResult$ True | RememberNumber$ Wins | RememberLoser$ True")
	got := rememberFlags(saAll)
	want := []string{"RememberResult$", "RememberNumber$", "RememberLoser$"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("rememberFlags = %v, want %v", got, want)
	}
	if rememberFlags(sa(t, "DB$ FlipCoin | WinSubAbility$ X")) != nil {
		t.Fatalf("a flip with no memory flag must name none")
	}

	// End to end: each present flag reaches a loud Note on the resolving flip.
	h, c := fixtureHost(t)
	Resolve(h, c, sa(t, "DB$ FlipCoin | RememberLoser$ True"))
	var loud []string
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "RememberLoser$") {
			loud = append(loud, ev.Text)
		}
	}
	if len(loud) != 1 {
		t.Fatalf("RememberLoser$ produced %d loud notes, want 1: %v", len(loud), loud)
	}
	if !strings.Contains(loud[0], "unread") {
		t.Fatalf("the RememberLoser$ note is not a loud unread warning: %q", loud[0])
	}
}
