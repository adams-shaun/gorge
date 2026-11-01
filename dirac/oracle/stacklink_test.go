package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The AffectedZone$ Stack lifelink-grant lockstep:
// Heartflame Duelist ("instant and sorcery spells you control have
// lifelink") and Lo and Li, Twin Tutors ("Lesson spells you control have
// lifelink"). gorge reads the keyword off the resolving spell when it deals
// damage (effects/damage.go HasKeyword), so every damage spell the granter's
// controller resolves gains that player life. The kernel reads the same grant
// in srcKw (stackLifelinkKw).

var kstacklinkDecks = [2][]cardset.Entry{
	{
		{Name: "Heartflame Duelist", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Raise the Alarm", Count: 3}, {Name: "Grizzly Bears", Count: 5},
		{Name: "Mountain", Count: 10}, {Name: "Plains", Count: 10},
	},
	{
		{Name: "Lo and Li, Twin Tutors", Count: 4}, {Name: "Firebending Lesson", Count: 6}, {Name: "Combustion Technique", Count: 1},
		{Name: "Hill Giant", Count: 5}, {Name: "Prodigal Sorcerer", Count: 4},
		{Name: "Swamp", Count: 10}, {Name: "Mountain", Count: 10},
	},
}

// One Combustion Technique only: two of them on one creature register two
// exile-instead-of-die replacements, which gorge asks the controller to order
// and the kernel merges.
var kstacklinkSpells = map[string]bool{"Shock": true, "Lightning Bolt": true, "Firebending Lesson": true, "Combustion Technique": true}

// runStacklink plays seeds 1..seeds over two matchups (the decks in both
// seat orders) and counts, from gorge's event log, the life gains that
// follow the resolution of a damage spell (none of the pool's spells has a
// printed lifelink, so each is a granted one): "gain" all of them,
// "lesson" those of a Lesson resolving with Lo and Li on its controller's
// battlefield.
func runStacklink(t *testing.T, seeds int, report bool) (games, divs, stalls int, cnt map[string]int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("stacklink")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][2][]*cards.Card
	var tbls [2]*kop.Table
	for mi := range decks {
		order := [2]int{0, 1}
		if mi == 1 {
			order = [2]int{1, 0}
		}
		var es [2][]cardset.Entry
		for s := range order {
			es[s] = kstacklinkDecks[order[s]]
			for _, e := range es[s] {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[mi][s] = append(decks[mi][s], c)
				}
			}
			if len(decks[mi][s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[mi][s]))
			}
		}
		tbl, _, _, err := set.Matchup(es[0], es[1])
		if err != nil {
			t.Fatal(err)
		}
		tbls[mi] = tbl
	}
	cnt = map[string]int{}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		mi := int(seed/2) % 2
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kstacklink-a", "kstacklink-b"}, decks[mi], 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbls[mi], cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		var pol Policy = HeuristicPolicy(seed)
		if seed%4 == 0 {
			pol = RandomPolicy(seed)
		}
		seen := 0
		topName, topCtrl, topLoLi := "", state.PlayerID(0), false
		check := func(ref *GorgeSide, _ Side) *Divergence {
			evs := ref.E.L.Events
			for ; seen < len(evs); seen++ {
				if ev := evs[seen]; ev.Kind == events.LifeChange && ev.Amount > 0 && kstacklinkSpells[topName] && ev.Player == topCtrl {
					cnt["gain"]++
					if topLoLi && (topName == "Firebending Lesson" || topName == "Combustion Technique") {
						cnt["lesson"]++
					}
				}
			}
			// the stack top after this step is what the next step resolves
			topName = ""
			if n := len(ref.E.G.Stack); n > 0 {
				if o := ref.E.G.Obj(ref.E.G.Stack[n-1]); o != nil && o.Face() != nil {
					topName, topCtrl, topLoLi = o.Face().Name, o.Controller, false
					for _, id := range ref.E.G.Zone(state.ZBattlefield, o.Controller) {
						if bo := ref.E.G.Obj(id); bo != nil && bo.Face() != nil && bo.Face().Name == "Lo and Li, Twin Tutors" {
							topLoLi = true
						}
					}
				}
			}
			return nil
		}
		res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: check})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if report {
				t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if report {
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[max(d.At-8, 0):d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			if !report || divs >= 3 {
				break
			}
		}
	}
	return games, divs, stalls, cnt
}

func TestStacklinkLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, cnt := runStacklink(t, 256, true)
	t.Logf("%d games, %d divergences, %d stalls, granted-lifelink gains %v", games, divs, stalls, cnt)
	if cnt["gain"] < 20 {
		t.Errorf("only %d granted lifelink gains (want >= 20)", cnt["gain"])
	}
	if cnt["lesson"] == 0 {
		t.Errorf("no gain from a Lesson resolving under Lo and Li")
	}
}

// TestStacklinkMutation makes the kernel ignore the granted lifelink: the
// life gains vanish, so the lockstep must diverge.
func TestStacklinkMutation(t *testing.T) {
	old := dirac.DebugStackLifelinkOff
	dirac.DebugStackLifelinkOff = true
	defer func() { dirac.DebugStackLifelinkOff = old }()
	games, divs, _, _ := runStacklink(t, 64, false)
	if divs == 0 {
		t.Fatalf("grant ignored in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
