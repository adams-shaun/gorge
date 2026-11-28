package dirac

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The quiet-seat proof's verify runs (cast_quietseat.go,
// docs/design/dirac-quiet-seat-proof.md): every posed priority walk runs with
// the proof read beside it, and a proved window whose walk offers more than
// pass, or changes the State beyond the filter-bound memo, panics.

// quietVerifyRun plays n transitions of the bench's four-seat edh pods under
// cfg (game g is pod g%16 under seed g+1, as BenchmarkTransitionEDH4 plays
// them) with DebugQuietVerify on, and returns the tally. A kernel panic that
// is not the verify's abandons that game, as the bench does.
func quietVerifyRun(t *testing.T, cfg Config, n int) *QuietStats {
	t.Helper()
	pods := edhPods(t, 16)
	st := &QuietStats{Panic: true}
	DebugQuietVerify = st
	t.Cleanup(func() { DebugQuietVerify = nil })
	var s State
	var d Decision
	var pol RandomPolicy
	g := 0
	start := func() {
		p := &pods[g%len(pods)]
		s.InitN(p.tbl, p.decks, MaxSeats, uint64(g)+1, cfg)
		pol = NewRandomPolicy(uint64(g) + 1)
	}
	start()
	for i := 0; i < n; {
		func() {
			defer func() {
				if r := recover(); r != nil {
					if msg, ok := r.(string); ok && strings.HasPrefix(msg, "dirac: quiet proof") {
						t.Fatal(msg)
					}
					i++
					g++
					start()
				}
			}()
			for ; i < n; i++ {
				s.StepInto(&d)
				if d.Kind == DkNone {
					g++
					start()
					s.StepInto(&d)
				}
				s.Apply(pol.Pick(&d))
			}
		}()
	}
	quietReport(t, st)
	return st
}

func quietReport(t *testing.T, st *QuietStats) {
	t.Helper()
	if st.Proved == 0 {
		t.Fatalf("the proof held for no window of %d walks", st.Walks)
	}
	t.Logf("%d walks, %d pass-only; proved %d (%.1f%% of walks, %.1f%% of pass-only), %d rewrote only the fbp memo",
		st.Walks, st.PassOnly, st.Proved, 100*float64(st.Proved)/float64(st.Walks), 100*float64(st.ProvedPass)/float64(max(st.PassOnly, 1)), st.MemoOnly)
	for b := 1; b < int(qbN); b++ {
		if st.Blocker[b] != 0 {
			t.Logf("  unproved pass-only, first blocker %-12s %5.1f%%", QuietBlockerName(b), 100*float64(st.Blocker[b])/float64(st.PassOnly))
		}
	}
}

// TestQuietProofEDH: the bench's configuration.
func TestQuietProofEDH(t *testing.T) { quietVerifyRun(t, Config{Mulligans: 2}, 100000) }

// TestQuietProofEDHAutopay: the lockstep auto-pay surface.
func TestQuietProofEDHAutopay(t *testing.T) {
	cfg := LockstepConfig(SurfaceAutopay)
	cfg.Mulligans = 2
	quietVerifyRun(t, cfg, 100000)
}

// TestQuietProofFDN: the FDN bench's 64 two-seat pairings (fdnBenchTables),
// 200000 random-policy transitions, every decision posed in the second half.
func TestQuietProofFDN(t *testing.T) {
	games := fdnBenchTables(t, 64)
	st := &QuietStats{Panic: true}
	DebugQuietVerify = st
	t.Cleanup(func() { DebugQuietVerify = nil })
	var s State
	var d Decision
	pol := NewRandomPolicy(1)
	g := 0
	cfg := Config{Mulligans: 2}
	s.Init(games[0].tbl, games[0].da, games[0].db, 1, cfg)
	for i := 0; i < 200000; i++ {
		if i == 100000 {
			cfg.NoElide = true
		}
		s.StepInto(&d)
		if d.Kind == DkNone {
			g++
			gm := &games[g%len(games)]
			s.Init(gm.tbl, gm.da, gm.db, uint64(g)+1, cfg)
			s.StepInto(&d)
		}
		s.Apply(pol.Pick(&d))
	}
	quietReport(t, st)
}

// TestQuietFloor: the least mana a cost takes.
func TestQuietFloor(t *testing.T) {
	cases := []struct {
		m    kop.ManaCost
		want uint8
	}{
		{kop.ManaCost{}, 0},
		{kop.ManaCost{Generic: 2, Pips: [kop.NumManaKinds]uint8{1, 0, 0, 0, 1}}, 4},
		{kop.ManaCost{Generic: 1, X: 2}, 1},                                                                // X at 0
		{kop.ManaCost{Pips: [kop.NumManaKinds]uint8{0, 1}, Hyb: [5]kop.ColorMask{kop.ColorMask(0b11)}}, 2}, // a hybrid pip is one mana
		{kop.ManaCost{Generic: 1, Snow: 1}, 2},
	}
	for _, c := range cases {
		if got := quietFloor(&c.m); got != c.want {
			t.Errorf("quietFloor(%+v) = %d, want %d", c.m, got, c.want)
		}
	}
}

// TestQuietHarmlessKinds: the kinds that lower a payment or grant a route
// are never harmless, and a kind past the last one named is not either (a
// new kind is qMod until it is classified).
func TestQuietHarmlessKinds(t *testing.T) {
	for _, sk := range []kop.StaticKind{kop.StatReduceCost, kop.StatAlternativeCost, kop.StatSetCost, kop.StatDefiler,
		kop.StatReduceAbility, kop.StatStackKw, kop.StatWarpGrant, kop.StatTapExtra, kop.StatCyclingAlt} {
		if quietHarmless(sk) {
			t.Errorf("kind %d is harmless", sk)
		}
	}
	if quietHarmless(kop.StatLethalByPower + 1) {
		t.Error("an unnamed kind is harmless")
	}
	if !quietHarmless(kop.StatContinuous) || !quietHarmless(kop.StatIncreaseCost) {
		t.Error("a plain kind is not harmless")
	}
}
