package oracle

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 500.7's extra-turn rotation across seats. Gorge keeps ONE queue of
// pending grants (e.G.ExtraTurnQueue, engine/rules/turn.go): a grant is
// created during the turn that resolves its spell -- on an opponent's turn
// for a flash extra-turn card -- and is spent at that turn's cleanup, when
// the granted seat takes the extra turn immediately. The ordinary rotation
// resumes after the LAST NORMAL turn's holder once every pending grant is
// spent. The kernel spent a per-seat counter only at that seat's own
// cleanup, so a Nexus of Fate cast on an opponent's turn pended until the
// caster's next turn: at three and four seats the granted turn started
// several seats late and the turn order diverged.

// kextraNexusDeck is a flash extra-turn deck: Nexus of Fate shuffles itself
// back into its owner's library, so the same copies are cast across the
// game, on the owner's turn and on every other seat's.
var kextraNexusDeck = []cardset.Entry{
	{Name: "Nexus of Fate", Count: 10},
	{Name: "Island", Count: 30},
}

// kextraStats is what the policy counts across the pod's games.
type kextraStats struct {
	casts      int   // Nexus of Fate casts
	crossTurn  int   // of those, casts made during another seat's turn
	turnNumber int32 // gorge turn number at the end of the last game
}

type kextraProbe struct {
	st   *kextraStats
	base Policy
	r    *rand.Rand
}

// pick casts Nexus of Fate only on another seat's turn (a cast on the
// caster's own turn chains the extra turns through one seat and starves the
// pod of turns and mana), and never otherwise: the rest of the options it
// picks from exclude the card, so a turn passes it on untapped.
func (p *kextraProbe) pick(c *PickCtx) Canon {
	var cross, rest []Canon
	active := state.PlayerID(0)
	if c.G != nil {
		active = c.G.E.G.Active
	}
	for _, a := range c.D.Options {
		if k := a.Kind(); k == ActCast || k == ActCastFace {
			if refCardName(c.G, a.Src()) == "Nexus of Fate" {
				if state.PlayerID(c.D.Player) != active {
					cross = append(cross, a)
				}
				continue
			}
		}
		rest = append(rest, a)
	}
	if len(cross) > 0 {
		p.st.casts++
		p.st.crossTurn++
		return cross[p.r.IntN(len(cross))]
	}
	if len(rest) == 0 {
		for _, a := range c.D.Options {
			if a.Kind() == ActPass {
				return a
			}
		}
	}
	return rest[p.r.IntN(len(rest))]
}

