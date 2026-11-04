package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The CantBeActivated lockstep: the four printed
// S:Mode$ CantBeActivated carriers (Clarion Conqueror: every artifact,
// creature and planeswalker; Grand Abolisher: the opponents' artifacts,
// creatures and enchantments on its controller's turn; the Auras Petrify
// and Stuck in Summoner's Sanctum: the enchanted permanent; The Immortal Sun:
// loyalty abilities only) against mana
// creatures, Mind Stone, a pinger, pumps, an Equipment, an enchantment and
// a planeswalker with activated abilities, so the kernel withholds the same
// activation offers and the same mana sources as gorge's abilityRestricted.

var kcantactDecks = [2][]cardset.Entry{
	// W/G: Clarion Conqueror, Grand Abolisher, Petrify, mana creatures, pumps
	{
		{Name: "Clarion Conqueror", Count: 3}, {Name: "Grand Abolisher", Count: 3}, {Name: "Petrify", Count: 2},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Elvish Mystic", Count: 4}, {Name: "Twinblade Slasher", Count: 2},
		{Name: "Bonesplitter", Count: 2}, {Name: "Mind Stone", Count: 2}, {Name: "Ajani, Caller of the Pride", Count: 2},
		{Name: "The Immortal Sun", Count: 2}, {Name: "Plains", Count: 7}, {Name: "Forest", Count: 7},
	},
	// U/G/w: Stuck in Summoner's Sanctum, Clarion Conqueror, the pinger, Omen
	{
		{Name: "Stuck in Summoner's Sanctum", Count: 3}, {Name: "Clarion Conqueror", Count: 2}, {Name: "Prodigal Sorcerer", Count: 4},
		{Name: "Canyon Crab", Count: 3}, {Name: "Omen of the Sea", Count: 3}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Elvish Mystic", Count: 2}, {Name: "Mind Stone", Count: 2},
		{Name: "Island", Count: 9}, {Name: "Forest", Count: 6}, {Name: "Plains", Count: 2},
	},
}

// kcantactAbolished: the deck cards with an activated ability Grand
// Abolisher's ValidCard$ (Artifact,Creature,Enchantment) covers.
var kcantactAbolished = map[string]bool{"Llanowar Elves": true, "Elvish Mystic": true, "Twinblade Slasher": true,
	"Bonesplitter": true, "Mind Stone": true, "Prodigal Sorcerer": true, "Canyon Crab": true, "Omen of the Sea": true}

type kcantactStats struct {
	games, divs, stalls int
	held                int // decisions where a live carrier held an activation offer out (kernel side)
	manaHeld            int // ... a mana source out of the planner (manaPCGen's noManaPC through actForbidden)
	abolHeld            int // ... Grand Abolisher's conditional lock, on its controller's turn
	abolFree            int // decisions on the Abolisher's opponent's own turn offering that player's covered permanent's ability
	sunWalker           int // decisions with The Immortal Sun and a planeswalker both on the battlefield
	activations         int
}

// kcantactPolicy plays lands, casts and activates when it can (so a lock
// is tested against a live activation), else random; between picks it
// reads the kernel's hold-out counters (dirac.DebugCantActCount).
func kcantactPolicy(seed uint64, st *kcantactStats) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	h0, m0, c0 := dirac.CantActHeld, dirac.CantActManaHeld, dirac.CantActCondHeld
	return PolicyFunc(func(c *PickCtx) Canon {
		if dirac.CantActHeld != h0 {
			st.held++
		}
		if dirac.CantActManaHeld != m0 {
			st.manaHeld++
		}
		if dirac.CantActCondHeld != c0 {
			st.abolHeld++
		}
		h0, m0, c0 = dirac.CantActHeld, dirac.CantActManaHeld, dirac.CantActCondHeld
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActPlayLand || k == ActCast || k == ActActivate {
				buf = append(buf, a)
			}
		}
		if c.G != nil {
			st.abolFree += kcantactAbolFree(c)
			st.sunWalker += kcantactSunWalker(c)
		}
		a := Canon(0)
		if len(buf) == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if a.Kind() == ActActivate {
			st.activations++
		}
		return a
	})
}

