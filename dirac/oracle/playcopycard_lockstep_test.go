package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// CopyCard$ True over Defined$ Remembered (dirac/kgen/lower_playcopypop.go;
// gorge effects/play.go, cast_begin.go beginPlay, CR 707.12): Narset,
// Enlightened Exile exiles a cheap noncreature card from a graveyard when it
// attacks, and Roving Actuator does the same behind its Void condition; each
// then casts a copy of the exiled card for free, the original staying in
// exile. The expected answer at every decision is gorge's.
const playCopyCardPool = "playcopycard"

func playCopyCardDeck() []string {
	return pod4Deck(deckOf(4, "Narset, Enlightened Exile"), deckOf(4, "Roving Actuator"), deckOf(4, "Shock"), deckOf(3, "Lightning Bolt"),
		deckOf(2, "Opt"), deckOf(2, "Think Twice"), deckOf(2, "Brute Strength"), deckOf(3, "Grizzly Bears"), deckOf(2, "Runeclaw Bear"),
		deckOf(1, "Hill Giant"), deckOf(4, "Plains"), deckOf(4, "Island"), deckOf(5, "Mountain"))
}

// pcTally counts the play asks over a CopyCard$ True Play and the copies the
// answer began (a "mode" pick of an exiled card).
type pcTally struct{ asks, began int }

func playCopyCardPolicy(t *pcTally) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		return PolicyFunc(func(c *PickCtx) Canon {
			buf := make([]Canon, 0, 8)
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActCastFace, ActActivate, ActPlayLand, ActMode, ActAttack:
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
				playCopyCardCount(c, a, t)
			}
			return a
		})
	}
}

func playCopyCardCount(c *PickCtx, a Canon, t *pcTally) {
	pd := c.G.Pending()
	if pd == nil || pd.ResumeKind != "play" || pd.ResumeSA == nil ||
		!strings.EqualFold(strings.TrimSpace(pd.ResumeSA.ParamStr(cards.PKCopyCard)), "True") {
		return
	}
	t.asks++
	if opt, ok := c.G.GorgeOption(a); ok && opt.Kind == "mode" && opt.Obj != 0 {
		t.began++
	}
}

func playCopyCardDecks() [2][]cardset.Entry {
	var out [2][]cardset.Entry
	for i := range out {
		for _, n := range playCopyCardDeck() {
			out[i] = append(out[i], cardset.Entry{Name: n, Count: 1})
		}
	}
	return out
}

func TestPlayCopyCardLockstep(t *testing.T) {
	tally := new(pcTally)
	if n := lockstepRunPolicy(t, playCopyCardPool, playCopyCardDecks(), 96, true, nil, nil, playCopyCardPolicy(tally)); n != 0 {
		t.Fatalf("%d diverged", n)
	}
	t.Logf("copy play asks %d, copies begun %d", tally.asks, tally.began)
	if tally.began == 0 {
		t.Fatal("no CopyCard$ play began: the lockstep proves nothing about the copy cast")
	}
}

func TestFourSeatPlayCopyCardLockstep(t *testing.T) {
	d := playCopyCardDeck()
	if n := pod4FixtureLockstep(t, playCopyCardPool, [4][]string{d, d, d, d}, 24, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
}

// TestPlayCopyCardMutation: casting the original instead of its copy moves the
// card out of exile, so some game diverges from gorge.
func TestPlayCopyCardMutation(t *testing.T) {
	old := dirac.DebugCopyCastNoCopy
	dirac.DebugCopyCastNoCopy = true
	defer func() { dirac.DebugCopyCastNoCopy = old }()
	if n := lockstepRunPolicy(t, playCopyCardPool, playCopyCardDecks(), 96, false, nil, nil, playCopyCardPolicy(new(pcTally))); n == 0 {
		t.Fatal("casting the original changed no game: the pool does not reach the copy cast")
	}
}
