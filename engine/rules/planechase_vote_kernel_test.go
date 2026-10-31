package rules

// A Planechase vote: two voters answering different choices tie, the tie runs
// VoteTiedAbility$ alone, and each voter's Note names the choice actually
// answered (not the stand-in's option 0).

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestVoteAnsweredTieRunsVoteTiedAbility(t *testing.T) {
	t.Parallel()
	e := kernelEngine(t, 2)
	spell := putCardIn(t, e, 0, sorceryCard(t, "Council Tie",
		"A:SP$ Vote | Defined$ Player | Choices$ DBGainA,DBGainB | VoteTiedAbility$ DBGainB",
		"SVar:DBGainA:DB$ GainLife | LifeAmount$ 1 | Defined$ You | SpellDescription$ AChoice",
		"SVar:DBGainB:DB$ GainLife | LifeAmount$ 3 | Defined$ You | SpellDescription$ BChoice"), state.ZHand, false)
	life := e.G.Players[0].Life
	from := len(e.L.Events)
	d := requireResumeAsk(t, castWithMana(t, e, 0, spell), "vote")
	voted := map[state.PlayerID]bool{}
	for i := 0; d != nil && i < 4; i++ {
		requireResumeAsk(t, d, "vote")
		voted[d.Player] = true
		// Seat 0 votes for the first choice, seat 1 for the second: a tie.
		d = answerDecision(t, e, d, d.Options[int(d.Player)%len(d.Options)].Index)
	}
	if !voted[0] || !voted[1] {
		t.Fatalf("voters = %v, want both seats", voted)
	}
	if got := e.G.Players[0].Life; got != life+3 {
		t.Fatalf("life = %d, want %d (only VoteTiedAbility$ DBGainB ran)", got, life+3)
	}
	labels := map[string]int{}
	for _, ev := range eventsOfKindFrom(e, from, events.Note) {
		if strings.HasPrefix(ev.Text, "votes for ") {
			labels[ev.Text]++
		}
	}
	if len(labels) != 2 {
		t.Fatalf("vote Notes = %v, want one for each answered choice", labels)
	}
	for k, n := range labels {
		if n != 1 {
			t.Fatalf("vote Note %q logged %d times, want once", k, n)
		}
	}
}
