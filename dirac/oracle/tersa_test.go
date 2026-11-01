package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Tersa Lightshatter lockstep: "Whenever Tersa attacks, if there
// are seven or more cards in your graveyard, exile a card at random from your
// graveyard. You may play that card this turn." The random pick is gorge's
// engine-rng draw (effHiddenPick AtRandom$), which the kernel must repeat
// draw for draw; the burn and Mind Rot fill the graveyards.
var ktersaDecks = [][2][]cardset.Entry{{
	{
		{Name: "Tersa Lightshatter", Count: 10}, {Name: "Mountain", Count: 12}, {Name: "Shock", Count: 8},
		{Name: "Mind Rot", Count: 4}, {Name: "Giant Growth", Count: 3}, {Name: "Forest", Count: 3},
	},
	{
		{Name: "Tersa Lightshatter", Count: 6}, {Name: "Mountain", Count: 12}, {Name: "Shock", Count: 10},
		{Name: "Mind Rot", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Forest", Count: 4},
	},
}, {
	{
		{Name: "Tersa Lightshatter", Count: 8}, {Name: "Mountain", Count: 14}, {Name: "Shock", Count: 6},
		{Name: "Mind Rot", Count: 6}, {Name: "Llanowar Elves", Count: 6},
	},
	{
		{Name: "Tersa Lightshatter", Count: 8}, {Name: "Mountain", Count: 14}, {Name: "Shock", Count: 6},
		{Name: "Mind Rot", Count: 6}, {Name: "Grizzly Bears", Count: 6},
	},
}}

// ktersaTally counts, on gorge's side, the cards exiled from a graveyard
// (Tersa's random exile) and those then cast from exile.
type ktersaTally struct {
	exiled, played, games int
}

// ktersaRun plays seeds 1..n of every matchup (n per matchup), failing on
// each divergence (report) or stopping at the first; it returns the number
// of divergent games.
func ktersaRun(t *testing.T, n uint64, report bool, tal *ktersaTally, mut func(*kop.Instr)) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("tersa")
	if err != nil {
		t.Fatal(err)
	}
	div := 0
	for mi, md := range ktersaDecks {
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
						mut(&tbl.Cards[ci].Abilities[ai].Prog[ii])
					}
				}
			}
		}
		for seed := uint64(1); seed <= n; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"ts-a", "ts-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			seen := 0
			exiledObjs := map[state.ObjID]bool{}
			check := func(ref *GorgeSide, _ Side) *Divergence {
				evs := ref.E.L.Events
				for ; seen < len(evs); seen++ {
					ev := evs[seen]
					if tal == nil {
						continue
					}
					if ev.Kind == events.PutOnStack && exiledObjs[ev.Obj] {
						tal.played++
						continue
					}
					if ev.Kind != events.MoveZone {
						continue
					}
					if ev.From == state.ZGraveyard && ev.To == state.ZExile {
						tal.exiled++
						exiledObjs[ev.Obj] = true
					}
				}
				return nil
			}
			res := Lockstep(m, seed, k, pkPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40, Check: check})
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
					t.Logf("matchup %d seed %d diverges (decision %d: %s)", mi, seed, d.At, d.What)
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

func TestTersaLockstep(t *testing.T) {
	t.Parallel()
	var tal ktersaTally
	ktersaRun(t, 300, true, &tal, nil)
	t.Logf("%d games: graveyard cards exiled at random %d, cast from exile %d", tal.games, tal.exiled, tal.played)
	if tal.exiled < 30 || tal.played < 5 {
		t.Errorf("exiled %d, played %d; want at least 30 and 5", tal.exiled, tal.played)
	}
}

// TestTersaLockstepMutation: with the pick no longer random (the kernel
// asks instead of drawing), some seed diverges.
func TestTersaLockstepMutation(t *testing.T) {
	mut := func(in *kop.Instr) {
		if in.Op == kop.OpChooseMove {
			in.B = kop.Const(0)
		}
	}
	if n := ktersaRun(t, 100, false, nil, mut); n == 0 {
		t.Errorf("mutation: no seed diverged")
	}
}
