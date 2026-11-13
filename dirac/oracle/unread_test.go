package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The unread-params lockstep (the unread-params family of the
// MageZero sample decks, CARD-CENSUS.md): constructed matchups over the
// generated unread pool, so the expected answer at every decision is
// gorge's. The carriers: World Shaper's Optional$ mill and Cori-Steel
// Cutter's Optional$ attach (OpMayAsk); Evolving Adaptive's and Sharp-Eyed
// Rookie's Condition$ Evolve (Tf2Evolve); Reckless Impulse's Dig Imprint$
// (inert on a spell); Citadel Gate's and Sea Gate's ChooseColor Exclude$; Ascendant Packleader's
// etbCounter IsPresent$; Brainstorm's hand put-back (two cards on top in
// answer order); Lembas' Defined$ shuffle-in with an inert ChangeNum$/Mandatory$.

var unreadDecks = [][2][]cardset.Entry{{
	{
		{Name: "Cori-Steel Cutter", Count: 4}, {Name: "World Shaper", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Giant Growth", Count: 4}, {Name: "Raging Goblin", Count: 4}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Mountain", Count: 9}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Savannah Lions", Count: 4}, {Name: "Deep-Cavern Bat", Count: 4}, {Name: "Doom Blade", Count: 4},
		{Name: "Mind Rot", Count: 2}, {Name: "Agonizing Memories", Count: 2}, {Name: "Night's Whisper", Count: 4}, {Name: "Walking Corpse", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8},
	},
}, {
	{
		{Name: "Evolving Adaptive", Count: 4}, {Name: "Sharp-Eyed Rookie", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Ascendant Packleader", Count: 4}, {Name: "Centaur Courser", Count: 3}, {Name: "Craw Wurm", Count: 3},
		{Name: "Giant Growth", Count: 1}, {Name: "Forest", Count: 17},
	},
	{
		{Name: "Savannah Lions", Count: 4}, {Name: "Vampire Interloper", Count: 4}, {Name: "Doom Blade", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Raging Goblin", Count: 4}, {Name: "Walking Corpse", Count: 4},
		{Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 8},
	},
}, {
	{
		{Name: "Citadel Gate", Count: 4}, {Name: "Sea Gate", Count: 4}, {Name: "Floodpits Drowner", Count: 4},
		{Name: "Brainstorm", Count: 4}, {Name: "Lembas", Count: 3}, {Name: "Snap", Count: 3},
		{Name: "Doom Blade", Count: 2}, {Name: "Thirst for Discovery", Count: 3}, {Name: "Plains", Count: 7}, {Name: "Island", Count: 3}, {Name: "Thriving Isle", Count: 3},
	},
	{
		{Name: "Reckless Impulse", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Raging Goblin", Count: 4},
		{Name: "Cori-Steel Cutter", Count: 3}, {Name: "Grizzly Bears", Count: 4}, {Name: "Giant Growth", Count: 1}, {Name: "Gleeful Demolition", Count: 2},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 8},
	},
}}

// unreadTally counts, on the gorge side, each carrier ability that left
// the stack (source name / API, a spell as name / Spell) and each gorge
// ask by ResumeKind.
type unreadTally struct {
	pending  map[state.ObjID]string
	resolved map[string]int
	asks     map[string]int
	lastAsk  any
}

func (t *unreadTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	if d := ref.E.Pending(); d != nil && d.ResumeKind != "" && any(d) != t.lastAsk {
		t.lastAsk = d
		t.asks[d.ResumeKind]++
	}
	on := map[state.ObjID]bool{}
	for _, id := range g.Stack {
		o := g.Obj(id)
		if o == nil {
			continue
		}
		on[id] = true
		if _, ok := t.pending[id]; ok {
			continue
		}
		if o.Ability == nil {
			if o.Face() != nil {
				t.pending[id] = o.Face().Name + "/Spell"
			}
			continue
		}
		name := ""
		if src := g.Obj(o.Source); src != nil && src.Face() != nil {
			name = src.Face().Name
		}
		t.pending[id] = name + "/" + o.Ability.API
	}
	for id, key := range t.pending {
		if !on[id] {
			t.resolved[key]++
			delete(t.pending, id)
		}
	}
	return nil
}

func TestUnreadLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("unread")
	if err != nil {
		t.Fatal(err)
	}
	tal := &unreadTally{resolved: map[string]int{}, asks: map[string]int{}}
	games := 0
	for mi, md := range unreadDecks {
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
		for seed := uint64(1); seed <= 96; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kunread-a", "kunread-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := castActivateKindPolicy(seed)
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			tal.pending = map[state.ObjID]string{}
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: tal.check})
			CloseSide(k)
			games++
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			if res.Stall != "" && strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
		}
	}
	t.Logf("%d games; resolved %v; asks %v", games, tal.resolved, tal.asks)
	for _, want := range []struct {
		key string
		min int
	}{
		{"World Shaper/Mill", 5},
		{"World Shaper/ChangeZoneAll", 2},
		{"Cori-Steel Cutter/Token", 5},
		{"Evolving Adaptive/PutCounter", 5},
		{"Sharp-Eyed Rookie/PutCounter", 5},
		{"Reckless Impulse/Spell", 5},
		{"Ascendant Packleader/PutCounter", 5},
		{"Brainstorm/Spell", 5},
		{"Lembas/ChangeZone", 3},
		{"Agonizing Memories/Spell", 3},
		{"Floodpits Drowner/ChangeZone", 3},
		{"Deep-Cavern Bat/ChangeZone", 5},
		{"Gleeful Demolition/Spell", 5},
		{"Thirst for Discovery/Spell", 5},
		{"Snap/Spell", 5},
	} {
		if tal.resolved[want.key] < want.min {
			t.Errorf("%s resolved %d times, want at least %d", want.key, tal.resolved[want.key], want.min)
		}
	}
	for _, want := range []struct {
		kind string
		min  int
	}{
		{"mill_optional", 5},
		{"attach_optional", 5},
		{"discard_unless", 5},
		{"untap", 5},
	} {
		if tal.asks[want.kind] < want.min {
			t.Errorf("gorge asked %s %d times, want at least %d", want.kind, tal.asks[want.kind], want.min)
		}
	}
}