// runExtraTurnPods plays seats seats of the Nexus deck over seeds seeds,
// gorge leading, and returns the games, the divergences and the stalls.
func runExtraTurnPods(t *testing.T, seats, seeds int, report bool) (games, divs, stalls int, st kextraStats) {
	t.Helper()
	c, _ := testCorpus(t)
	set, err := cardset.RawByName("extraturn")
	if err != nil {
		t.Fatal(err)
	}
	var decks []*cards.Card
	for _, e := range kextraNexusDeck {
		cc, ok := c.Reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		for i := 0; i < e.Count; i++ {
			decks = append(decks, cc)
		}
	}
	md := make([][]cardset.Entry, seats)
	names := make([]string, seats)
	gd := make([][]*cards.Card, seats)
	for s := 0; s < seats; s++ {
		md[s], names[s], gd[s] = kextraNexusDeck, fmt.Sprint("kextra-", s), decks
	}
	tbl, _, err := set.MatchupN(md)
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchupN(c.Reg, names, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &kextraProbe{st: &st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x6578747261))}
		res := LockstepLed(m, seed, g, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		games++
		st.turnNumber = g.E.G.Turn
		if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
			if report {
				t.Logf("seed %d: kernel stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if report {
				lo := max(d.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, stalls, st
}

// TestExtraTurnPodNexusLockstep is the four-seat pod the class names: Nexus
// of Fate cast across the table, the granted turn taken at the granting
// turn's cleanup and the rotation resuming after the granting seat's normal
// successor. Every game must agree decision by decision.
func TestExtraTurnPodNexusLockstep(t *testing.T) {
	t.Parallel()
	games, divs, stalls, st := runExtraTurnPods(t, 4, 6, true)
	if st.casts == 0 || st.crossTurn == 0 {
		t.Fatalf("the pod never exercised the class: %d Nexus casts, %d on another seat's turn", st.casts, st.crossTurn)
	}
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d/%d games diverged, %d stalls", divs, games, stalls)
	}
}

// kextraFortuneDeck is the rider pod's deck: each caster's granted turn
// begins at the granting turn's cleanup and its end step fires the loss, so
// the pod plays down to one seat standing.
var kextraFortuneDeck = []cardset.Entry{
	{Name: "Final Fortune", Count: 4},
	{Name: "Mountain", Count: 36},
}

// kextraFFProbe prefers a Final Fortune cast; the rest it picks at random.
type kextraFFProbe struct {
	base Policy
	r    *rand.Rand
}

func (p *kextraFFProbe) pick(c *PickCtx) Canon {
	for _, a := range c.D.Options {
		if (a.Kind() == ActCast || a.Kind() == ActCastFace) && refCardName(c.G, a.Src()) == "Final Fortune" {
			return a
		}
	}
	return p.base.Pick(c)
}

// runFortunePods plays seats seats of the Final Fortune deck over seeds
// seeds and returns the games that reached a winner with the kernel, the
// divergences and the stalls.
func runFortunePods(t *testing.T, seats, seeds int, report bool) (games, overs, divs, stalls int) {
	t.Helper()
	c, _ := testCorpus(t)
	set, err := cardset.RawByName("extraturn")
	if err != nil {
		t.Fatal(err)
	}
	var decks []*cards.Card
	for _, e := range kextraFortuneDeck {
		cc, ok := c.Reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		for i := 0; i < e.Count; i++ {
			decks = append(decks, cc)
		}
	}
	md := make([][]cardset.Entry, seats)
	names := make([]string, seats)
	gd := make([][]*cards.Card, seats)
	for s := 0; s < seats; s++ {
		md[s], names[s], gd[s] = kextraFortuneDeck, fmt.Sprint("kfortune-", s), decks
	}
	tbl, _, err := set.MatchupN(md)
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchupN(c.Reg, names, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &kextraFFProbe{base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x666f727475))}
		res := LockstepLed(m, seed, g, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		games++
		if res.Over && res.Winner >= 0 {
			overs++
		}
		if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
			if report {
				t.Logf("seed %d: kernel stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if report {
				lo := max(d.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, overs, divs, stalls
}

// TestExtraTurnPodFinalFortuneLockstep is the rider over the four-seat pod:
// each caster's granted turn begins at the granting turn's cleanup and its
// end step fires the loss, until one seat stands.
func TestExtraTurnPodFinalFortuneLockstep(t *testing.T) {
	t.Parallel()
	games, overs, divs, stalls := runFortunePods(t, 4, 4, true)
	if overs == 0 {
		t.Fatalf("no game reached a winner: the riders never fired")
	}
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d/%d games diverged, %d stalls", divs, games, stalls)
	}
}

// TestExtraTurnPodFinalFortuneMutation: with DebugExRideAtGrant the kernel's
// rider arms at the grant, so its caster loses at the granting turn's end
// step and the granted turn never begins -- a turn and a whole extra turn
// ahead of gorge. Not parallel: it flips the package-global
// DebugExRideAtGrant, which a parallel lockstep run would read.
func TestExtraTurnPodFinalFortuneMutation(t *testing.T) {
	old := dirac.DebugExRideAtGrant
	dirac.DebugExRideAtGrant = true
	defer func() { dirac.DebugExRideAtGrant = old }()
	games, overs, divs, stalls := runFortunePods(t, 4, 1, false)
	if overs != 0 {
		t.Fatalf("%d/%d games reached a winner under the mutation", overs, games)
	}
	if divs == 0 {
		t.Fatal("the mutation never diverged from gorge")
	}
	_ = stalls
}
