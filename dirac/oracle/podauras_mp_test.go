package oracle

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// Four-seat pods of auras whose static reads the defender, the classes the
// Commander pod census (TestCommanderPodTwoSeatCensus) once held to two
// seats:
//
//   - Vow of Duty and Vow of Lightning ("enchanted creature can't attack you
//     or planeswalkers you control", CantAttack Target$
//     You,Planeswalker.YouCtrl) forbid the attack on the aura's controller
//     alone. gorge attackOffersCompute drops that defender's player and
//     planeswalker pairs (combat.AttackBlocked) and offers every other
//     living opponent's; dirac's attackOffersN reads the same pair test per
//     defender (combat_cantattack.go).
//   - Shiny Impetus and Psychic Impetus ("enchanted creature is goaded",
//     Goad$ True) goad for the aura's controller (gorge staticGoaders), so
//     the goaded creature attacks a player other than that controller if
//     able (CR 701.15b, gorge GoadMayAttack); dirac's goadDefenderBanned
//     reads the static's controller as a goader (act_goad.go goadedBy).

var vowPodDeck = []cardset.Entry{
	{Name: "Vow of Duty", Count: 7}, {Name: "Vow of Lightning", Count: 5},
	{Name: "Savannah Lions", Count: 4}, {Name: "Elite Vanguard", Count: 4},
	{Name: "Goblin Piker", Count: 2}, {Name: "Raging Goblin", Count: 2},
	{Name: "Plains", Count: 9}, {Name: "Mountain", Count: 7},
}

var impetusPodDeck = []cardset.Entry{
	{Name: "Shiny Impetus", Count: 7}, {Name: "Psychic Impetus", Count: 5},
	{Name: "Goblin Piker", Count: 4}, {Name: "Raging Goblin", Count: 4},
	{Name: "Gray Ogre", Count: 4},
	{Name: "Mountain", Count: 10}, {Name: "Island", Count: 6},
}

// podAuraStats counts the games and, at gorge's attackers decisions, the
// creatures of the active seat enchanted by a watched aura another seat
// controls (the pod case).
type podAuraStats struct {
	games, divs, stalls, foreign int
}

// podAuraPolicy casts, plays lands and attacks whenever offered, else
// random, counting foreign watched auras at attackers decisions.
func podAuraPolicy(seed uint64, auras []string, st *podAuraStats) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && c.D.Kind == DAttackers {
			g := c.G.E.G
			for p := range g.Players {
				for _, aid := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
					a := g.Obj(aid)
					if a == nil || a.Controller == g.Active || a.Face() == nil || !slices.Contains(auras, a.Face().Name) {
						continue
					}
					if host := g.Obj(a.AttachedTo); host != nil && host.Controller == g.Active && host.Zone == state.ZBattlefield {
						st.foreign++
					}
				}
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActAttack, ActPlayLand:
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

// runPodAuras plays seeds four-seat games of deck in every seat, lowered in
// process (kgen.TableFromCards), with mut set for the run.
func runPodAuras(t *testing.T, deck []cardset.Entry, auras []string, seeds int, mut *bool, on bool) podAuraStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	var pool []*cards.Card
	for _, e := range deck {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		pool = append(pool, c)
	}
	pd, err := kgen.TableFromCards(reg, pool, "podaura-oracle")
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	tbl, ids, err := set.MatchupN([][]cardset.Entry{deck, deck, deck, deck})
	if err != nil {
		t.Fatal(err)
	}
	if dirac.TwoSeatOnly(tbl) {
		t.Fatalf("the %v table is still two-seat only: %v", auras, dirac.TwoSeatCauses(tbl))
	}
	old := *mut
	*mut = on
	defer func() { *mut = old }()
	var st podAuraStats
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		m, err := KernelMatchupN(reg, tbl, []string{"pod-a", "pod-b", "pod-c", "pod-d"}, ids, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{MaxDecisions: 4000, MaxTurns: 40}
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, podAuraPolicy(seed, auras, &st), rc)
		CloseSide(k)
		st.games++
		if d := res.Div; d != nil {
			st.divs++
			if !on {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
			}
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			st.stalls++
			if !on {
				t.Errorf("seed %d: stalled: %s", seed, res.Stall)
			}
		}
	}
	return st
}

var (
	vowAuras     = []string{"Vow of Duty", "Vow of Lightning"}
	impetusAuras = []string{"Shiny Impetus", "Psychic Impetus"}
)

// TestFourSeatVowLockstep: four-seat Vow games play in lockstep, with Vows
// on creatures other seats control at attackers decisions.
func TestFourSeatVowLockstep(t *testing.T) {
	st := runPodAuras(t, vowPodDeck, vowAuras, 24, &dirac.DebugCantAtkPodBlind, false)
	t.Logf("%d games, %d divergences, %d stalls, %d foreign Vows at attackers decisions", st.games, st.divs, st.stalls, st.foreign)
	if st.foreign < 10 {
		t.Errorf("only %d foreign Vows at attackers decisions, want >= 10", st.foreign)
	}
}

// TestFourSeatVowMutation offers every defender's pairs as if no Vow stood
// (DebugCantAtkPodBlind): the games must diverge.
func TestFourSeatVowMutation(t *testing.T) {
	st := runPodAuras(t, vowPodDeck, vowAuras, 24, &dirac.DebugCantAtkPodBlind, true)
	if st.divs == 0 {
		t.Fatalf("mutation left %d games with no divergence", st.games)
	}
	t.Logf("mutation diverged in %d of %d games", st.divs, st.games)
}

// TestFourSeatStaticGoadLockstep: four-seat Impetus games play in lockstep,
// with an Impetus on a creature another seat controls at attackers
// decisions.
func TestFourSeatStaticGoadLockstep(t *testing.T) {
	st := runPodAuras(t, impetusPodDeck, impetusAuras, 24, &dirac.DebugStatGoadNoGoader, false)
	t.Logf("%d games, %d divergences, %d stalls, %d foreign Impetus auras at attackers decisions", st.games, st.divs, st.stalls, st.foreign)
	if st.foreign < 10 {
		t.Errorf("only %d foreign Impetus auras at attackers decisions, want >= 10", st.foreign)
	}
}

// TestFourSeatStaticGoadMutation reads a static goad as goading for no seat
// (DebugStatGoadNoGoader), so its controller stays on offer as a defender:
// the games must diverge.
func TestFourSeatStaticGoadMutation(t *testing.T) {
	st := runPodAuras(t, impetusPodDeck, impetusAuras, 24, &dirac.DebugStatGoadNoGoader, true)
	if st.divs == 0 {
		t.Fatalf("mutation left %d games with no divergence", st.games)
	}
	t.Logf("mutation diverged in %d of %d games", st.divs, st.games)
}
