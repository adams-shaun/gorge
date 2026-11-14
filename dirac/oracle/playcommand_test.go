package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// api:Play from the command zone (Geode Golem: "you may cast your commander
// from the command zone without paying its mana cost", CR 903.8): Commander
// duels over the playcmd fixture pool, one cheap commander a seat and Geode
// Golems in both libraries, with Shock and blockers so a commander dies and
// goes back to the command zone. The policy casts whenever it can and takes
// the free cast whenever the Play offers it, so the games reach free casts
// at a commander tax of {2} or more, which gorge charges and the kernel must
// charge too.

// playCmdSeats are the two seats: the commander (deck index 0) and its
// 40-card library.
var playCmdSeats = [2]struct {
	name string
	cmdr string
	lib  []cardset.Entry
}{
	{"playcmd-rograkh", "Rograkh, Son of Rohgahh", []cardset.Entry{
		{Name: "Geode Golem", Count: 10}, {Name: "Shock", Count: 6}, {Name: "Gray Ogre", Count: 4},
		{Name: "Mountain", Count: 20},
	}},
	{"playcmd-yoshimaru", "Yoshimaru, Ever Faithful", []cardset.Entry{
		{Name: "Geode Golem", Count: 10}, {Name: "Savannah Lions", Count: 6}, {Name: "Ornithopter", Count: 4},
		{Name: "Plains", Count: 20},
	}},
}

// playCmdTally counts, from gorge's pending asks and its answers, the Geode
// Golem Play asks that offered a commander and the picks that cast one, and
// of those the picks whose commander had been cast from the command zone
// before (so the free cast owes a tax); paid counts the Play casts gorge
// committed, its commander left on the stack (playCmdPaid).
type playCmdTally struct{ asks, picks, taxed, paid int }

// playCmdPolicy casts or activates whenever it can and takes a pick (ActMode)
// over stopping (ActDone), so a Geode Golem's Play casts the commander.
func playCmdPolicy(seed uint64, t *playCmdTally) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		hasDone, hasMode := false, false
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActDone:
				hasDone = true
			case ActMode:
				hasMode = true
			case ActCast, ActActivate:
				buf = append(buf, a)
			}
		}
		var a Canon
		switch {
		case hasDone && hasMode:
			for _, o := range c.D.Options {
				if o.Kind() == ActMode {
					a = o
					break
				}
			}
		case len(buf) > 0:
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		default:
			a = rnd.Pick(c)
		}
		if c.G != nil {
			playCmdCount(c, a, t)
		}
		return a
	})
}

// playCmdCount classifies one answer against gorge's pending decision.
func playCmdCount(c *PickCtx, a Canon, t *playCmdTally) {
	pd := c.G.Pending()
	if pd == nil || pd.ResumeKind != "play" || pd.ResumeSA == nil ||
		strings.TrimSpace(pd.ResumeSA.ParamStr(cards.PKValidZone)) != "Command" {
		return
	}
	t.asks++
	opt, ok := c.G.GorgeOption(a)
	if !ok || opt.Kind != "mode" || opt.Obj == 0 {
		return
	}
	ob := c.G.E.G.Obj(opt.Obj)
	if ob == nil || ob.Zone != state.ZCommand {
		return
	}
	t.picks++
	pl := &c.G.E.G.Players[pd.Player]
	for k, id := range pl.Commanders {
		if id == opt.Obj && pl.CmdCasts[k] > 0 {
			t.taxed++
		}
	}
}

// playCmdPaid counts gorge's Play casts of a commander that were not
// reversed: a mode chosen as "Play <name>", then the commander's push from
// the command zone with no SpellCastReversed before the next priority.
func playCmdPaid(g *GorgeSide) int {
	evs := g.E.L.Events
	n := 0
	for i, ev := range evs {
		if ev.Kind != events.PutOnStack || ev.From != state.ZCommand || i == 0 ||
			evs[i-1].Kind != events.ModeChosen || !strings.HasPrefix(evs[i-1].Text, "Play ") {
			continue
		}
		ok := true
		for _, e := range evs[i+1:] {
			if e.Kind == events.SpellCastReversed {
				ok = false
			}
			if e.Kind == events.Priority || !ok {
				break
			}
		}
		if ok {
			n++
		}
	}
	return n
}

// playCmdRun plays seeds 1..n of the duel in lockstep; it returns the tally
// and the first divergence's seed (0 none).
func playCmdRun(t *testing.T, n uint64) (playCmdTally, uint64) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("playcmd")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	var ents [2][]cardset.Entry
	var names [2]string
	for s, seat := range playCmdSeats {
		names[s] = seat.name
		ents[s] = append([]cardset.Entry{{Name: seat.cmdr, Count: 1}}, seat.lib...)
		for _, e := range ents[s] {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 41 {
			t.Fatalf("%s: %d cards, want a commander and a 40-card library", seat.name, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(ents[0], ents[1])
	if err != nil {
		t.Fatal(err)
	}
	var tally playCmdTally
	for seed := uint64(1); seed <= n; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, names, decks, 0)
		if err != nil {
			t.Fatal(err)
		}
		m.Commander = true
		m.Commanders, m.Partners = [2]int{0, 0}, [2]int{-1, -1}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		for s := 0; s < 2; s++ {
			pl := &g.E.G.Players[s]
			if len(pl.Commanders) != 1 || g.E.G.Obj(pl.Commanders[0]).Zone != state.ZCommand {
				t.Fatalf("%s seed %d: gorge seated %v, want one commander in the command zone", names[s], seed, pl.Commanders)
			}
		}
		res := LockstepLed(m, seed, g, k, playCmdPolicy(seed, &tally), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		tally.paid += playCmdPaid(g)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d: kernel stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[max(d.At-14, 0):d.At]))
			t.Logf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return tally, seed
		}
	}
	return tally, 0
}

// TestPlayCommandZoneLowers: Geode Golem's free commander cast plays in
// lockstep, at least one pick owing the commander tax; with the kernel's tax
// dropped from the free cast (dirac.DebugFreeCmdNoTax) the same games
// diverge.
func TestPlayCommandZoneLowers(t *testing.T) {
	const seeds = 40
	tally, div := playCmdRun(t, seeds)
	t.Logf("%d Geode Golem Play asks, %d commanders cast, %d of them taxed, %d committed", tally.asks, tally.picks, tally.taxed, tally.paid)
	if div != 0 {
		t.Fatalf("seed %d diverged", div)
	}
	// the precondition the agreement depends on: the free cast is reached,
	// and some pick owes the tax the mutation drops
	if tally.picks == 0 || tally.taxed == 0 || tally.paid == 0 {
		t.Fatalf("free casts %d, taxed %d, committed %d: the games never cast a commander at a tax through Geode Golem", tally.picks, tally.taxed, tally.paid)
	}
	dirac.DebugFreeCmdNoTax = true
	defer func() { dirac.DebugFreeCmdNoTax = false }()
	if _, div := playCmdRun(t, seeds); div == 0 {
		t.Fatal("the kernel's free cast with no commander tax still agrees with gorge")
	}
}
