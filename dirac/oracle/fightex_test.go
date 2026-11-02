package oracle

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: Rhino's Rampage (Fight ExcessSVar$, CR 120.10,
// gating a reflexive Destroy of a noncreature artifact) and The Last Agni Kai
// (the Excess as red mana, with the Effect UnspentMana Red that keeps it as
// steps end until end of turn) over generated pool fightex. Creatures of
// toughness 1-3 against powers 2-4 give excess > 0 and excess 0; deck B's
// artifacts are the Destroy targets.
//
// Gorge finding: a Rhino's Rampage whose Fight target left before resolution
// (the Pump target is still legal, so the spell resolves) binds no Excess, and
// gorge's ConditionCheckSVar$ Excess then fails OPEN (effects/conditions.go
// CheckSVarCompare: EvalCountOK of the unbound name is not ok, conditionMet
// "resolved=false" runs the gated ImmediateTrigger anyway) and queues the
// reflexive Destroy. Forge reads the unbound SVar as 0 and does nothing; the
// kernel does the same. So deck A is split in two so that no instant-speed
// removal (Shock, The Last Agni Kai) can remove the Fight target in response
// to a Rhino's Rampage: the Rhino variant has no other removal, and the
// kernel is not bent to the gorge quirk.
var kfightexDecks = [4][]cardset.Entry{
	{ // Rhino's Rampage
		{Name: "Rhino's Rampage", Count: 4}, {Name: "Colossal Dreadmaw", Count: 9}, {Name: "Hill Giant", Count: 9},
		{Name: "Mountain", Count: 9}, {Name: "Forest", Count: 9},
	},
	{ // The Last Agni Kai
		{Name: "The Last Agni Kai", Count: 4}, {Name: "Colossal Dreadmaw", Count: 3},
		{Name: "Hill Giant", Count: 4}, {Name: "Shock", Count: 5}, {Name: "Mountain", Count: 13}, {Name: "Forest", Count: 7}, {Name: "Grizzly Bears", Count: 4},
	},
	{
		{Name: "Elite Vanguard", Count: 6}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 3}, {Name: "Colossal Dreadmaw", Count: 2},
		{Name: "Mind Stone", Count: 3}, {Name: "Short Sword", Count: 3}, {Name: "Soul-Guide Lantern", Count: 2},
		{Name: "Plains", Count: 7}, {Name: "Mountain", Count: 5}, {Name: "Forest", Count: 5},
	},
	{ // The Last Agni Kai beside the one Fated Firepower (a second would be the replacement-order ask the kernel stalls): the excess is the damage dealt after the replacement
		{Name: "The Last Agni Kai", Count: 4}, {Name: "Fated Firepower", Count: 1}, {Name: "Hill Giant", Count: 4},
		{Name: "Colossal Dreadmaw", Count: 3}, {Name: "Grizzly Bears", Count: 4}, {Name: "Shock", Count: 3},
		{Name: "Mountain", Count: 18}, {Name: "Forest", Count: 3},
	},
}

// kfightexTally is what gorge's event log showed.
type kfightexTally struct {
	games      int
	rhino      int // Rhino's Rampage spell resolutions
	rhinoExc   int // ... whose reflexive Destroy resolved (excess > 0)
	agniMana   int // The Last Agni Kai ManaAdd events (mana added)
	agniFloats int // turns a ManaClear kept red across a step boundary
}

func kfightexCheck(tal *kfightexTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	seen, turn, lastFloat := 0, 0, -1
	srcName := func(ref *GorgeSide, o *state.Object) string {
		if o != nil && o.Source != 0 {
			o = ref.E.G.Obj(o.Source)
		}
		if o == nil || o.Face() == nil {
			return ""
		}
		return o.Face().Name
	}
	lastResolved := ""
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, seen, turn, lastFloat, lastResolved = ref, 0, 0, -1, ""
			tal.games++
		}
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			ev := evs[seen]
			switch ev.Kind {
			case events.Priority:
				lastResolved = "" // the resolution's own ManaAdd events are over
			case events.TurnChange:
				turn++
			case events.ManaClear:
				if strings.Contains(ev.Text, "R") && lastFloat != turn {

					lastFloat = turn
					tal.agniFloats++
				}
			case events.ManaAdd:
				if ev.Amount > 0 && lastResolved == "The Last Agni Kai" {
					tal.agniMana++
				}
			case events.Resolve:
				o := ref.E.G.Obj(ev.Obj)
				lastResolved = srcName(ref, o)
				if lastResolved != "Rhino's Rampage" {
					continue
				}
				if o.Ability != nil && o.Ability.API == "Destroy" {
					tal.rhinoExc++ // the reflexive Destroy resolved: excess > 0
				} else {
					tal.rhino++
				}
			}
		}
		return nil
	}
}

