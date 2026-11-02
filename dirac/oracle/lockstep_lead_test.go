package oracle

import (
	"strings"
	"testing"
)

// extraOption is a faulty kernel side: its leader decision carries one
// option more than the gorge side's, so the first decision diverges.
// plainKernel is a kernel-labelled side with no fault: the gorgo engine
// behind it, but named as the kernel (as the real adapter is).
type plainKernel struct{ Side }

type extraOption struct{ Side }

func (s extraOption) Step() CDecision {
	d := s.Side.Step()
	if d.Kind != DNone {
		d.Options = append(d.Options, Pack(ActChoose+120, RefNone, RefNone, 0))
	}
	return d
}

// TestLockstepKernelLed plays a few FDN pairings with the kernel leading and
// gorge following, under the random policy, and requires no divergence.
func TestLockstepKernelLed(t *testing.T) {
	t.Parallel()

	c, pool := testCorpus(t)
	cfg := Config{GorgeAutopaySurface: true}
	for _, p := range Pairings(pool, 3, 20261017) {
		m, tbl, err := SetMatchup(rawSet("fdn"), c, p.A, p.B, 0)
		if err != nil {
			t.Fatal(err)
		}
		lead, err := SetSide(rawSet("fdn"), tbl, cfg)(m, p.Seed)
		if err != nil {
			t.Fatal(err)
		}
		follow, err := NewGorgeSide(m, p.Seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, p.Seed, lead, follow, RandomPolicy(p.Seed), RunConfig{Config: cfg, MaxDecisions: 20000, MaxTurns: 100})
		CloseSide(lead)
		if res.Div != nil {
			t.Fatalf("pair %d (%s vs %s seed %d): %v\n%v\nref %v\nker %v", p.Index, p.A, p.B, p.Seed, res.Div, res.Div.Diff, res.Div.RefOptions, res.Div.KerOptions)
		}
		if !res.Over && res.Stall == "" {
			t.Fatalf("pair %d: game neither over nor stalled", p.Index)
		}
		t.Logf("pair %d %s vs %s: %d decisions, turn %d, winner %d, stall %q", p.Index, p.A, p.B, res.Decisions, res.Turns, res.Winner, res.Stall)
	}
}

