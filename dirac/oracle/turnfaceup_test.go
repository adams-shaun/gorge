package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The SetState Mode$ TurnFaceUp lockstep (kop.ExtTurnFaceUp; gorge
// effSetState's turnUp arm). Zimone, Mystery Unraveler's second landfall a
// turn and Ugin's Mastery's attack trigger each let their controller turn a
// face-down permanent (creature) they control face up: gorge setStateChoose
// poses a "setstate_pick" KChoose, Min 0 Max 1, over the permanents that
// would turn up, so the pick is taken or declined (Done). Nothing is paid,
// and the turn-up fires the turned-face-up triggers of Temur War Shaman
// and Aven Farseer. Break Open and Skirk Alarmist turn up one target (C 0,
// the shape of Expose the Culprit's first mode); a target no longer face
// down is left alone. The policy never takes the turn-face-up special
// action, so every turn-up in these games is an effect's: the picks taken
// account for theirs, the rest are the targets'. Expose the Culprit and
// Hustle // Bustle lower the same op, but neither card lowers whole yet
// (Expose's second mode, Hustle's MustAttack/MustBlock), so they are not in
// this pool.
var kturnupeffDeck = []cardset.Entry{
	{Name: "Zimone, Mystery Unraveler", Count: 4}, {Name: "Ugin's Mastery", Count: 2},
	{Name: "Temur War Shaman", Count: 2}, {Name: "Aven Farseer", Count: 1},
	{Name: "Manifest Dread", Count: 3}, {Name: "Soul Summons", Count: 2},
	{Name: "Memnite", Count: 2}, {Name: "Skirk Alarmist", Count: 3}, {Name: "Break Open", Count: 2},
	{Name: "Rampant Growth", Count: 2}, {Name: "Evolving Wilds", Count: 2},
	{Name: "Mox Emerald", Count: 2}, {Name: "Mox Ruby", Count: 2}, {Name: "Mox Sapphire", Count: 1},
	{Name: "Forest", Count: 4}, {Name: "Island", Count: 2}, {Name: "Mountain", Count: 4},
}

type kturnupeffCounts struct {
	taken, declined int // setstate_pick decisions answered with a permanent / Done
	turnUps         int // gorge events.TurnFaceUp (all an effect's: the special action is never taken)
	upTriggers      int // TurnFaceUp triggers pushed (Temur War Shaman, Aven Farseer)
}

// kturnupeffPolicy never takes the turn-face-up special action, answers a
// setstate_pick with Done one time in three and a random permanent
// otherwise, and else prefers a non-Pass action four times in five.
func kturnupeffPolicy(seed uint64, n *kturnupeffCounts) Policy {
	r := rand.New(rand.NewPCG(seed, 0x7475726e7570)) // "turnup"
	pickKind := ActChoose + CanonKind(func() uint8 { c, _ := chooseCode("setstate_pick"); return c }())
	var buf, act []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf, act = buf[:0], act[:0]
		pick := false
		for _, a := range c.D.Options {
			if a.Kind() == ActSpecial && a.Mode()&15 == kfacedownTurnUp {
				continue
			}
			pick = pick || a.Kind() == pickKind
			buf = append(buf, a)
			if a.Kind() != ActPass && a.Kind() != ActDone {
				act = append(act, a)
			}
		}
		if len(buf) == 0 {
			return c.D.Options[r.IntN(len(c.D.Options))]
		}
		if pick {
			var picks []Canon
			done, haveDone := Canon(0), false
			for _, a := range buf {
				switch a.Kind() {
				case pickKind:
					picks = append(picks, a)
				case ActDone:
					done, haveDone = a, true
				}
			}
			if haveDone && (len(picks) == 0 || r.IntN(3) == 0) {
				n.declined++
				return done
			}
			if len(picks) > 0 {
				n.taken++
				return picks[r.IntN(len(picks))]
			}
		}
		if len(act) > 0 && r.IntN(5) != 0 {
			return act[r.IntN(len(act))]
		}
		return buf[r.IntN(len(buf))]
	})
}

// kturnupeffFinish counts gorge's turn-ups and the TurnFaceUp triggers they
// pushed.
func kturnupeffFinish(n *kturnupeffCounts) func(g *GorgeSide) {
	return func(g *GorgeSide) {
		for _, ev := range g.E.L.Events {
			switch ev.Kind {
			case events.TurnFaceUp:
				n.turnUps++
			case events.TriggerPush:
				o := g.E.G.Obj(ev.Obj)
				if o == nil || o.Face() == nil || int(ev.Amount) >= len(o.Face().Triggers) {
					continue
				}
				if strings.EqualFold(o.Face().Triggers[ev.Amount].Mode, "TurnFaceUp") {
					n.upTriggers++
				}
			}
		}
	}
}

func TestSetStateTurnFaceUpLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("turnupeffect")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range kturnupeffDeck {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kturnupeffDeck, kturnupeffDeck)
	if err != nil {
		t.Fatal(err) // on land: Zimone and Ugin's Mastery refuse SetState Mode$ "TurnFaceUp"
	}
	seeds := uint64(128)
	if testing.Short() {
		seeds = 24
	}
	var n kturnupeffCounts
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"tu-a", "tu-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, kturnupeffPolicy(seed, &n), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 25, Finish: kturnupeffFinish(&n)})
		CloseSide(k)
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: stall %q", seed, res.Stall)
		}
	}
	t.Logf("seeds=%d picks taken=%d declined=%d turn-ups=%d (targeted %d) turned-face-up triggers=%d", seeds, n.taken, n.declined, n.turnUps, n.turnUps-n.taken, n.upTriggers)
	// the preconditions: the pick was posed and answered both ways, and the
	// effect's turn-ups happened and fired triggers
	if n.taken < 5 || n.declined < 3 {
		t.Errorf("setstate_pick taken %d, declined %d: want >= 5 and >= 3", n.taken, n.declined)
	}
	if n.turnUps-n.taken < 5 || n.upTriggers < 3 {
		t.Errorf("targeted turn-ups %d, turned-face-up triggers %d: want >= 5 and >= 3", n.turnUps-n.taken, n.upTriggers)
	}
}
