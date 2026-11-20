package shadow_test

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The Commander pod verifier: the pod staging verifier (stage_pods_mp_test.go)
// on four seats of 40-card Commander decks, each led by its commander (CR
// 903). A position is staged from one seat's view, the four commanders in
// their zones with their cast counts (CR 903.8) and the commander damage each
// player has taken (CR 903.10a), and held to the replayed state.

const cmdPodGames = 120

// cmdPodCards are the commander (deck index 0) and the seven creatures a
// Commander pod deck holds, one copy each, before its 32 Forests: combat
// keywords only, all in the cmd5 table.
var cmdPodCards = []string{
	"Terrian, World Tyrant", "Ancient Brontodon", "Brushstrider", "Colossal Dreadmaw",
	"Garruk's Companion", "Gigantosaurus", "Kalonian Tusker", "Leatherback Baloth",
}

// cmdPodEnv is the cmd5 pool at four seats, every seat's commander the deck's
// first card. Seeds 0 mod 4 take the first stageable position, 1 mod 4 wait
// for a commander on the battlefield, 2 mod 4 for a commander cast at least
// once from the command zone (twice for 6 mod 8) and 3 mod 4 for commander damage. It skips the
// test with no cmd5 table.
func cmdPodEnv(t *testing.T) *stageEnv {
	t.Helper()
	cardsettest.Require(t, "cmd5")
	set, err := cardset.ByName("cmd5")
	if err != nil {
		t.Fatal(err)
	}
	set = cardset.AllowUnverified(set)
	var list []cardset.Entry
	for _, n := range cmdPodCards {
		list = append(list, cardset.Entry{Name: n, Count: 1})
	}
	list = append(list, cardset.Entry{Name: "Forest", Count: 32})
	tbl, _, _, err := set.Matchup(list, list)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]kop.CardID{}
	for i := range tbl.Cards {
		if tbl.Cards[i].Types&kop.TToken == 0 {
			id[tbl.Cards[i].Name] = kop.CardID(i)
		}
	}
	deck := func(*rand.Rand) []kop.CardID {
		var d []kop.CardID
		for _, n := range cmdPodCards {
			d = append(d, id[n])
		}
		for i := 0; i < 32; i++ {
			d = append(d, id["Forest"])
		}
		return d
	}
	return &stageEnv{
		tbl: tbl, deck: deck, kernel: oracle.SetSide(set, tbl, oracle.Config{}), seats: podSeats,
		matchup: func(m *oracle.Matchup) {
			m.Commander = true
			m.Commanders, m.Partners = [2]int{0, 0}, [2]int{-1, -1}
			for i := range m.More {
				m.More[i].Commander, m.More[i].Partner = 0, -1
			}
		},
		want: func(seed uint64, v *view.View, _ *decision.Decision) bool {
			switch seed % 4 {
			case 1:
				return anyCommander(v, func(p *view.PlayerView, c *view.CardView, _ int) bool {
					for i := range p.Battlefield {
						if p.Battlefield[i].ID == c.ID {
							return true
						}
					}
					return false
				})
			case 2:
				// seeds 6 mod 8 wait for a second cast, a taxed commander (CR 903.8)
				need := int32(1 + seed/4%2)
				return anyCommander(v, func(p *view.PlayerView, _ *view.CardView, k int) bool { return p.CommanderCasts[k] >= need })
			case 3:
				return anyCommander(v, func(p *view.PlayerView, _ *view.CardView, _ int) bool { return len(p.CmdDamage) != 0 })
			}
			return true
		},
	}
}

// anyCommander reports whether f holds for a commander of v: its owner, its
// roster card and its place in the owner's roster.
func anyCommander(v *view.View, f func(p *view.PlayerView, c *view.CardView, k int) bool) bool {
	for i := range v.Players {
		p := &v.Players[i]
		for k := range p.Commanders {
			if f(p, &p.Commanders[k], k) {
				return true
			}
		}
	}
	return false
}

func TestStageCommanderPodsMatchReplay(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	env := cmdPodEnv(t)
	st := newStats()
	failed := 0
	for seed := uint64(1); seed <= cmdPodGames; seed++ {
		if msg := stageGame(reg, env, seed, st, nil); msg != "" {
			t.Error(msg)
			if failed++; failed >= 5 {
				break
			}
		}
	}
	t.Logf("%d Commander pods: %d positions tried, %d staged (by step %v), %d continuation decisions; unstageable by reason %v", cmdPodGames, st.tried, st.staged, st.byStep, st.contSteps, st.reasons)
	t.Logf("first detail of each reason: %v", st.firstDetail)
	t.Logf("staged positions with a commander in a command zone %d, on the battlefield %d, cast (1+) %d, taxed (2+) %d, with commander damage %d",
		st.cmdZone, st.cmdBF, st.cmdCast1, st.cmdCast2, st.cmdDmg)
	if st.cmdZone < 20 || st.cmdBF < 15 || st.cmdCast1 < 15 || st.cmdCast2 < 5 || st.cmdDmg < 10 {
		t.Errorf("the pods exercised too little: command zone %d (want 20), battlefield %d (15), cast %d (15), taxed %d (5), damage %d (10)",
			st.cmdZone, st.cmdBF, st.cmdCast1, st.cmdCast2, st.cmdDmg)
	}
}

// TestStageCommanderMutationsAreCaught: staging that drops a Commander field
// must fail the verifier.
func TestStageCommanderMutationsAreCaught(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	runMutations(t, reg, cmdPodEnv(t), map[string]func(*dirac.World){
		"commander damage cleared": func(w *dirac.World) { w.CmdDamage = [dirac.MaxSeats][dirac.MaxSeats]uint8{} },
		"commander casts cleared":  func(w *dirac.World) { w.CmdCasts = [dirac.MaxSeats]uint8{} },
		"command zone commander moved to the library bottom": func(w *dirac.World) {
			for p := uint8(0); p < w.NSeat; p++ {
				c := w.Cmd[p]
				if c == dirac.NoObj || w.Objs[c].Zone != kop.ZoneCommand {
					continue
				}
				n := uint8(0)
				for i := range w.Objs {
					if o := &w.Objs[i]; o.Owner == p && o.Zone == kop.ZoneLibrary {
						n++
					}
				}
				w.Objs[c].Zone, w.Objs[c].Pos = kop.ZoneLibrary, n
			}
		},
	})
}

// countCommanders tallies the Commander features a staged world holds, once
// per position.
func (st *stageStats) countCommanders(w *dirac.World) {
	if !w.Commander {
		return
	}
	var zone, bf, c1, c2, dmg bool
	for p := uint8(0); p < w.NSeat; p++ {
		if c := w.Cmd[p]; c != dirac.NoObj {
			zone = zone || w.Objs[c].Zone == kop.ZoneCommand
			bf = bf || w.Objs[c].Zone == kop.ZoneBattlefield
		}
		c1, c2 = c1 || w.CmdCasts[p] >= 1, c2 || w.CmdCasts[p] >= 2
		for q := uint8(0); q < w.NSeat; q++ {
			dmg = dmg || w.CmdDamage[q][p] != 0
		}
	}
	for _, f := range []struct {
		on bool
		n  *int
	}{{zone, &st.cmdZone}, {bf, &st.cmdBF}, {c1, &st.cmdCast1}, {c2, &st.cmdCast2}, {dmg, &st.cmdDmg}} {
		if f.on {
			*f.n++
		}
	}
}
