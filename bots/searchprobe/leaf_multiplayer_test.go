package searchprobe

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/view"
)

// TestLeafScorePodStrongestLivingOpponent pins the multiplayer leaf: the actor
// leads one living opponent and trails another, and a lost opponent whose
// score is highest does not count.
func TestLeafScorePodStrongestLivingOpponent(t *testing.T) {
	v := view.View{Players: []view.PlayerView{
		{ID: 0, Life: 40},
		{ID: 1, Life: 30},
		{ID: 2, Life: 45},
		{ID: 3, Life: 100, Lost: true}, // would win a naive max, must be skipped
	}}
	if v.Over {
		t.Fatal("precondition: the pod position must be non-terminal")
	}
	if !(v.Players[0].Life > v.Players[1].Life && v.Players[0].Life < v.Players[2].Life) {
		t.Fatal("precondition: the actor must lead seat 1 and trail seat 2")
	}
	if got, want := LeafScore(v, 0), 40.0-45.0; got != want {
		t.Fatalf("pod leaf %v, want %v (life minus the strongest living opponent)", got, want)
	}
	if got := LeafScore(v, 1); got != 30.0-45.0 {
		t.Fatalf("seat 1 leaf %v, want %v (lost seats and the max are read per actor)", got, 30.0-45.0)
	}
}

// TestLeafScoreLostActorIsTerminal pins a lost actor at the lose score even
// though the game continues for the other seats.
func TestLeafScoreLostActorIsTerminal(t *testing.T) {
	v := view.View{Players: []view.PlayerView{
		{ID: 0, Life: 40, Lost: true},
		{ID: 1, Life: 30},
		{ID: 2, Life: 45},
	}}
	if v.Over {
		t.Fatal("precondition: a lost actor in a running pod is not v.Over")
	}
	if got := LeafScore(v, 0); got != -100000 {
		t.Fatalf("lost actor LeafScore %v, want -100000", got)
	}
	if got := LeafValue(v, 0); got != 0 {
		t.Fatalf("lost actor LeafValue %v, want 0", got)
	}
	if got := LeafScore(v, 1); got != 30.0-45.0 {
		t.Fatalf("living seat 1 leaf %v, want %v (a lost actor is skipped, not winning the max)", got, 30.0-45.0)
	}
}
