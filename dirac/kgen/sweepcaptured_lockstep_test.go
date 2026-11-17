package kgen

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The captured-source sweep in lockstep. The form test
// (lower_sweepcaptured_test.go) proves the lowering; no corpus card reaches
// the shape (DamageSource$ Remembered on a DamageAll names Arcbond, whose
// DamageDoneOnce emblem hook is a sibling ticket), so this test plays the
// synthetic probe card in a real lockstep over a table built in process: the
// pool of corpus fillers plus the probe goes through the ordinary kgen
// tableOf path, and cardset.NewDataSet + cardset.Raw turn it into the set the
// oracle's SetSide drives. gorge plays the same probe script parsed beside
// the same corpus fillers, so the sweep — the capture as source, every other
// creature, then each player, one batch — is compared with gorge's
// effDamageAll for real, over seeds of ordinary play.
//
// The gone-source arm is not reached by these games: the shape's CantBlockBy
// static makes the capture unblockable for its effect's turn, the trigger is
// combat-only, and its DamageDealtOnce resolves without a response window a
// Lightning Bolt could use (80 diag seeds, none), so the capture is alive at
// every natural sweep. It stays covered by the kernel scenario test
// TestSweepFromCapturedBatch, and the follow-up ticket carries the
// gone-source question (gorge reads a gone source's keywords through LKI,
// engine/effects/damage.go newDamageRider; the kernel deals from the
// resolving effect).
//
// The table is in-memory and untracked; nothing lands under the corpus.

// sweepCapGameProbe is the lockstep's probe script: the form test's shape
// with a plain three-mana cost (the deck's basics pay it). Both sides play
// this exact script.
var sweepCapGameProbe = strings.Replace(sweepCapProbe, "ManaCost:2 U", "ManaCost:3", 1)

// sweepCapDecks are the two seats' 40-card decks: the probe, creatures and
// basics of every colour they cost. A 2/2's capture keeps the sweep's amount
// small, a 3/3's big.
func sweepCapDecks() [2][]cardset.Entry {
	d0 := []cardset.Entry{
		{Name: "Sweep Cap Probe", Count: 4},
		{Name: "Grizzly Bears", Count: 12},
		{Name: "Mountain", Count: 8},
		{Name: "Plains", Count: 6},
		{Name: "Forest", Count: 10},
	}
	d1 := []cardset.Entry{
		{Name: "Sweep Cap Probe", Count: 4},
		{Name: "Hill Giant", Count: 12},
		{Name: "Mountain", Count: 14},
		{Name: "Plains", Count: 10},
	}
	return [2][]cardset.Entry{slices.Clone(d0), slices.Clone(d1)}
}

// sweepCapTable builds the in-memory cardset for the fixture: every filler's
// ordinary lowering beside the probe's.
func sweepCapTable(t *testing.T, cardsDir string, probe *cards.Card) cardset.Set {
	t.Helper()
	p, err := loadCardNames([]string{"Grizzly Bears", "Hill Giant", "Mountain", "Plains", "Forest"}, cardsDir)
	if err != nil {
		t.Fatalf("filler pool: %v", err)
	}
	p.byName["Sweep Cap Probe"] = probe
	p.nDecks["Sweep Cap Probe"] = 1
	if !slices.Contains(p.names, "Sweep Cap Probe") {
		p.names = append(p.names, "Sweep Cap Probe")
		slices.Sort(p.names)
	}
	pd, _, _, err := tableOf(p, "sweepcap-fixture", nil)
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	return cardset.Raw(cardset.NewDataSet(pd))
}

