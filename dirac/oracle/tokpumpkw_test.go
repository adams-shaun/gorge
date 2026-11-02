package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The non-head Token PumpKeywords$ lockstep: a Token whose
// keyword rider ("it gains haste until end of turn", gorge token.go's
// LAbilities grant) is not the first instruction of its resolution chain.
// Artistic Process and Stingerquill Charm put it inside a chosen charm mode
// (the kernel program has the Charm op and the other modes' skipped bodies
// before it), Salt Road Skirmish puts it after a Destroy, and Krenko, Baron
// of Tin Street puts it after the trigger's paid-{R} IfPaid wrapper. The
// kernel's remembered set is per resolution, so kgen clears it just before
// the mint (OpClearRemembered) and the PumpAll that follows sees only the
// mints; the expected answer at every decision is gorge's.

var ktokpumpkwDecks = [2][]cardset.Entry{
	{
		{Name: "Artistic Process", Count: 2}, {Name: "Stingerquill Charm", Count: 3},
		{Name: "Salt Road Skirmish", Count: 3}, {Name: "Krenko, Baron of Tin Street", Count: 3},
		{Name: "Ornithopter", Count: 3}, {Name: "Bronze Sable", Count: 3},
		{Name: "Raging Goblin", Count: 3}, {Name: "Shock", Count: 3},
		{Name: "Lightning Bolt", Count: 2}, {Name: "Doom Blade", Count: 2},
		{Name: "Mountain", Count: 9}, {Name: "Swamp", Count: 4},
	},
	{
		{Name: "Artistic Process", Count: 2}, {Name: "Stingerquill Charm", Count: 2},
		{Name: "Salt Road Skirmish", Count: 2}, {Name: "Krenko, Baron of Tin Street", Count: 2},
		{Name: "Ornithopter", Count: 3}, {Name: "Bronze Sable", Count: 3},
		{Name: "Gray Ogre", Count: 4}, {Name: "Raging Goblin", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Doom Blade", Count: 2},
		{Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 4},
	},
}

// ktokpumpkwTokens are the token script keys the four carriers mint. Every
// one of them gets the carriers' PumpKeywords$ haste rider.
var ktokpumpkwTokens = map[string]bool{
	"ur_3_3_elemental_flying": true, // Artistic Process
	"cadet":                   true, // Stingerquill Charm
	"r_1_1_warrior":           true, // Salt Road Skirmish (two per cast)
	"r_1_1_goblin":            true, // Krenko, Baron of Tin Street
}

// ktokpumpkwCounts is what the games exercised.
type ktokpumpkwCounts struct {
	mints   int // carrier tokens minted (a TokenCreate event per token)
	attacks int // attacks by a token that entered the battlefield this turn
}

// ktokpumpkwProbe is a seeded random policy biased to attack and to play the
// carriers: at an attackers decision it picks among the attacking options,
// otherwise it casts a carrier or activates Krenko when it can. It counts
// the carrier mints from gorge's event log and the attacks actually chosen
// with a token that entered this turn.
type ktokpumpkwProbe struct {
	st   *ktokpumpkwCounts
	base Policy
	evAt int
	buf  []Canon
}

func ktokpumpkwCarrier(name string) bool {
	switch name {
	case "Artistic Process", "Stingerquill Charm", "Salt Road Skirmish", "Krenko, Baron of Tin Street":
		return true
	}
	return false
}

func (p *ktokpumpkwProbe) pick(c *PickCtx) Canon {
	if c.G != nil {
		evs := c.G.E.L.Events
		for p.evAt < len(evs) {
			e := &evs[p.evAt]
			p.evAt++
			if e.Kind == events.TokenCreate && ktokpumpkwTokens[e.Text] {
				p.st.mints++
			}
		}
	}
	p.buf = p.buf[:0]
	var carriers []Canon
	for _, a := range c.D.Options {
		switch a.Kind() {
		case ActAttack:
			p.buf = append(p.buf, a)
		case ActCast:
			if c.G != nil {
				if o, ok := c.G.GorgeOption(a); ok {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ktokpumpkwCarrier(ob.Face().Name) {
						carriers = append(carriers, a)
					}
				}
			}
		case ActActivate:
			if c.G != nil {
				if o, ok := c.G.GorgeOption(a); ok {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Krenko, Baron of Tin Street" {
						carriers = append(carriers, a)
					}
				}
			}
		}
	}
	pick := func(opts []Canon) Canon {
		d := *c.D
		d.Options = opts
		return p.base.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	}
	var a Canon
	switch {
	case len(p.buf) > 0:
		a = pick(p.buf)
	case len(carriers) > 0:
		a = pick(carriers)
	default:
		a = p.base.Pick(c)
	}
	if a.Kind() == ActAttack && c.G != nil {
		if o, ok := c.G.GorgeOption(a); ok {
			if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.IsToken && ob.EnteredThisTurn {
				p.st.attacks++
			}
		}
	}
	return a
}

