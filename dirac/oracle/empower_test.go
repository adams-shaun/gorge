package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Empower lockstep (CR 701.71a): constructed matchups over
// the FRA cards whose only refused ability was api:Empower. The counters and
// the minted token are gorge's; Doubling Season makes the two-mint pick.

// empowerCarriers are the pool's Empower cards (Fatehold Charm, Solve for
// Disappointment and Tam's Resistance stay refused for reasons unrelated to
// Empower, so they are not in the pool).
var empowerCarriers = []string{
	"Academic Ascent", "Arcane Amphisbaena", "Avatar of Burgeoning Echoes",
	"Campus Crier", "Hexhaven Battalion", "Inspired Tethermage",
	"Keeper of the Quiet Hour", "Mindseeker Oculus", "No Admittance",
	"Plan for All Outcomes", "Protege's Awakening", "Repurposed Enforcer",
	"Rewrite Regrets", "Violent Echoes",
	"Vraska's Final Mercy", "Way of the Deathbringer", "Way of the Healer",
	"Way of the Mentor", "Way of the Necromancer", "Way of the Warlord",
	"Way of the Wildspeaker",
}

var empowerDecks = [][2][]cardset.Entry{{
	{
		{Name: "Academic Ascent", Count: 3}, {Name: "Arcane Amphisbaena", Count: 3},
		{Name: "Avatar of Burgeoning Echoes", Count: 3},
		{Name: "Campus Crier", Count: 3}, {Name: "Plains", Count: 4},
		{Name: "Island", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
		{Name: "Forest", Count: 6},
	},
	{
		{Name: "Hexhaven Battalion", Count: 3}, {Name: "Keeper of the Quiet Hour", Count: 3},
		{Name: "Mindseeker Oculus", Count: 3}, {Name: "Plains", Count: 7},
		{Name: "Island", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
		{Name: "Forest", Count: 6},
	},
}, {
	{
		{Name: "No Admittance", Count: 3}, {Name: "Protege's Awakening", Count: 3},
		{Name: "Rewrite Regrets", Count: 3}, {Name: "Campus Crier", Count: 3},
		{Name: "Plains", Count: 4}, {Name: "Island", Count: 6}, {Name: "Swamp", Count: 6},
		{Name: "Mountain", Count: 6}, {Name: "Forest", Count: 6},
	},
	{
		{Name: "Campus Crier", Count: 3}, {Name: "Violent Echoes", Count: 3},
		{Name: "Vraska's Final Mercy", Count: 3}, {Name: "Plains", Count: 7},
		{Name: "Island", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
		{Name: "Forest", Count: 6},
	},
}, {
	{
		{Name: "Way of the Deathbringer", Count: 3}, {Name: "Academic Ascent", Count: 3},
		{Name: "Way of the Mentor", Count: 3}, {Name: "Plains", Count: 7},
		{Name: "Island", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
		{Name: "Forest", Count: 6},
	},
	{
		{Name: "Way of the Necromancer", Count: 3}, {Name: "Campus Crier", Count: 3},
		{Name: "Way of the Wildspeaker", Count: 3}, {Name: "Plains", Count: 7},
		{Name: "Island", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
		{Name: "Forest", Count: 6},
	},
}, {
	{
		{Name: "Repurposed Enforcer", Count: 3}, {Name: "Plan for All Outcomes", Count: 3},
		{Name: "Inspired Tethermage", Count: 3}, {Name: "Plains", Count: 7},
		{Name: "Island", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
		{Name: "Forest", Count: 6},
	},
	{
		{Name: "Campus Crier", Count: 3}, {Name: "Violent Echoes", Count: 3},
		{Name: "Doubling Season", Count: 3}, {Name: "Plains", Count: 7},
		{Name: "Island", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
		{Name: "Forest", Count: 6},
	},
}, {
	{
		{Name: "Academic Ascent", Count: 3}, {Name: "Mindseeker Oculus", Count: 3},
		{Name: "Way of the Mentor", Count: 3}, {Name: "Plains", Count: 7},
		{Name: "Island", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
		{Name: "Forest", Count: 6},
	},
	{
		{Name: "Way of the Wildspeaker", Count: 3}, {Name: "Keeper of the Quiet Hour", Count: 3},
		{Name: "Doubling Season", Count: 3}, {Name: "Plains", Count: 7},
		{Name: "Island", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
		{Name: "Forest", Count: 6},
	},
}, {
	{
		{Name: "No Admittance", Count: 3}, {Name: "Campus Crier", Count: 3},
		{Name: "Vraska's Final Mercy", Count: 3}, {Name: "Plains", Count: 7},
		{Name: "Island", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
		{Name: "Forest", Count: 6},
	},
	{
		{Name: "Protege's Awakening", Count: 3}, {Name: "Rewrite Regrets", Count: 3},
		{Name: "Doubling Season", Count: 3}, {Name: "Campus Crier", Count: 3},
		{Name: "Plains", Count: 4}, {Name: "Island", Count: 6}, {Name: "Swamp", Count: 6},
		{Name: "Mountain", Count: 6}, {Name: "Forest", Count: 6},
	},
}, {
	{
		{Name: "Repurposed Enforcer", Count: 3}, {Name: "Campus Crier", Count: 3},
		{Name: "Doubling Season", Count: 3}, {Name: "Plains", Count: 7},
		{Name: "Island", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
		{Name: "Forest", Count: 6},
	},
	{
		{Name: "Inspired Tethermage", Count: 3}, {Name: "Plan for All Outcomes", Count: 3},
		{Name: "Arcane Amphisbaena", Count: 3}, {Name: "Plains", Count: 7},
		{Name: "Island", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
		{Name: "Forest", Count: 6},
	},
}, {
	{
		{Name: "Hexhaven Battalion", Count: 3}, {Name: "Way of the Warlord", Count: 3},
		{Name: "Campus Crier", Count: 3}, {Name: "Plains", Count: 7},
		{Name: "Island", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
		{Name: "Forest", Count: 6},
	},
	{
		{Name: "Academic Ascent", Count: 3}, {Name: "Way of the Necromancer", Count: 3},
		{Name: "Way of the Healer", Count: 3}, {Name: "Plains", Count: 7},
		{Name: "Island", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
		{Name: "Forest", Count: 6},
	},
}}

// empPolicy prefers casts, activations and land plays (so the games reach
// every arm), else random.
func empPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActActivate, ActPlayLand:
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

// empTally counts the arms from gorge's event log: the u_empower_jace mints, the
// loyalty placements on a token minted on an earlier turn (the existing
// token), the counter_pick asks, and each carrier put on the stack.
type empTally struct {
	evAt     int
	mints    int
	existing int
	picks    int
	pickSeq  uint64
	seen     map[string]bool
	carrier  map[string]bool
	tokTurn  map[state.ObjID]int32
}

func newEmpTally() *empTally {
	t := &empTally{seen: map[string]bool{}, carrier: map[string]bool{}, tokTurn: map[state.ObjID]int32{}}
	for _, n := range empowerCarriers {
		t.carrier[n] = true
	}
	return t
}

func (t *empTally) reset() {
	t.evAt = 0
	t.pickSeq = 0
	t.tokTurn = map[state.ObjID]int32{}
}

func (t *empTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	// A TokenCreate event's Obj is zero (the fold assigns the id), so the
	// mint's turn is stamped from the battlefield: every token the board
	// holds for the first time at this decision was minted this turn.
	for p := uint8(0); p < 2; p++ {
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
			if _, ok := t.tokTurn[id]; ok {
				continue
			}
			if o := g.Obj(id); o != nil && o.IsToken {
				t.tokTurn[id] = g.Turn
			}
		}
	}
	evs := ref.E.L.Events
	if len(evs) < t.evAt {
		t.evAt = 0
	}
	for j := t.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.TokenCreate:
			if e.Text == "u_empower_jace" {
				t.mints++
			}
		case events.CounterChange:
			if e.Counter == "LOYALTY" && e.Amount > 0 {
				if turn, ok := t.tokTurn[e.Obj]; ok && turn < g.Turn {
					t.existing++
				}
			}
		case events.PutOnStack:
			if t.carrier[e.Text] {
				t.seen[e.Text] = true
			}
		}
	}
	t.evAt = len(evs)
	if fr := ref.fr; fr != nil && fr.d != nil && fr.d.ResumeKind == "counter_pick" && fr.seq != t.pickSeq {
		t.pickSeq = fr.seq
		t.picks++
	}
	return nil
}

func TestEmpowerLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("empower")
	if err != nil {
		t.Fatal(err)
	}
	tally := *newEmpTally()
	games := 0
	for mi, md := range empowerDecks {
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
		for seed := uint64(1); seed <= 128; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"empower-a", "empower-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			tally.reset()
			res := Lockstep(m, seed, k, empPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 20, Check: tally.check})
			CloseSide(k)
			games++
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Fatalf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nlast actions %v\nref %v\nker %v",
					mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, actionStrings(res.Actions[lo:d.At]), d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games, %d mints, %d existing-token placements, %d picks", games, tally.mints, tally.existing, tally.picks)
	if tally.mints < 50 {
		t.Errorf("only %d mints, want >= 50", tally.mints)
	}
	if tally.existing < 50 {
		t.Errorf("only %d existing-token placements, want >= 50", tally.existing)
	}
	if tally.picks < 5 {
		t.Errorf("only %d counter_pick asks, want >= 5", tally.picks)
	}
	for _, n := range empowerCarriers {
		if !tally.seen[n] {
			t.Errorf("carrier %s never put on the stack", n)
		}
	}
}

// TestEmpowerMutation: with the debug switch on, the kernel always mints
// instead of reusing an existing Jace token, and the lockstep diverges.
func TestEmpowerMutation(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("empower")
	if err != nil {
		t.Fatal(err)
	}
	md := empowerDecks[0]
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
	}
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugEmpowerAlwaysMint
	dirac.DebugEmpowerAlwaysMint = true
	defer func() { dirac.DebugEmpowerAlwaysMint = old }()
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"empower-a", "empower-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, empPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 20})
		CloseSide(k)
		if res.Div != nil {
			return
		}
	}
	t.Fatal("debug switch did not diverge in 256 seeds")
}

// Two grantors give one planeswalker two different abilities, and each is its own
// option, told apart by its grantor.
func TestGrantedAbilitiesToldApart(t *testing.T) {
	if isolate(t) {
		return
	}
	deck := []cardset.Entry{
		{Name: "Way of the Warlord", Count: 10}, {Name: "Way of the Healer", Count: 10},
		{Name: "Mountain", Count: 10}, {Name: "Plains", Count: 10},
	}
	lockstepRun(t, "empower", [2][]cardset.Entry{deck, deck}, 60, true, nil)
}
