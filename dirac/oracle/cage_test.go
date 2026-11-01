package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Collector's Cage lockstep: "Hideaway 5. {1}, {T}: Put a +1/+1
// counter on target creature you control. Then if you control three or more
// creatures with different powers, you may play the exiled card without
// paying its mana cost." The Hideaway ask (gorge effHideaway), the random
// bottom order (moveRestToBottom), a hidden land played by the effect
// (beginPlay) and a hidden spell cast free must match decision for decision.
var kcageDecks = [][2][]cardset.Entry{{
	{
		{Name: "Collector's Cage", Count: 6}, {Name: "Plains", Count: 9}, {Name: "Forest", Count: 7},
		{Name: "Llanowar Elves", Count: 3}, {Name: "Grizzly Bears", Count: 3}, {Name: "Centaur Courser", Count: 3},
		{Name: "Savannah Lions", Count: 3}, {Name: "Serra Angel", Count: 3}, {Name: "Giant Growth", Count: 3},
	},
	{
		{Name: "Collector's Cage", Count: 4}, {Name: "Plains", Count: 8}, {Name: "Forest", Count: 8},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Centaur Courser", Count: 4},
		{Name: "Savannah Lions", Count: 4}, {Name: "Giant Growth", Count: 4},
	},
}, {
	{
		{Name: "Collector's Cage", Count: 5}, {Name: "Plains", Count: 9}, {Name: "Forest", Count: 6},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Centaur Courser", Count: 4},
		{Name: "Savannah Lions", Count: 4}, {Name: "Serra Angel", Count: 2}, {Name: "Giant Growth", Count: 2},
	},
	{
		{Name: "Plains", Count: 10}, {Name: "Forest", Count: 8}, {Name: "Llanowar Elves", Count: 5},
		{Name: "Grizzly Bears", Count: 5}, {Name: "Centaur Courser", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Serra Angel", Count: 4},
	},
}, {
	{
		{Name: "Windbrisk Heights", Count: 3}, {Name: "Mosswort Bridge", Count: 3}, {Name: "Fight Rigging", Count: 3},
		{Name: "Plains", Count: 8}, {Name: "Forest", Count: 7}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Centaur Courser", Count: 3}, {Name: "Savannah Lions", Count: 3},
		{Name: "Serra Angel", Count: 2}, {Name: "Craw Wurm", Count: 2},
	},
	{
		{Name: "Plains", Count: 10}, {Name: "Forest", Count: 8}, {Name: "Llanowar Elves", Count: 5},
		{Name: "Grizzly Bears", Count: 5}, {Name: "Centaur Courser", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Serra Angel", Count: 4},
	},
}}

// kcageTally counts, on gorge's side, the cards hidden away, and those then
// played from exile: a spell cast (PutOnStack) and a land put onto the
// battlefield.
type kcageTally struct {
	hidden, spells, lands, games int
	// byCard counts, per Hideaway card, the cards it hid and the hidden
	// cards then played
	byCard map[string][2]int
}

// kcageRun plays seeds 1..n of every matchup (n per matchup), failing on
// each divergence (report) or stopping at the first; it returns the number
// of divergent games.
func (tal *kcageTally) note(name string, i int) {
	if tal.byCard == nil {
		tal.byCard = map[string][2]int{}
	}
	c := tal.byCard[name]
	c[i]++
	tal.byCard[name] = c
}

func kcageRun(t *testing.T, n uint64, report bool, tal *kcageTally, mut func(*kop.Instr)) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("cage")
	if err != nil {
		t.Fatal(err)
	}
	div := 0
	for mi, md := range kcageDecks {
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
			hiddenObjs := map[state.ObjID]string{}
			check := func(ref *GorgeSide, _ Side) *Divergence {
				evs := ref.E.L.Events
				for ; seen < len(evs); seen++ {
					ev := evs[seen]
					if tal == nil || ev.Kind != events.MoveZone && ev.Kind != events.PutOnStack {
						continue
					}
					switch {
					case ev.Kind == events.PutOnStack && hiddenObjs[ev.Obj] != "":
						tal.spells++
						tal.note(hiddenObjs[ev.Obj], 1)
					case ev.Kind == events.MoveZone && ev.From == state.ZLibrary && ev.To == state.ZExile && ev.Counter == "exiled_with_face_down_maylook":
						tal.hidden++
						src := cardName(ref.E.G.Obj(ev.IDs[0]).Card)
						hiddenObjs[ev.Obj] = src
						tal.note(src, 0)
					case ev.Kind == events.MoveZone && ev.From == state.ZExile && ev.To == state.ZBattlefield && hiddenObjs[ev.Obj] != "":
						tal.lands++
						tal.note(hiddenObjs[ev.Obj], 1)
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

func TestCageLockstep(t *testing.T) {
	t.Parallel()
	var tal kcageTally
	kcageRun(t, 200, true, &tal, nil)
	t.Logf("%d games: hidden %d, played hidden spells %d, hidden lands %d", tal.games, tal.hidden, tal.spells, tal.lands)
	t.Logf("per card (hidden, played): %v", tal.byCard)
	if pc := tal.byCard; pc["Collector's Cage"][1] < 50 || pc["Fight Rigging"][1] < 5 || pc["Mosswort Bridge"][1] < 5 || pc["Windbrisk Heights"][0] < 5 {
		t.Errorf("per-card counts too low: %v", pc)
	}
	if tal.hidden < 100 || tal.spells < 10 || tal.lands < 10 {
		t.Errorf("hidden %d, spells %d, lands %d; want at least 100, 10 and 10", tal.hidden, tal.spells, tal.lands)
	}
}

// TestCageLockstepMutation: with the Play's condition gate dropped (the
// card is played whatever the powers), the Play dropped, the bottomed cards
// left unshuffled, or a hidden land queued like a spell, some seed diverges.
func TestCageLockstepMutation(t *testing.T) {
	for _, mc := range []struct {
		name string
		mut  func(*kop.Instr)
		dbg  *bool
	}{
		{"condition dropped", func(in *kop.Instr) {
			if in.Op == kop.OpSkipUnless {
				in.Op = kop.OpNop
			}
		}, nil},
		{"play dropped", func(in *kop.Instr) {
			if in.Ext() == kop.ExtHideaway && in.C == 1 {
				*in = kop.Instr{Op: kop.OpNop}
			}
		}, nil},
		{"no shuffle", nil, &dirac.DebugHideNoShuffle},
		{"land queued", nil, &dirac.DebugHideNoLand},
	} {
		if mc.dbg != nil {
			*mc.dbg = true
		}
		n := kcageRun(t, 150, false, nil, mc.mut)
		if mc.dbg != nil {
			*mc.dbg = false
		}
		if n == 0 {
			t.Errorf("mutation %q: no seed diverged", mc.name)
		}
	}
}
