package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
)

// The unlesstapward lockstep: an UnlessCost$
// tapXType<N/Spec> (Command Bridge's and Public Thoroughfare's ETB, Rent Is
// Due's tap-two, Reservoir Kraken's opponent-paid tap: gorge's unless pay /
// decline modes, then the "tapcost" part, unasked with exactly N
// candidates) and Ward—{N}, pay N life (Ovika, Enigma Goliath; Gisa, the
// Hellraiser: effWard's pay / decline, the mana window, then the life),
// targeted by burn, bounce and Swords. The expected answer at every
// decision is gorge's.
var kunlesstapwardDecks = [2][]cardset.Entry{
	{
		{Name: "Ovika, Enigma Goliath", Count: 3}, {Name: "Gisa, the Hellraiser", Count: 3},
		{Name: "Rent Is Due", Count: 2}, {Name: "Reservoir Kraken", Count: 2},
		{Name: "Command Bridge", Count: 4}, {Name: "Public Thoroughfare", Count: 4},
		{Name: "Ornithopter", Count: 2}, {Name: "Grizzly Bears", Count: 1}, {Name: "Shock", Count: 2},
		// granted-ward carriers: a ward {N} beside the printed mana-and-life
		// ward, each its own trigger (CR 113.2c)
		{Name: "Sheltered by Ghosts", Count: 2}, {Name: "Hardlight Containment", Count: 1}, {Name: "Combat Research", Count: 1},
		{Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 2}, {Name: "Island", Count: 4}, {Name: "Plains", Count: 3},
	},
	{
		// mostly lands beside cheap targeting: the caster often has the
		// ward's mana open
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Unsummon", Count: 4},
		{Name: "Swords to Plowshares", Count: 2}, {Name: "Reservoir Kraken", Count: 2},
		{Name: "Command Bridge", Count: 3}, {Name: "Public Thoroughfare", Count: 3},
		{Name: "Mountain", Count: 8}, {Name: "Island", Count: 6}, {Name: "Plains", Count: 4},
	},
}

// kunlesstapwardUnless names the unless-tapXType carriers by counter arm:
// the lands' ETB sacrifice, Rent Is Due's end-step tap-two (UnlessSwitched,
// WhenNotPaid subs) and Reservoir Kraken's opponent-paid combat tap.
var kunlesstapwardUnless = map[string]string{"Command Bridge": "land", "Public Thoroughfare": "land",
	"Rent Is Due": "rent", "Reservoir Kraken": "kraken"}

// kutwState is the policy's counters and the pending ward payment whose
// life loss the next decision checks.
type kutwState struct {
	counts   map[string]int
	flip     int
	wardP    int // the ward payer awaiting the check (-1 = none)
	wardLife int32
}

// kunlesstapwardPolicy answers the unless-tapXType asks and the
// mana-and-life ward asks pay, pay, decline in turn (so both arms are
// counted); the rest is cast-first (seed%8 == 0: random).
func kunlesstapwardPolicy(seed uint64, st *kutwState) Policy {
	base := castFirstPolicy(seed)
	if seed%8 == 0 {
		base = RandomPolicy(seed)
	}
	rnd := RandomPolicy(seed ^ 0x5eed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return base.Pick(c)
		}
		g := c.G.E.G
		if st.wardP >= 0 {
			if g.Players[st.wardP].Life < st.wardLife {
				st.counts["ward/paid"]++
			}
			st.wardP = -1
		}
		d := c.G.Pending()
		if d == nil || d.ResumeKind != "unless_pay" {
			return base.Pick(c)
		}
		name := ""
		if ob := g.Obj(d.Source); ob != nil && ob.Face() != nil {
			name = ob.Face().Name
		}
		ward := strings.HasSuffix(d.Prompt, " for ward?") && strings.Contains(strings.ToLower(d.Prompt), "life")
		arm := kunlesstapwardUnless[name]
		if !ward && arm == "" {
			return base.Pick(c)
		}
		pay := st.flip%3 != 2
		st.flip++
		want := decision.ModeUnlessDecline
		if pay {
			want = decision.ModeUnlessPay
		}
		idx := -1
		for _, o := range d.Options {
			if o.Mode == want {
				idx = o.Index
			}
		}
		for _, a := range c.D.Options {
			if idx < 0 || a.Kind() != ActMode || int(a.Mode()) != idx {
				continue
			}
			switch {
			case ward && pay:
				st.counts["ward/pay"]++
				st.wardP, st.wardLife = int(d.Player), g.Players[d.Player].Life
			case ward:
				st.counts["ward/declined"]++
			case pay:
				st.counts[arm+"/paid"]++
			default:
				st.counts[arm+"/declined"]++
			}
			return a
		}
		return rnd.Pick(c)
	})
}

func kunlesstapwardRun(t *testing.T, seeds uint64, stopAtDiv bool, st *kutwState) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("unlesstapward")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range kunlesstapwardDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kunlesstapwardDecks[0], kunlesstapwardDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	div := 0
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"kunlesstapward-a", "kunlesstapward-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%4 == 0}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		st.wardP = -1
		res := Lockstep(m, seed, k, kunlesstapwardPolicy(seed, st), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			st.counts["stall"]++
		}
		if d := res.Div; d != nil {
			div++
			if stopAtDiv {
				t.Logf("seed %d diverged at decision %d: %s %s", seed, d.At, d.What, d.Field)
				return div
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return div
}

func TestUnlesstapwardLockstep(t *testing.T) {
	t.Parallel()
	st := &kutwState{counts: map[string]int{}}
	kunlesstapwardRun(t, 300, false, st)
	c := st.counts
	c["ward/triggers"] = c["ward/pay"] + c["ward/declined"]
	c["ward/countered"] = c["ward/triggers"] - c["ward/paid"]
	t.Logf("counts %v", c)
	for arm, min := range map[string]int{"land/paid": 10, "land/declined": 5, "ward/triggers": 10, "ward/paid": 3, "ward/countered": 3,
		"rent/paid": 3, "rent/declined": 3, "kraken/paid": 3, "kraken/declined": 3} {
		if c[arm] < min {
			t.Errorf("%s: %d, want at least %d", arm, c[arm], min)
		}
	}
	if c["stall"] != 0 {
		t.Errorf("%d stalled games", c["stall"])
	}
}

// TestUnlesstapwardMutation proves the pool pays the mana-and-life ward:
// with the kernel's life loss skipped, some seed diverges.
func TestUnlesstapwardMutation(t *testing.T) {
	dirac.DebugWardMLNoLife = true
	defer func() { dirac.DebugWardMLNoLife = false }()
	st := &kutwState{counts: map[string]int{}}
	if kunlesstapwardRun(t, 128, true, st) == 0 {
		t.Fatal("no seed diverged with the mana-and-life ward's life loss skipped")
	}
}
