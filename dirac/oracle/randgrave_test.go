package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The ChangeZone AtRandom$ lockstep (Forge changeHiddenOriginResolve, gorge
// effHiddenPick): Haunted Fengraf, Ghoulraiser, Charmbreaker Devils, Woodland
// Sleuth, Make a Wish and Exalted Flamer of Tzeentch (YouOwn filter) return
// cards at random from the controller's graveyard to the hand, Moldgraf
// Monstrosity and Tomb Tyrant (YouOwn filter, GE3 condition) return creature
// cards at random to the battlefield. The pick is the engine's seeded Rand per
// card over the eligible cards in graveyard order (no draw when the pool is no
// larger than the count), which the kernel repeats draw for draw. Bolt, Shock
// and Murder fill the graveyards with the creatures and spells the picks choose
// among.

var krgDecks = [][2][]cardset.Entry{{
	{
		{Name: "Ghoulraiser", Count: 4}, {Name: "Make a Wish", Count: 4}, {Name: "Woodland Sleuth", Count: 3},
		{Name: "Haunted Fengraf", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Murder", Count: 3},
		{Name: "Shock", Count: 3}, {Name: "Swamp", Count: 7}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Ghoulraiser", Count: 4}, {Name: "Make a Wish", Count: 4}, {Name: "Woodland Sleuth", Count: 3},
		{Name: "Haunted Fengraf", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Murder", Count: 3},
		{Name: "Lightning Bolt", Count: 3}, {Name: "Swamp", Count: 7}, {Name: "Forest", Count: 8},
	},
}, {
	{
		{Name: "Moldgraf Monstrosity", Count: 5}, {Name: "Llanowar Elves", Count: 6}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Murder", Count: 4}, {Name: "Swamp", Count: 8}, {Name: "Forest", Count: 11},
	},
	{
		{Name: "Moldgraf Monstrosity", Count: 5}, {Name: "Llanowar Elves", Count: 6}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Murder", Count: 4}, {Name: "Swamp", Count: 8}, {Name: "Forest", Count: 11},
	},
}, {
	{
		{Name: "Charmbreaker Devils", Count: 6}, {Name: "Lightning Bolt", Count: 6}, {Name: "Shock", Count: 6},
		{Name: "Giant Growth", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 14},
	},
	{
		{Name: "Charmbreaker Devils", Count: 6}, {Name: "Lightning Bolt", Count: 6}, {Name: "Shock", Count: 6},
		{Name: "Giant Growth", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 14},
	},
}, {
	// Exalted Flamer of Tzeentch: the upkeep trigger returns an instant or
	// sorcery card at random from the graveyard to the hand (YouOwn pool). The
	// mana base carries U, R and G: the Flamer costs {2}{U}{R}, Bolt and Shock
	// want R, Giant Growth and the Bears want G.
	{
		{Name: "Exalted Flamer of Tzeentch", Count: 4}, {Name: "Lightning Bolt", Count: 6}, {Name: "Shock", Count: 6},
		{Name: "Giant Growth", Count: 6}, {Name: "Grizzly Bears", Count: 4}, {Name: "Island", Count: 5},
		{Name: "Mountain", Count: 5}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Exalted Flamer of Tzeentch", Count: 4}, {Name: "Lightning Bolt", Count: 6}, {Name: "Shock", Count: 6},
		{Name: "Giant Growth", Count: 6}, {Name: "Grizzly Bears", Count: 4}, {Name: "Island", Count: 5},
		{Name: "Mountain", Count: 5}, {Name: "Forest", Count: 4},
	},
}, {
	// Tomb Tyrant: sacrifice a creature to return a Zombie card at random
	// from the graveyard to the battlefield (YouOwn pool, GE3 condition).
	{
		{Name: "Tomb Tyrant", Count: 4}, {Name: "Ghoulraiser", Count: 6}, {Name: "Scathe Zombies", Count: 6},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Murder", Count: 4}, {Name: "Swamp", Count: 16},
	},
	{
		{Name: "Tomb Tyrant", Count: 4}, {Name: "Ghoulraiser", Count: 6}, {Name: "Scathe Zombies", Count: 6},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Murder", Count: 4}, {Name: "Swamp", Count: 16},
	},
}, {
	// Storm of Memories: the exile branch — an instant or sorcery card at
	// random from the graveyard to exile (YouOwn cmcLE3 pool), then a free
	// cast of it.
	{
		{Name: "Storm of Memories", Count: 4}, {Name: "Lightning Bolt", Count: 8}, {Name: "Shock", Count: 8},
		{Name: "Mountain", Count: 20},
	},
	{
		{Name: "Storm of Memories", Count: 4}, {Name: "Lightning Bolt", Count: 8}, {Name: "Shock", Count: 8},
		{Name: "Mountain", Count: 20},
	},
}}

// krgTally counts, in gorge's event log, the random returns: a card moved from
// the graveyard to the hand (every carrier but Moldgraf) or to the battlefield.
type krgTally struct {
	games, toHand, toBattlefield int
}

// krgRun plays seeds 1..n of every matchup; it returns the number of divergent
// games. mut edits each lowered instruction (the mutation test). A non-empty
// only plays just the matchups whose decks carry that card: a mutation of one
// card's program is a no-op in a matchup without it.
func krgRun(t *testing.T, n uint64, report bool, tal *krgTally, only string, mut func(name string, in *kop.Instr)) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("randgrave")
	if err != nil {
		t.Fatal(err)
	}
	div := 0
	for mi, md := range krgDecks {
		if only != "" && !krgCarries(md, only) {
			continue
		}
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
		if mut != nil {
			for ci := range tbl.Cards {
				for ai := range tbl.Cards[ci].Abilities {
					for ii := range tbl.Cards[ci].Abilities[ai].Prog {
						mut(tbl.Cards[ci].Name, &tbl.Cards[ci].Abilities[ai].Prog[ii])
					}
				}
			}
		}
		for seed := uint64(1); seed <= n; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"rg-a", "rg-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			seen := 0
			check := func(ref *GorgeSide, _ Side) *Divergence {
				evs := ref.E.L.Events
				for ; seen < len(evs); seen++ {
					ev := evs[seen]
					if tal == nil || ev.Kind != events.MoveZone || ev.From != state.ZGraveyard {
						continue
					}
					switch ev.To {
					case state.ZHand:
						tal.toHand++
					case state.ZBattlefield:
						tal.toBattlefield++
					}
				}
				return nil
			}
			res := Lockstep(m, seed, k, eagerPolicy(seed, map[string]int{}), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40, Check: check})
			CloseSide(k)
			if tal != nil {
				tal.games++
			}
			if res.Stall != "" && res.Stall != "turns" {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				div++
				if !report {
					return div
				}
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return div
}

// krgCarries reports whether either deck of a matchup lists the card.
func krgCarries(md [2][]cardset.Entry, name string) bool {
	for _, d := range md {
		for _, e := range d {
			if e.Name == name {
				return true
			}
		}
	}
	return false
}

func TestRandGraveLockstep(t *testing.T) {
	t.Parallel()
	var tal krgTally
	krgRun(t, 150, true, &tal, "", nil)
	t.Logf("%d games: graveyard cards returned to hand %d, to the battlefield %d", tal.games, tal.toHand, tal.toBattlefield)
	if tal.toHand < 40 || tal.toBattlefield < 6 {
		t.Errorf("to hand %d, to battlefield %d; want at least 40 and 6", tal.toHand, tal.toBattlefield)
	}
}

// TestRandGraveLockstepMutation: for each of the six carriers alone, with its
// pick no longer random (the kernel asks instead of drawing), some seed
// diverges. Each carrier plays only the matchups that hold it.
func TestRandGraveLockstepMutation(t *testing.T) {
	t.Parallel()
	carriers := map[string]bool{}
	krgRun(t, 0, false, nil, "", func(name string, in *kop.Instr) {
		if in.Op == kop.OpChooseMove && uint16(in.B.N)&kop.SrRandom != 0 {
			carriers[name] = true
		}
	})
	for _, want := range []string{"Haunted Fengraf", "Ghoulraiser", "Moldgraf Monstrosity", "Charmbreaker Devils", "Woodland Sleuth", "Make a Wish", "Exalted Flamer of Tzeentch", "Tomb Tyrant"} {
		if !carriers[want] {
			t.Fatalf("%s carries no random pick (carriers %v)", want, carriers)
		}
		n := krgRun(t, 150, false, nil, want, func(name string, in *kop.Instr) {
			if name == want && in.Op == kop.OpChooseMove {
				in.B = kop.Const(0)
			}
		})
		if n == 0 {
			t.Errorf("mutation: %s's pick made no seed diverge", want)
		}
	}
}
