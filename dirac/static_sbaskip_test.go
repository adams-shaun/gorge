package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// sbaPlay plays steps transitions of the bench pairings of pool on the pool's
// table with a seeded random policy and returns a trace hash of the play: the
// decision kinds, the seats and (every 16th step and at each game end) the
// seats' view hashes, so a skipped scan that changed the play shows.
func sbaPlay(t *testing.T, pool string, steps int) uint64 {
	t.Helper()
	decks := benchDecks(t, pool)
	set := cardsettest.Set(t, pool)
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	ids := func(es []cardset.Entry) []CardID {
		var out []CardID
		for _, e := range es {
			i, ok := set.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s: not in the pool", e.Name)
			}
			for k := 0; k < e.Count; k++ {
				out = append(out, set.PoolID(i))
			}
		}
		return out
	}
	Register(tbl)
	defer ReleaseTable(tbl)
	var s State
	var d Decision
	pol := NewRandomPolicy(7)
	cfg := Config{Mulligans: 2}
	g := 0
	init := func() {
		s.Init(tbl, ids(decks[2*(g%64)]), ids(decks[2*(g%64)+1]), uint64(g)+1, cfg)
	}
	init()
	h := uint64(0)
	for i := 0; i < steps; i++ {
		s.StepInto(&d)
		if d.Kind == DkNone {
			h = hmix(h, s.ViewHash(0, nil)^s.ViewHash(1, nil))
			g++
			init()
			s.StepInto(&d)
		}
		h = hmix(h, uint64(d.Kind)|uint64(d.Player)<<8)
		if i%16 == 0 {
			h = hmix(h, s.ViewHash(0, nil)^s.ViewHash(1, nil)<<1)
		}
		s.Apply(pol.Pick(&d))
	}
	return h
}

// TestSbaSkipMatchesFullScan: with every skippable scan run anyway (sbaVerify),
// none that the clean-scan key would skip finds a casualty, changes anything or
// reads what the key does not hold. The precondition is that the key does skip.
func TestSbaSkipMatchesFullScan(t *testing.T) {
	for _, pool := range []string{"fdn", "blb"} {
		sbaVerify, sbaHits, sbaBad = true, 0, 0
		sbaPlay(t, pool, 60000)
		hits, bad := sbaHits, sbaBad
		sbaVerify = false
		if hits < 1000 {
			t.Fatalf("%s: the key skipped %d scans in 60000 steps: the test does not reach the skip", pool, hits)
		}
		if bad != 0 {
			t.Fatalf("%s: %d of %d skipped scans would have found something", pool, bad, hits)
		}
		t.Logf("%s: %d scans the key skips, none wrong", pool, hits)
	}
}

// TestSbaSkipPlaysAsFullScan: the play with the skip and the play that always
// scans (sbaMutate 3) are the same play.
func TestSbaSkipPlaysAsFullScan(t *testing.T) {
	for _, pool := range []string{"fdn", "blb"} {
		on := sbaPlay(t, pool, 60000)
		sbaMutate = 3
		off := sbaPlay(t, pool, 60000)
		sbaMutate = 0
		if on != off {
			t.Fatalf("%s: the play with the scan skip (%x) is not the play without it (%x)", pool, on, off)
		}
	}
}

// TestSbaSkipKeyIsLoadBearing: a key that ignores the touch count skips scans
// that find casualties, and both of the checks above catch it.
func TestSbaSkipKeyIsLoadBearing(t *testing.T) {
	sbaVerify, sbaHits, sbaBad, sbaMutate = true, 0, 0, 1
	defer func() { sbaVerify, sbaMutate = false, 0 }()
	sbaPlay(t, "fdn", 60000)
	if sbaBad == 0 {
		t.Fatalf("a key without the touch count skipped %d scans and none was wrong: the check cannot fail", sbaHits)
	}
	t.Logf("without the touch count %d of %d skipped scans are wrong", sbaBad, sbaHits)
	sbaVerify, sbaMutate = false, 1
	broken := sbaPlay(t, "fdn", 60000)
	sbaMutate = 3
	if want := sbaPlay(t, "fdn", 60000); broken == want {
		t.Fatalf("a key without the touch count played the same game: the equivalence check cannot fail")
	}
}

// TestSbaFingerprintReadsItsInputs: each non-object input of the key moves the
// fingerprint (the pools above cannot tell, none of their statics reads them).
func TestSbaFingerprintReadsItsInputs(t *testing.T) {
	decks := benchDecks(t, "fdn")
	set := cardsettest.Set(t, "fdn")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	Register(tbl)
	defer ReleaseTable(tbl)
	var ids [2][]CardID
	for i := range ids {
		for _, e := range decks[i] {
			j, _ := set.Lookup(e.Name)
			for k := 0; k < e.Count; k++ {
				ids[i] = append(ids[i], set.PoolID(j))
			}
		}
	}
	var s State
	s.Init(tbl, ids[0], ids[1], 3, Config{Mulligans: 0})
	base := s.sbaFingerprint()
	for name, mut := range map[string]func(){
		"turn":        func() { s.turn++ },
		"step":        func() { s.step++ },
		"active":      func() { s.active ^= 1 },
		"layer clock": func() { s.ts++ },
		"life":        func() { s.pl[1].life-- },
		"life gained": func() { s.pl[0].lifeGained++ },
		"life lost":   func() { s.pl[0].lifeLost++ },
		"lost":        func() { s.pl[1].lost = true },
		"lands":       func() { s.pl[0].landsPlayed++ },
		"sacrificed":  func() { s.pl[0].sac[1]++ },
		"discarded":   func() { s.pl[1].disc++ },
		"died":        func() { s.g.died++ },
		"attacked":    func() { s.g.attacked[1]++ },
		"cast":        func() { s.g.nused++ },
		"day":         func() { s.g.dayNight = 1 },
		"spent":       func() { s.g.spent[0]++ },
		"history":     func() { s.g.hist[1][2]++ },
	} {
		saved := s.stateCore
		mut()
		if s.sbaFingerprint() == base {
			t.Errorf("the fingerprint does not read %s", name)
		}
		s.stateCore = saved
	}
	if s.sbaFingerprint() != base {
		t.Fatal("the fingerprint is not a function of the state")
	}
}