// kcantactAbolFree is 1 when, on the decider's own turn, a Grand Abolisher
// of the other player is on the battlefield and the decision offers an
// activation of the decider's permanent the Abolisher covers.
func kcantactAbolFree(c *PickCtx) int {
	gg := c.G.E.G
	p := state.PlayerID(c.D.Player)
	if gg.Active != p {
		return 0
	}
	abol := false
	for _, id := range gg.Zone(state.ZBattlefield, 1-p) {
		if ob := gg.Obj(id); ob != nil && ob.Face() != nil && ob.Face().Name == "Grand Abolisher" {
			abol = true
		}
	}
	if !abol {
		return 0
	}
	for _, a := range c.D.Options {
		if a.Kind() != ActActivate {
			continue
		}
		if o, ok := c.G.GorgeOption(a); ok {
			if ob := gg.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Controller == p && kcantactAbolished[ob.Face().Name] {
				return 1
			}
		}
	}
	return 0
}

// kcantactSunWalker is 1 when The Immortal Sun and a planeswalker are both
// on the battlefield, so the loyalty-only lock is live at the decision.
func kcantactSunWalker(c *PickCtx) int {
	gg := c.G.E.G
	sun, walker := false, false
	for p := state.PlayerID(0); p < 2; p++ {
		for _, id := range gg.Zone(state.ZBattlefield, p) {
			if ob := gg.Obj(id); ob != nil && ob.Face() != nil {
				sun = sun || ob.Face().Name == "The Immortal Sun"
				walker = walker || ob.Face().Name == "Ajani, Caller of the Pride"
			}
		}
	}
	if sun && walker {
		return 1
	}
	return 0
}

// runCantact plays seeds 1..seeds of the constructed matchup. mutate sets
// DebugCantActMana (the mutation must diverge).
func runCantact(t *testing.T, seeds int, mutate bool) *kcantactStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("cantact")
	if err != nil {
		t.Fatal(err)
	}
	oldM, oldC := dirac.DebugCantActMana, dirac.DebugCantActCount
	dirac.DebugCantActMana, dirac.DebugCantActCount = mutate, true
	defer func() { dirac.DebugCantActMana, dirac.DebugCantActCount = oldM, oldC }()
	st := &kcantactStats{}
	var decks [2][]*cards.Card
	for s, d := range kcantactDecks {
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
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kcantactDecks[0], kcantactDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kcantact-a", "kcantact-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, kcantactPolicy(seed, st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		st.games++
		if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			st.stalls++
			if !mutate {
				t.Errorf("seed %d: kernel stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			st.divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return st
}

func TestCantactLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runCantact(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, %d activations; held-out decisions %d (mana %d, Abolisher on its turn %d), Abolisher's opponent's own-turn activations %d, decisions beside The Immortal Sun and a planeswalker %d",
		st.games, st.divs, st.stalls, st.activations, st.held, st.manaHeld, st.abolHeld, st.abolFree, st.sunWalker)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	if st.held < 50 {
		t.Errorf("only %d decisions held an activated ability out", st.held)
	}
	if st.manaHeld < 30 {
		t.Errorf("only %d decisions held a mana source out", st.manaHeld)
	}
	if st.abolHeld < 10 {
		t.Errorf("only %d Grand Abolisher held-outs on its controller's turn", st.abolHeld)
	}
	if st.abolFree < 10 {
		t.Errorf("only %d own-turn activations beside an opposing Grand Abolisher", st.abolFree)
	}
	if st.sunWalker < 10 {
		t.Errorf("only %d decisions beside The Immortal Sun and a planeswalker", st.sunWalker)
	}
}

// TestCantactMutation keeps a locked mana source live: the games must
// diverge. A green mutation (no divergence) means the lockstep never pays
// with, or never counts, a locked mana creature or rock.
func TestCantactMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runCantact(t, 128, true)
	if st.divs == 0 {
		t.Fatalf("mutation kept locked mana sources live in %d games with no divergence", st.games)
	}
	t.Logf("mutation diverged in %d of %d games", st.divs, st.games)
}
