package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The opponent-chooses TwoPiles shape (Curator of Destinies, Steam Augury:
// "separate them into two piles. An opponent chooses one"). At three or
// more seats the spell's controller picks WHICH opponent chooses (CR
// 801.5b), and that opponent answers the pile pick; the controller keeps the
// chosen pile and the other goes to the graveyard. The mutation
// (DebugPilesNextOpp) has the next live seat choose whatever was picked, the
// old two-seat reading, and must diverge.
var twoPilesChooserPodDeck = []cardset.Entry{
	{Name: "Curator of Destinies", Count: 10}, {Name: "Steam Augury", Count: 4},
	{Name: "Coral Merfolk", Count: 6}, {Name: "Wind Drake", Count: 4},
	{Name: "Island", Count: 16}, {Name: "Mountain", Count: 4},
}

// TestFourSeatTwoPilesChooserLockstep: four-seat Curator of Destinies games
// play in lockstep, the caster choosing a non-first opponent to pick the
// pile, and that opponent answering the pile choice.
func TestFourSeatTwoPilesChooserLockstep(t *testing.T) {
	// The subtests skip without the corpus, so the parent must too, or its
	// watched-count check below fails on an empty run.
	corpusDirOrSkip(t)
	var chosen, controller int
	var pending bool
	var asks, picks int
	oldSteer := podClassSteer
	defer func() { podClassSteer = oldSteer }()
	// The caster always answers the which-opponent ask with its second
	// opponent in AliveFrom order (d.Options is gorge's order), never the
	// first the old two-seat reading took.
	podClassSteer = func(c *PickCtx) (Canon, bool) {
		if c.G == nil || c.G.fr == nil {
			return 0, false
		}
		d := c.G.fr.d
		if d == nil || d.ResumeKind != "twopiles_opponent" || len(d.Options) < 2 {
			return 0, false
		}
		want := RefPlayer + Ref(d.Options[1].Player)
		for _, a := range c.D.Options {
			if a.Tgt() == want {
				return a, true
			}
		}
		return 0, false
	}
	watch := func(c *PickCtx) bool {
		if c.G == nil || c.G.fr == nil {
			return false
		}
		d := c.G.fr.d
		if d == nil {
			return false
		}
		switch d.ResumeKind {
		case "twopiles_opponent":
			if len(d.Options) < 2 {
				return false
			}
			controller, chosen, pending = int(d.Player), int(d.Options[1].Player), true
			if chosen == int(d.Options[0].Player) {
				t.Errorf("the ask offers no non-first opponent: %d, %d", d.Options[0].Player, d.Options[1].Player)
			}
			asks++
			return true
		case "twopiles_pick":
			if !pending {
				return false
			}
			if int(d.Player) != chosen {
				t.Errorf("the pile choice is posed to seat %d, want the chosen opponent %d (controller %d)", d.Player, chosen, controller)
			}
			pending = false
			picks++
			return true
		}
		return false
	}
	podClassLockstep(t, twoPilesChooserPodDeck, 24, 10, watch, &dirac.DebugPilesNextOpp)
	if asks == 0 || picks == 0 {
		t.Fatalf("the run watched no opponent choice (asks %d, picks %d)", asks, picks)
	}
}
