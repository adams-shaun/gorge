package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The mana-fee restricted carrier lockstep: Castle Garenbrig's "{2}{G}{G}, {T}:
// Add six {G}" beside its {T}: Add {G} census ability (the restrictedmana
// fixture, restrictedmana_lockstep_test.go's class scan). The decks are
// built so the fee alternative is the only way a Craw Wurm is payable: four
// to six mana of fee-free green against the Wurm's {4}{G}{G} once the board
// is small, the fee paid from the mana the window has already accumulated
// (CR 605.3b) and the six {G} floating creature-spells-only. Both surfaces
// (gorge's autopay option surface and the full one) agree with the kernel,
// and the fee carrier's mana was spent.

// restrictedManaFeeDecks are the two decks; the mirrors keep both seats'
// casts on the fee unit.
func restrictedManaFeeDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Craw Wurm", Count: 12}, {Name: "Castle Garenbrig", Count: 12},
		{Name: "Forest", Count: 6}, {Name: "Llanowar Elves", Count: 6}, {Name: "Giant Growth", Count: 4},
	}
	return [2][]cardset.Entry{d, d}
}

// restrictedManaFeePod is the two-seat pod of restrictedManaFeeDecks.
func restrictedManaFeePod(t *testing.T) restrictedManaPod {
	t.Helper()
	reg := synthReg(t)
	set, err := cardset.RawByName("restrictedmana")
	if err != nil {
		t.Fatal(err)
	}
	all := restrictedManaFeeDecks()
	p := restrictedManaPod{reg: reg, set: set}
	for i, d := range all {
		c := 0
		for _, e := range d {
			c += e.Count
		}
		if c != 40 {
			t.Fatalf("deck %d: %d cards", i, c)
		}
		p.decks = append(p.decks, d)
	}
	return p
}

// play plays one seed of the pod on the named option surface.
func (p restrictedManaPod) playSurface(t *testing.T, seed uint64, maxTurns int32, surface string) (Result, restrictedManaCounts) {
	t.Helper()
	expand := func(d []cardset.Entry) []string {
		var out []string
		for _, e := range d {
			for j := 0; j < e.Count; j++ {
				out = append(out, e.Name)
			}
		}
		return out
	}
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{expand(p.decks[0]), expand(p.decks[1])},
		Seed: seed, PassElision: true, Surface: surface}
	m, err := r.Matchup(p.reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, maxTurns
	c := restrictedManaCounts{byName: map[string]int{}}
	rc.Finish = func(g *GorgeSide) { restrictedManaScan(g, &c) }
	tbl, _, err := p.set.MatchupN(p.decks)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(p.set, tbl, rc.Config)(m, seed)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	defer CloseSide(k)
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	return LockstepLed(m, seed, lead, k, castFirstPolicy(seed), rc), c
}

// TestRestrictedManaFeeLockstep plays two-seat pods of the fee decks on both
// option surfaces: no divergence and no stall, and a mana-fee restricted unit
// was spent (restrictedManaScan's rmFee class).
func TestRestrictedManaFeeLockstep(t *testing.T) {
	p := restrictedManaFeePod(t)
	for _, surface := range []string{"full", "autopay"} {
		var fee int
		for _, seed := range restrictedManaSeeds {
			res, c := p.playSurface(t, seed, 40, surface)
			if res.Div != nil {
				restrictedManaFail(t, seed, res)
			}
			if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Fatalf("seed %d (%s): kernel stopped: %s", seed, surface, res.Stall)
			}
			fee += c.byClass[rmFee]
			t.Logf("%s seed %d: %d decisions; spent fee %v by %v", surface, seed, res.Decisions, c.byClass, c.byName)
		}
		// gorge's autopay planner never funds a restricted fee ability, so
		// that surface never spends one; the full surface's cast window does
		if want := surface == "full"; (fee != 0) != want {
			t.Fatalf("%s: mana-fee restricted mana spent %d times over the seeds, want some: %v", surface, fee, want)
		}
	}
}

// TestRestrictedManaFeeThresholdLockstep plays a Forest-heavy fee deck (Craw
// Wurm, Castle Garenbrig, Forest only) over sixty seeds of the full surface:
// the boards of three and four Forests beside a Castle and a Wurm in hand come
// up in nearly every game, so the kernel's threshold for the fee cast (the
// Castle's fee {2}{G}{G} paid from the Forests' mana: three Forests do not
// offer the Wurm, four do) is held to gorge's at every decision.
func TestRestrictedManaFeeThresholdLockstep(t *testing.T) {
	p := restrictedManaFeePod(t)
	d := []cardset.Entry{{Name: "Craw Wurm", Count: 12}, {Name: "Castle Garenbrig", Count: 8}, {Name: "Forest", Count: 20}}
	p.decks = [][]cardset.Entry{d, d}
	fee := 0
	for seed := uint64(1); seed <= 60; seed++ {
		res, c := p.playSurface(t, seed, 40, "full")
		if res.Div != nil {
			restrictedManaFail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		fee += c.byClass[rmFee]
	}
	if fee == 0 {
		t.Fatal("no mana-fee restricted mana was spent over the seeds")
	}
}