// sweepCapRun plays the fixture's lockstep over seeds 1..seeds; report fails
// on each divergence (else the first stops the run). mutate, when set, runs
// before the games (a kernel mutation: its reset is the caller's).
func sweepCapRun(t *testing.T, seeds uint64, report bool, mutate func()) int {
	t.Helper()
	cardsDir, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("no corpus: %v", err)
	}
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	probe, diags := cards.ParseBytes("sweepcap_probe.txt", []byte(sweepCapGameProbe))
	if probe == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := probe.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	set := sweepCapTable(t, cardsDir, probe)
	decks := sweepCapDecks()
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s := range gd {
		for i := 0; i < 4; i++ {
			gd[s] = append(gd[s], probe)
		}
		for _, e := range decks[s] {
			if e.Name == "Sweep Cap Probe" {
				continue
			}
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
	if mutate != nil {
		mutate()
	}
	div := 0
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := oracle.NewMatchup(reg, [2]string{"sweepcap-a", "sweepcap-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := oracle.Config{}
		if seed%4 == 0 {
			cfg = oracle.Config{GorgeAutopaySurface: true}
		}
		k, err := oracle.SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := sweepCapPolicy(seed)
		if seed%8 == 0 {
			pol = oracle.RandomPolicy(seed)
		}
		res := oracle.Lockstep(m, seed, k, pol, oracle.RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		oracle.CloseSide(k)
		if d := res.Div; d != nil {
			div++
			if !report {
				t.Logf("seed %d diverges (decision %d: %s)", seed, d.At, d.What)
				return div
			}
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
		}
	}
	return div
}

// sweepCapPolicy casts, activates and plays lands whenever it can; in combat
// it attacks with everything and blocks with everything (a blocked attack
// still deals its damage); else random.
func sweepCapPolicy(seed uint64) oracle.Policy {
	rnd := oracle.RandomPolicy(seed)
	return oracle.PolicyFunc(func(c *oracle.PickCtx) oracle.Canon {
		buf := make([]oracle.Canon, 0, 8)
		for _, a := range c.D.Options {
			switch c.D.Kind {
			case oracle.DAttackers:
				if a.Kind() == oracle.ActAttack || a.Kind() == oracle.ActDone || a.Kind() == oracle.ActPass {
					buf = append(buf, a)
				}
			case oracle.DBlockers:
				if a.Kind() == oracle.ActBlock || a.Kind() == oracle.ActDone || a.Kind() == oracle.ActPass {
					buf = append(buf, a)
				}
			default:
				switch a.Kind() {
				case oracle.ActCast, oracle.ActCastFace, oracle.ActActivate, oracle.ActPlayLand:
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 || len(buf) == len(c.D.Options) {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&oracle.PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// TestSweepFromCapturedLockstep: the fixture's lockstep is clean over the
// seeds, and the pool reached the sweep — the kernel's test-only counter
// asserts capture sweeps ran, with player hits in the batch (the mutation
// tests pin each arm's observability separately).
func TestSweepFromCapturedLockstep(t *testing.T) {
	t.Parallel()
	st := &dirac.DebugSweepCounters{}
	dirac.DebugSweepStats = st
	defer func() { dirac.DebugSweepStats = nil }()
	div := sweepCapRun(t, 40, true, nil)
	if st.Runs == 0 || st.PlHits == 0 {
		t.Fatalf("the pool never reached the sweep (runs %d, player hits %d): the lockstep proves nothing", st.Runs, st.PlHits)
	}
	if div != 0 {
		t.Fatalf("%d seeds diverged", div)
	}
	t.Logf("capture sweeps %d, player hits %d over 40 seeds", st.Runs, st.PlHits)
}

// TestSweepFromCapturedLockstepMutationPl: with the sweep's player arm
// dropped some seed diverges — the proof that the lockstep observes the
// player arm (a player's life then differs). Test-only globals: never
// parallel with the other mutation.
func TestSweepFromCapturedLockstepMutationPl(t *testing.T) {
	if n := sweepCapRun(t, 40, false, func() { dirac.DebugSweepNoPlHits = true }); n > 0 {
		dirac.DebugSweepNoPlHits = false
		return
	}
	dirac.DebugSweepNoPlHits = false
	t.Fatal("no seed diverged with the sweep's player arm dropped: the pool does not reach it")
}

// TestSweepFromCapturedLockstepMutationSrc: with the sweep's source forced to
// the resolving effect some seed diverges — the proof that the lockstep
// observes the sweep's source identity (the capture's own slot and its
// exclusion then differ).
func TestSweepFromCapturedLockstepMutationSrc(t *testing.T) {
	if n := sweepCapRun(t, 40, false, func() { dirac.DebugSweepFromSelf = true }); n > 0 {
		dirac.DebugSweepFromSelf = false
		return
	}
	dirac.DebugSweepFromSelf = false
	t.Fatal("no seed diverged with the sweep's source forced to the resolving effect: the pool does not reach a live capture")
}
