package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"strings"
)

// The free exile filter populations without IsRemembered
// (dirac/kgen/lower_playexilepop.go; the walk in dirac/cast_freecast.go):
// Krang & Shredder's end-step cast of a nonland card exiled with it
// (Valid$ Card.nonLand+ExiledWithSource, the opponents' exiled cards) and
// Kylox, Visionary Inventor's cast of any number of imprinted instants and
// sorceries from its dug cards (Valid$ Card.IsImprinted+YouOwn).
const playExileFilterPopPool = "playexilepop"

func playExileFilterPopDeck() []string {
	return pod4Deck(deckOf(4, "Krang & Shredder"), deckOf(4, "Kylox, Visionary Inventor"), deckOf(4, "Shock"), deckOf(3, "Opt"),
		deckOf(2, "Divination"), deckOf(4, "Grizzly Bears"), deckOf(4, "Runeclaw Bear"), deckOf(3, "Hill Giant"),
		deckOf(4, "Island"), deckOf(4, "Mountain"), deckOf(2, "Forest"), deckOf(2, "Swamp"))
}

func playExileFilterPopDecks() [2][]cardset.Entry {
	var out [2][]cardset.Entry
	for i := range out {
		for _, n := range playExileFilterPopDeck() {
			out[i] = append(out[i], cardset.Entry{Name: n, Count: 1})
		}
	}
	return out
}

// pefTally counts the free plays begun from exile off a population carrier.
type pefTally struct{ begun, kylox, asks int }

func playExileFilterPopPolicy(t *pefTally) func(uint64) Policy {
	return func(seed uint64) Policy {
		rnd := RandomPolicy(seed)
		return PolicyFunc(func(c *PickCtx) Canon {
			buf := make([]Canon, 0, 8)
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActCastFace, ActActivate, ActPlayLand, ActAttack, ActMode:
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
				playExileFilterPopCount(c, a, t)
			}
			return a
		})
	}
}

func playExileFilterPopCount(c *PickCtx, a Canon, t *pefTally) {
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
	if ob != nil && ob.Zone == state.ZExile && strings.EqualFold(strings.TrimSpace(pd.ResumeSA.ParamStr(cards.PKWithoutManaCost)), "True") {
		t.begun++
		if strings.Contains(pd.ResumeSA.ParamStr(cards.PKValidSA), "Instant") {
			t.kylox++
		}
	}
}

func TestPlayExileFilterPopLockstep(t *testing.T) {
	tally := new(pefTally)
	if n := lockstepRunPolicy(t, playExileFilterPopPool, playExileFilterPopDecks(), 64, true, nil, nil, playExileFilterPopPolicy(tally)); n != 0 {
		t.Fatalf("%d diverged", n)
	}
	t.Logf("play asks %d, free exile plays begun %d (Kylox %d)", tally.asks, tally.begun, tally.kylox)
	if tally.kylox == 0 || tally.begun == tally.kylox {
		t.Fatal("no free exile filter play began from one of Krang & Shredder, Kylox: the lockstep proves nothing")
	}
}

func TestFourSeatPlayExileFilterPopLockstep(t *testing.T) {
	d := playExileFilterPopDeck()
	if n := pod4FixtureLockstep(t, playExileFilterPopPool, [4][]string{d, d, d, d}, 24, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
}

// TestPlayExileFilterPopMutation: with the population ignoring its filter,
// Krang & Shredder offers lands and other seats' cards, so a game diverges.
func TestPlayExileFilterPopMutation(t *testing.T) {
	old := dirac.DebugPlayExilePopNoFilter
	dirac.DebugPlayExilePopNoFilter = true
	defer func() { dirac.DebugPlayExilePopNoFilter = old }()
	if n := lockstepRunPolicy(t, playExileFilterPopPool, playExileFilterPopDecks(), 64, false, nil, nil, playExileFilterPopPolicy(new(pefTally))); n == 0 {
		t.Fatal("ignoring the filter changed no game: the pool does not reach it")
	}
}
