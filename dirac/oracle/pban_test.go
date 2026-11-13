package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The player-prohibition lockstep: Giant Cindermaw and
// Mornsong Aria make "players can't gain life" / "players can't draw cards"
// statics over lifelink, lifegain triggers and draw spells; the kernel must
// swallow exactly the gains and draws gorge does (replacement_life.go
// lifeGainForbidden, replacement.go drawForbidden).

var kpbanDecks = [][2][]cardset.Entry{{
	{
		{Name: "Giant Cindermaw", Count: 4}, {Name: "Mornsong Aria", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 10},
	},
	{
		{Name: "Healer's Hawk", Count: 4}, {Name: "Vampire Nighthawk", Count: 4}, {Name: "Soul Warden", Count: 4},
		{Name: "Ajani's Pridemate", Count: 4}, {Name: "Divination", Count: 4}, {Name: "Opt", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Island", Count: 8},
	},
}}

type kpbanStats struct {
	games, divs, stalls int
	gains, draws        int // gorge Notes: swallowed gains / draws
}

// kpbanPolicy attacks, blocks, plays and casts whenever it can, else random.
func kpbanPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActAttack, ActBlock, ActCast, ActPlayLand:
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// runPban plays the constructed matchup over seeds 1..seeds; off sets the
// DebugPlayerBanOff mutation.
func runPban(t *testing.T, seeds int, off bool) kpbanStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("pban")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugPlayerBanOff
	dirac.DebugPlayerBanOff = off
	defer func() { dirac.DebugPlayerBanOff = old }()
	var st kpbanStats
	for mi, md := range kpbanDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kpban-a", "kpban-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			g, err := NewGorgeSide(m, seed, cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			res := LockstepLed(m, seed, g, k, kpbanPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			for _, ev := range g.E.L.Events {
				if ev.Kind != events.Note {
					continue
				}
				switch ev.Text {
				case "prevented: cannot gain life":
					st.gains++
				case "prevented: cannot draw cards":
					st.draws++
				}
			}
			st.games++
			if d := res.Div; d != nil {
				st.divs++
				if !off {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				st.stalls++
				if !off {
					t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
				}
			}
		}
	}
	return st
}

func TestPbanLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runPban(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, %d gains swallowed, %d draws swallowed", st.games, st.divs, st.stalls, st.gains, st.draws)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	if st.gains < 30 {
		t.Errorf("only %d gains swallowed, want >= 30", st.gains)
	}
	if st.draws < 30 {
		t.Errorf("only %d draws swallowed, want >= 30", st.draws)
	}
}

// TestPbanMutation turns the bans off in the kernel: it then gains and
// draws where gorge does not, so the games must diverge.
func TestPbanMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runPban(t, 64, true)
	if st.divs == 0 {
		t.Fatalf("mutation left the bans off in %d games with no divergence", st.games)
	}
	t.Logf("mutation diverged in %d of %d games", st.divs, st.games)
}