// kfightexRun plays seeds 1..seeds (even/odd: Rhino/Agni Kai deck; bit 1: seat order; bit 2: autopay surface) and
// returns the tallies, the first divergence ("" if none) and the kernel
// stalls.
func kfightexRun(t *testing.T, seeds uint64) (tal kfightexTally, div string, stalls map[string]int) {
	return kfightexRunDecks(t, seeds, [2]int{0, 1})
}

// kfightexRunDecks is kfightexRun with deck A alternating between the two
// decks of variants.
func kfightexRunDecks(t *testing.T, seeds uint64, variants [2]int) (tal kfightexTally, div string, stalls map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("fightex")
	if err != nil {
		t.Fatal(err)
	}
	var deckCards [4][]*cards.Card
	for s, d := range kfightexDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				deckCards[s] = append(deckCards[s], c)
			}
		}
		if len(deckCards[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(deckCards[s]))
		}
	}
	stalls = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		order := [2]int{variants[seed%2], 2} // variant: Rhino / Agni Kai deck A against deck B
		if seed/2%2 == 1 {
			order = [2]int{2, variants[seed%2]} // both seat orders
		}
		var decks [2][]*cards.Card
		var entries [2][]cardset.Entry
		for i, o := range order {
			decks[i], entries[i] = deckCards[o], kfightexDecks[o]
		}
		tbl, _, _, err := set.Matchup(entries[0], entries[1])
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed/4%2 == 1}
		m, err := NewMatchup(reg, [2]string{"kfx-a", "kfx-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30, Check: kfightexCheck(&tal)})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls[res.Stall]++
		}
		if d := res.Div; d != nil {
			lo := max(d.At-8, 0)
			t.Logf("seed %d last actions %v\ndiff %v %s", seed, actionStrings(res.Actions[lo:d.At]), d.Diff, d.Detail)
			return tal, d.What + " " + d.Field + " seed " + strconv.FormatUint(seed, 10), stalls
		}
	}
	return tal, "", stalls
}

func TestFightexLockstep(t *testing.T) {
	t.Parallel()
	tal, div, stalls := kfightexRun(t, 300)
	t.Logf("%d games; Rhino's Rampage resolutions %d (excess>0 reflexive Destroy: %d, excess 0: %d); The Last Agni Kai mana adds %d; turns with red kept across a step boundary %d; stalls %v",
		tal.games, tal.rhino, tal.rhinoExc, tal.rhino-tal.rhinoExc, tal.agniMana, tal.agniFloats, stalls)
	if div != "" {
		t.Fatalf("diverged: %s", div)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
	if tal.rhinoExc < 10 || tal.rhino-tal.rhinoExc < 5 {
		t.Errorf("Rhino's Rampage excess>0 %d, excess 0 %d, want at least 10 and 5", tal.rhinoExc, tal.rhino-tal.rhinoExc)
	}
	if tal.agniMana < 10 {
		t.Errorf("Agni Kai mana adds %d, want at least 10", tal.agniMana)
	}
	if tal.agniFloats < 3 {
		t.Errorf("turns with red kept across a step boundary %d, want at least 3", tal.agniFloats)
	}
}

// TestFightexFirepowerLockstep: The Last Agni Kai's excess is the damage the
// Fight dealt after Fated Firepower's replacement (CR 120.4a), so the red it
// adds counts the fire counters.
func TestFightexFirepowerLockstep(t *testing.T) {
	t.Parallel()
	tal, div, stalls := kfightexRunDecks(t, 200, [2]int{3, 3})
	t.Logf("%d games; The Last Agni Kai mana adds %d; stalls %v", tal.games, tal.agniMana, stalls)
	if div != "" {
		t.Fatalf("diverged: %s", div)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
	if tal.agniMana < 5 {
		t.Errorf("Agni Kai mana adds %d, want at least 5", tal.agniMana)
	}
}

// TestFightexMutation: a tracked Fight that records 0 excess diverges
// within 128 seeds.
func TestFightexMutation(t *testing.T) {
	dirac.DebugFightExcessZero = true
	defer func() { dirac.DebugFightExcessZero = false }()
	_, div, _ := kfightexRun(t, 128)
	if div == "" {
		t.Fatal("no divergence in 128 games with DebugFightExcessZero")
	}
	t.Logf("mutation diverged: %s", div)
}

// TestFightexUnspentMutation: without the Effect UnspentMana Red mark the
// kernel empties the red at the first step boundary, which diverges.
func TestFightexUnspentMutation(t *testing.T) {
	dirac.DebugUnspentRedOff = true
	defer func() { dirac.DebugUnspentRedOff = false }()
	_, div, _ := kfightexRun(t, 128)
	if div == "" {
		t.Fatal("no divergence in 128 games with DebugUnspentRedOff")
	}
	t.Logf("mutation diverged: %s", div)
}