// TestLockstepLedReportsLeader injects an option-list mismatch on the kernel
// side and checks the divergence names the side that led: the kernel when it
// leads, gorge when it follows.
func TestLockstepLedReportsLeader(t *testing.T) {
	t.Parallel()

	c, pool := testCorpus(t)
	p := Pairings(pool, 1, 20261017)[0]
	m, err := c.Matchup(p.A, p.B, 0)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{}

	lead, err := GorgeKernel(cfg)(m, p.Seed)
	if err != nil {
		t.Fatal(err)
	}
	follow, err := NewGorgeSide(m, p.Seed, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	res := LockstepLed(m, p.Seed, extraOption{lead}, follow, FirstPolicy(), RunConfig{Config: cfg})
	if res.Div == nil || res.Div.What != "options" {
		t.Fatalf("kernel-led mismatch: %v, want an options divergence", res.Div)
	}
	if res.Div.Lead != "kernel" {
		t.Fatalf("kernel-led mismatch: lead %q, want %q", res.Div.Lead, "kernel")
	}

	k, err := GorgeKernel(cfg)(m, p.Seed)
	if err != nil {
		t.Fatal(err)
	}
	res = Lockstep(m, p.Seed, extraOption{k}, FirstPolicy(), RunConfig{Config: cfg})
	if res.Div == nil || res.Div.What != "options" {
		t.Fatalf("gorge-led mismatch: %v, want an options divergence", res.Div)
	}
	if res.Div.Lead != "gorge" {
		t.Fatalf("gorge-led mismatch: lead %q, want %q", res.Div.Lead, "gorge")
	}
}

// overSide is a side whose game is over at once (the kernel after a loss).
type overSide struct{ name string }

func (s overSide) Step() CDecision        { return CDecision{Kind: DNone} }
func (s overSide) Apply(Canon)            {}
func (s overSide) Terminal() (bool, int8) { return true, 0 }
func (s overSide) Decided() (bool, int8)  { return true, 0 }
func (s overSide) Project(p *Projection)  { *p = Projection{Over: true, Winner: 0} }
func (s overSide) Dirty() bool            { return true }
func (s overSide) Turn() int              { return 1 }
func (s overSide) Err() error             { return nil }
func (s overSide) SideName() string       { return s.name }

// askingSide is a side that is decided but still poses a decision (gorge
// removing the loser's objects before it ends the game, then a legend ask).
type askingSide struct{ name string }

func (s askingSide) Step() CDecision {
	return CDecision{Kind: DPriority, Player: 0,
		Options: []Canon{Pack(ActPass, RefNone, RefNone, 0)}}
}
func (s askingSide) Apply(Canon)            {}
func (s askingSide) Terminal() (bool, int8) { return false, -1 }
func (s askingSide) Decided() (bool, int8)  { return true, 0 }
func (s askingSide) Project(p *Projection)  { *p = Projection{Winner: -1} }
func (s askingSide) Dirty() bool            { return true }
func (s askingSide) Turn() int              { return 1 }
func (s askingSide) Err() error             { return nil }
func (s askingSide) SideName() string       { return s.name }

// TestLockstepAsksAfterLossBothDirections: the "gorge asks after the loss"
// terminal state is keyed on the engines, not their roles. It must report the
// same class whichever side leads: the gorge side asks (Decided, not
// Terminal), the kernel side is over (Terminal), and the detail names the
// asking side's decision.
func TestLockstepAsksAfterLossBothDirections(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		lead, follow Side
		leadName     string
	}{
		{"gorge leads", askingSide{name: "gorge"}, overSide{name: "kernel"}, "gorge"},
		{"kernel leads", overSide{name: "kernel"}, askingSide{name: "gorge"}, "kernel"},
	} {
		res := LockstepLed(nil, 1, tc.lead, tc.follow, FirstPolicy(), RunConfig{})
		if res.Div == nil || res.Div.What != "terminal" || res.Div.Field != "gorge asks after the loss" {
			t.Fatalf("%s: %v, want terminal \"gorge asks after the loss\"", tc.name, res.Div)
		}
		if res.Div.Lead != tc.leadName {
			t.Fatalf("%s: lead %q, want %q", tc.name, res.Div.Lead, tc.leadName)
		}
		if !res.Over || res.Winner != 0 {
			t.Fatalf("%s: over=%v winner=%d, want over winner 0", tc.name, res.Over, res.Winner)
		}
		if !strings.Contains(res.Div.Detail, "priority decision for p0") {
			t.Fatalf("%s: detail %q does not name the asking side's decision", tc.name, res.Div.Detail)
		}
	}
}

// TestHeuristicPolicyKernelLedRefuses: the heuristic policy drives gorge's
// pending decision, so a kernel-led run reports its refusal as a policy
// panic instead of silently picking something else.
func TestHeuristicPolicyKernelLedRefuses(t *testing.T) {
	t.Parallel()

	c, pool := testCorpus(t)
	p := Pairings(pool, 1, 20261017)[0]
	m, err := c.Matchup(p.A, p.B, 0)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{}
	lead, err := GorgeKernel(cfg)(m, p.Seed)
	if err != nil {
		t.Fatal(err)
	}
	follow, err := NewGorgeSide(m, p.Seed, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	res := LockstepLed(m, p.Seed, plainKernel{lead}, follow, HeuristicPolicy(p.Seed), RunConfig{Config: cfg})
	if res.Div == nil || res.Div.What != "policy" || res.Div.Field != "panic" {
		t.Fatalf("kernel-led heuristic: %v, want a policy panic", res.Div)
	}
	if res.Div.Lead != "kernel" {
		t.Fatalf("kernel-led heuristic: lead %q, want %q", res.Div.Lead, "kernel")
	}
	if !strings.Contains(res.Div.Detail, "needs gorge to lead") {
		t.Fatalf("kernel-led heuristic: detail %q does not name the refusal", res.Div.Detail)
	}
}
