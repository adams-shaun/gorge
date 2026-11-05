package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The V'Ger, the Intruder lockstep: its charm's second mode is a PumpAll
// Duration$ UntilYourNextTurn -1/-0 on the creatures its controller's
// opponents control (CR 611.2c: the creatures there when it resolves, not
// the ones that enter later), which ends at the cleanup before the
// controller's next turn. The kernel's P/T pump record (kop.Override.PTDelta)
// must show gorge's powers at every decision, through that next turn.

var kvgerDecks = [2][]cardset.Entry{
	{ // A: V'Ger, rocks to cast it, and a creature of its own the pump must not touch
		{Name: "V'Ger, the Intruder", Count: 6},
		{Name: "Sol Ring", Count: 4},
		{Name: "Mind Stone", Count: 4},
		{Name: "Grizzly Bears", Count: 4},
		{Name: "Island", Count: 22},
	},
	{ // B: creatures to pump, cheap enough to enter before and after
		{Name: "Grizzly Bears", Count: 10},
		{Name: "Hill Giant", Count: 6},
		{Name: "Forest", Count: 12},
		{Name: "Mountain", Count: 12},
	},
}

type kvgerStats struct {
	pumped  int // opposing creatures first seen with less power than printed
	fresh   int // creatures that entered unreduced while another opposing creature was reduced
	expired int // reduced creatures seen at printed power again on V'Ger's controller's turn
}

// kvgerProbe wraps the casting policy and reads the powers through gorge's
// projection at every decision (gorge leads, so PickCtx.G is set).
type kvgerProbe struct {
	st      *kvgerStats
	base    Policy
	tbl     *kop.Table
	pj      Projection
	reduced [MaxInst]bool // the instance was seen with less than printed power
	seen    [MaxInst]bool // the instance's first sighting is counted
}

func (p *kvgerProbe) Pick(c *PickCtx) Canon {
	p.observe(c)
	return p.base.Pick(c)
}

// observe counts, relative to the controller of an on-battlefield V'Ger, the
// opposing creatures that are reduced, that entered unreduced beside a
// reduced one, and that returned to their printed power on that controller's
// own turn (the pump's expiry).
func (p *kvgerProbe) observe(c *PickCtx) {
	if c.G == nil {
		return
	}
	c.G.Project(&p.pj)
	vger := nameHash("V'Ger, the Intruder")
	owner := -1
	for i := 0; i < int(p.pj.NInst) && i < MaxInst; i++ {
		if o := &p.pj.Cards[i]; o.Zone == ZoneBattlefield && o.NameHash == vger {
			owner = int(o.Controller)
		}
	}
	anyReduced := false
	for i := 0; i < int(p.pj.NInst) && i < MaxInst; i++ {
		o := &p.pj.Cards[i]
		if o.Zone != ZoneBattlefield || uint16(o.Chars.Types)&uint16(kop.TCreature) == 0 || o.Card == NoCardIndex || int(o.Card) >= len(p.tbl.Cards) {
			if o.Zone != ZoneBattlefield {
				p.seen[i], p.reduced[i] = false, false
			}
			continue
		}
		if o.Power < int16(p.tbl.Cards[o.Card].Power) && owner >= 0 && int(o.Controller) != owner {
			anyReduced = true
		}
	}
	for i := 0; i < int(p.pj.NInst) && i < MaxInst; i++ {
		o := &p.pj.Cards[i]
		if o.Zone != ZoneBattlefield || uint16(o.Chars.Types)&uint16(kop.TCreature) == 0 || o.Card == NoCardIndex || int(o.Card) >= len(p.tbl.Cards) {
			continue
		}
		printed := int16(p.tbl.Cards[o.Card].Power)
		first := !p.seen[i]
		p.seen[i] = true
		switch {
		case o.Power < printed && owner >= 0 && int(o.Controller) != owner:
			if !p.reduced[i] {
				p.st.pumped++
			}
			p.reduced[i] = true
		case o.Power == printed && p.reduced[i] && int(p.pj.Active) == owner:
			p.st.expired++
			p.reduced[i] = false
		case o.Power == printed && first && anyReduced && int(o.Controller) != owner:
			p.st.fresh++
		}
	}
}

// kvgerRun plays seeds 1..seeds and returns the games, the divergences and
// stalls (reported as test errors when report is set) and the probe tallies.
func kvgerRun(t *testing.T, seeds uint64, report bool) (games, divs, stalls int, st kvgerStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("vger")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kvgerDecks {
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
	tbl, _, _, err := set.Matchup(kvgerDecks[0], kvgerDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kvger-a", "kvger-b"}, decks, 1)
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
		probe := &kvgerProbe{st: &st, base: poisonhighPolicy(seed, map[string]int{}, map[string]int{}), tbl: tbl} // casts whenever it can
		res := LockstepLed(m, seed, g, k, probe, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 24})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if report {
				t.Errorf("seed %d: stalled: %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if report {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return
}

func TestVgerTheIntruderLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, st := kvgerRun(t, 128, true)
	t.Logf("%d games, %+v, %d divergences, %d stalls", games, st, divs, stalls)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if st.pumped < 10 {
		t.Errorf("only %d opposing creatures pumped (want at least 10)", st.pumped)
	}
	if st.fresh < 5 {
		t.Errorf("only %d creatures entered unreduced beside a pumped one (want at least 5)", st.fresh)
	}
	if st.expired < 5 {
		t.Errorf("only %d pumps seen to expire (want at least 5)", st.expired)
	}
}

// TestVgerTheIntruderMutation: a P/T pump that survives its controller's
// next turn must make the lockstep diverge.
func TestVgerTheIntruderMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugPTDeltaNoExpire = true
	defer func() { dirac.DebugPTDeltaNoExpire = false }()
	games, divs, _, _ := kvgerRun(t, 32, false)
	if divs == 0 {
		t.Fatalf("mutation kept every pump in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
