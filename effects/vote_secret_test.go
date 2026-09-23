package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func TestSecretVoteFinishedCarrierOmitsRawBallots(t *testing.T) {
	ballots := []VoteBallot{{Player: 0, Pick: 1}, {Player: 1, Pick: 0}}
	if ballots[0].Pick == ballots[1].Pick {
		t.Fatal("test requires distinct private picks")
	}
	got := voteFinishedBallots(ballots, true)
	if len(got) != 0 {
		t.Fatalf("secret ballot carrier retained picks: %+v", got)
	}
	ev := VoteFinishedNote(state.PlayerID(0), 1, got, true)
	if len(ev.Pairs) != 0 {
		t.Fatalf("secret ballot event exposes picks in Pairs: %v", ev.Pairs)
	}
	public := voteFinishedBallots(ballots, false)
	if len(public) != len(ballots) {
		t.Fatalf("public ballot lost picks: %+v", public)
	}
}