// ktokpumpkwRun plays seeds 1..seeds of the matchup. report logs a
// divergence as a test error (the mutation run expects one and stays quiet).
func ktokpumpkwRun(t *testing.T, reg *cards.Registry, set cardset.Set, decks [2][]*cards.Card, tbl *kop.Table, seeds uint64, report bool) (games, divs, stalls int, st ktokpumpkwCounts) {
	t.Helper()
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ktokpumpkw-a", "ktokpumpkw-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &ktokpumpkwProbe{st: &st, base: RandomPolicy(seed)}
		res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Logf("seed %d: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			if report {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			continue
		}
	}
	return
}

func TestTokpumpkwLockstep(t *testing.T) {
	t.Parallel()
	reg, set, decks, tbl := p3Setup(t, "tokpumpkw", ktokpumpkwDecks)
	games, divs, stalls, st := ktokpumpkwRun(t, reg, set, decks, tbl, 256, true)
	t.Logf("%d games, %d divergences, %d stalls, %d carrier token mints, %d attacks by tokens made that turn",
		games, divs, stalls, st.mints, st.attacks)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if stalls != 0 {
		t.Errorf("%d kernel stalls", stalls)
	}
	if st.mints < 20 {
		t.Errorf("carrier token mints reached %d, want at least 20", st.mints)
	}
	if st.attacks < 10 {
		t.Errorf("attacks by a token made that turn reached %d, want at least 10", st.attacks)
	}
}

// ktokpumpkwNoHaste rewrites the built table's carrier mints to drop the
// PumpAll haste grant that follows each remembered mint (the "no haste"
// reading). The instruction becomes OpNop instead of being removed so every
// Charm/IfPaid skip offset the kernel reads stays valid. It returns the
// number of grants dropped.
func ktokpumpkwNoHaste(tbl *kop.Table) int {
	n := 0
	for i := range tbl.Cards {
		n += ktokpumpkwDropPump(tbl.Cards[i].Abilities)
	}
	for i := range tbl.Emblems {
		n += ktokpumpkwDropPump(tbl.Emblems[i].Abilities)
	}
	return n
}

func ktokpumpkwDropPump(abs []kop.Ability) int {
	n := 0
	for a := range abs {
		prog := abs[a].Prog
		for i := 0; i < len(prog); i++ {
			if prog[i].Op != kop.OpCreateToken && prog[i].Op != kop.OpCreateTokenEx {
				continue
			}
			if prog[i].B.N&int16(kop.TokRememberObj) == 0 {
				continue
			}
			for j := i + 1; j < len(prog); j++ {
				switch prog[j].Op {
				case kop.OpPumpAll:
					prog[j] = kop.Instr{Op: kop.OpNop}
					n++
					i = j
				case kop.OpCreateToken, kop.OpCreateTokenEx:
					i = j - 1
				default:
					continue
				}
				break
			}
		}
	}
	return n
}

// TestTokpumpkwMutation: without the PumpAll, a token minted this turn has
// no haste in the kernel while gorge lets it attack, so the lockstep must
// diverge.
func TestTokpumpkwMutation(t *testing.T) {
	t.Parallel()
	reg, set, decks, tbl := p3Setup(t, "tokpumpkw", ktokpumpkwDecks)
	if n := ktokpumpkwNoHaste(tbl); n == 0 {
		t.Fatal("no carrier token mint with a PumpAll in the built table")
	} else {
		t.Logf("dropped %d token PumpAll grants", n)
	}
	games, divs, _, _ := ktokpumpkwRun(t, reg, set, decks, tbl, 64, false)
	if divs == 0 {
		t.Fatalf("no haste: no divergence in %d seeds", games)
	}
	t.Logf("no haste diverged in %d of %d games", divs, games)
}
