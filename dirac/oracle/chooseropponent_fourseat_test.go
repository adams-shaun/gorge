package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/effects"
)

// chooseropponent_fourseat_test.go: a library search an opponent answers
// (Chooser$ Opponent: Realms Uncharted's "An opponent chooses two of those
// cards", Guided Passage's per-type picks) plays four-seat games in lockstep.
// With two or more live opponents the caster chooses WHICH one answers (CR
// 608.2d; the CR 801.5b example), and that seat answers the search's picks.
var chooserOpponentPodDeck = []cardset.Entry{
	{Name: "Realms Uncharted", Count: 8}, {Name: "Guided Passage", Count: 4},
	{Name: "Llanowar Elves", Count: 4}, {Name: "Grizzly Bears", Count: 4},
	{Name: "Sol Ring", Count: 2},
	{Name: "Forest", Count: 10}, {Name: "Island", Count: 4}, {Name: "Mountain", Count: 4},
}

// TestFourSeatChooserOpponentLockstep: the caster always names its second
// opponent in AliveFrom order, never the next live seat, and that opponent
// answers the search pick that follows. The mutation (DebugSearchOppNext) has
// the next live seat answer whatever was picked and must diverge.
func TestFourSeatChooserOpponentLockstep(t *testing.T) {
	// The subtests skip without the corpus, so the parent must too, or its
	// watched-count check below fails on an empty run.
	corpusDirOrSkip(t)
	var chosen, asks int
	picks := map[string]int{} // opponent-answered picks per searching card
	oldSteer := podClassSteer
	defer func() { podClassSteer = oldSteer }()
	podClassSteer = func(c *PickCtx) (Canon, bool) {
		if c.G == nil || c.G.fr == nil {
			return 0, false
		}
		d := c.G.fr.d
		if d == nil || d.ResumeKind != "search_opponent" || len(d.Options) < 2 {
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
		case "search_opponent":
			if len(d.Options) < 2 {
				return false
			}
			chosen = int(d.Options[1].Player)
			asks++
			return true
		case "search":
			// Only the picks an opponent answers (the caster's own head
			// search has no Chooser$), while two or more opponents live:
			// with one left nobody is asked which.
			if effects.ChangeZoneOf(d.ResumeSA).Chooser != "Opponent" || liveSeats(c) < 3 {
				return false
			}
			if int(d.Player) != chosen {
				t.Errorf("the search pick is posed to seat %d, want the chosen opponent %d", d.Player, chosen)
			}
			picks[c.G.E.G.Obj(d.Source).Face().Name]++
			return true
		}
		return false
	}
	podClassLockstep(t, chooserOpponentPodDeck, 24, 10, watch, &dirac.DebugSearchOppNext)
	if asks < 5 || picks["Realms Uncharted"] < 5 || picks["Guided Passage"] == 0 {
		t.Fatalf("the run watched too few opponent choices (asks %d, picks %v)", asks, picks)
	}
	t.Logf("asks %d, picks %v", asks, picks)
}

// liveSeats counts the players still in gorge's game.
func liveSeats(c *PickCtx) (n int) {
	for _, p := range c.G.E.G.Players {
		if !p.Lost {
			n++
		}
	}
	return n
}
