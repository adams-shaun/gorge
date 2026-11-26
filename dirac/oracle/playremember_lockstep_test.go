package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// RememberPlayed$ True (dirac/cast_playremember.go; gorge effects/play.go's
// PlayDone pass): Kellan, the Kid's free cast from hand remembers the begun
// card, and its "if you don't" land arm reads the record. Flashback (Think
// Twice, Faithless Looting) and cascade (Bloodbraid Elf) are the casts from
// anywhere but the hand that wake the trigger.
const playRememberPool = "playremember"

func playRememberDeck() []string {
	return pod4Deck(deckOf(4, "Kellan, the Kid"), deckOf(3, "Helmut Zemo, Mastermind"), deckOf(4, "Think Twice"), deckOf(3, "Faithless Looting"),
		deckOf(3, "Bloodbraid Elf"), deckOf(3, "Shock"), deckOf(1, "Divination"), deckOf(4, "Grizzly Bears"),
		deckOf(2, "Runeclaw Bear"), deckOf(1, "Hill Giant"), deckOf(1, "Opt"),
		deckOf(3, "Plains"), deckOf(2, "Island"), deckOf(3, "Forest"), deckOf(3, "Mountain"))
}

// prTally counts the plays a RememberPlayed$ Play begins (Kellan's free cast
// from the hand, Helmut Zemo's paid cast from the graveyard; each one a
// remembered write in gorge) and the play asks posed.
type prTally struct{ begun, asks int }

// playRememberPolicy takes casts, activations and land plays over passing (so
// the flashback and cascade casts happen) and counts the begun plays from
// gorge's pending "play" decision.
func playRememberPolicy(t *prTally) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		return PolicyFunc(func(c *PickCtx) Canon {
			buf := make([]Canon, 0, 8)
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActCastFace, ActActivate, ActPlayLand, ActMode:
					buf = append(buf, a)
				}
			}
			var a Canon
			if len(buf) > 0 && len(buf) < len(c.D.Options) {
				d := *c.D
				d.Options = buf
				a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			} else {
				a = rnd.Pick(c)
			}
			if c.G != nil {
				playRememberCount(c, a, t)
			}
			return a
		})
	}
}

func playRememberCount(c *PickCtx, a Canon, t *prTally) {
	pd := c.G.Pending()
	if pd == nil || pd.ResumeKind != "play" || pd.ResumeSA == nil {
		return
	}
	t.asks++
	opt, ok := c.G.GorgeOption(a)
	if !ok || opt.Kind != "mode" || opt.Obj == 0 {
		return
	}
	ob := c.G.E.G.Obj(opt.Obj)
	if ob != nil && (ob.Zone == state.ZHand || ob.Zone == state.ZGraveyard) && strings.EqualFold(strings.TrimSpace(pd.ResumeSA.ParamStr(cards.PKRememberPlayed)), "True") {
		t.begun++
	}
}

func playRememberDecks() [2][]cardset.Entry {
	var out [2][]cardset.Entry
	for i := range out {
		for _, n := range playRememberDeck() {
			out[i] = append(out[i], cardset.Entry{Name: n, Count: 1})
		}
	}
	return out
}

func TestPlayRememberPlayedLockstep(t *testing.T) {
	tally := new(prTally)
	if n := lockstepRunPolicy(t, playRememberPool, playRememberDecks(), 64, true, nil, nil, playRememberPolicy(tally)); n != 0 {
		t.Fatalf("%d diverged", n)
	}
	t.Logf("play asks %d, remembered writes (plays begun) %d", tally.asks, tally.begun)
	if tally.begun == 0 {
		t.Fatal("no RememberPlayed$ play began: the lockstep proves nothing about the remembered write")
	}
}

func TestFourSeatPlayRememberPlayedLockstep(t *testing.T) {
	d := playRememberDeck()
	if n := pod4FixtureLockstep(t, playRememberPool, [4][]string{d, d, d, d}, 24, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
}

// TestPlayRememberPlayedMutation: with the write skipped, Kellan's "if you
// don't" arm sees nothing remembered after a cast it did make, so some game
// diverges from gorge.
func TestPlayRememberPlayedMutation(t *testing.T) {
	old := dirac.DebugNoRememberPlayed
	dirac.DebugNoRememberPlayed = true
	defer func() { dirac.DebugNoRememberPlayed = old }()
	if n := lockstepRunPolicy(t, playRememberPool, playRememberDecks(), 64, false, nil, nil, playRememberPolicy(new(prTally))); n == 0 {
		t.Fatal("skipping the remembered write changed no game: the pool does not reach it")
	}
}
