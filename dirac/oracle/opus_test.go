package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The lockstep: constructed matchups over the generated opus pool,
// whose carriers read TriggeredCard$CastTotalManaSpent in a SpellCast
// trigger (kop.Tf2CastSpent): the spell's pool mana spent, snapshotted as
// the trigger fires. While one is on a battlefield gorge's payment planner
// prices no cast (shape:mana_spent_reader), for either player. The frozen
// FDN and BLB pair pools carry no reader.

var kopusDecks = [][2][]cardset.Entry{{
	{
		{Name: "Shantotto, Tactician Magician", Count: 2}, {Name: "Tellah, Great Sage", Count: 2}, {Name: "Grandmother Goby", Count: 2},
		{Name: "Colorstorm Stallion", Count: 2}, {Name: "Elemental Mascot", Count: 1}, {Name: "Spectacular Skywhale", Count: 1},
		{Name: "Lightning Bolt", Count: 2}, {Name: "Burst Lightning", Count: 2}, {Name: "Blaze", Count: 2},
		{Name: "Crater's Claws", Count: 2}, {Name: "Wizard's Lightning", Count: 2}, {Name: "Divination", Count: 1},
		{Name: "Rite of Replication", Count: 1}, {Name: "Feldon's Cane", Count: 1}, {Name: "Bident of Thassa", Count: 1},
		{Name: "Aetherflux Conduit", Count: 1}, {Name: "Mountain", Count: 8}, {Name: "Island", Count: 7},
	},
	{
		{Name: "Deluge Virtuoso", Count: 2}, {Name: "Exhibition Tidecaller", Count: 2}, {Name: "Muse Seeker", Count: 2},
		{Name: "Tackle Artist", Count: 2}, {Name: "Thunderdrum Soloist", Count: 2}, {Name: "Molten-Core Maestro", Count: 2},
		{Name: "Expressive Firedancer", Count: 2}, {Name: "Goblin Electromancer", Count: 2}, {Name: "Counterspell", Count: 2},
		{Name: "Memory Lapse", Count: 2}, {Name: "Essence Scatter", Count: 1}, {Name: "Shock", Count: 1},
		{Name: "Abandon Attachments", Count: 1}, {Name: "Preordain", Count: 1}, {Name: "Opt", Count: 1},
		{Name: "Mountain", Count: 8}, {Name: "Island", Count: 7},
	},
}, {
	{
		{Name: "Aberrant Manawurm", Count: 4}, {Name: "Giant Growth", Count: 3}, {Name: "Opt", Count: 2},
		{Name: "Counterspell", Count: 2}, {Name: "Memory Lapse", Count: 2}, {Name: "Essence Scatter", Count: 2},
		{Name: "Exhibition Tidecaller", Count: 2}, {Name: "Muse Seeker", Count: 2}, {Name: "Doom Blade", Count: 1},
		{Name: "Forest", Count: 9}, {Name: "Island", Count: 11},
	},
	{
		{Name: "Shantotto, Tactician Magician", Count: 2}, {Name: "Tellah, Great Sage", Count: 2}, {Name: "Grandmother Goby", Count: 2},
		{Name: "Colorstorm Stallion", Count: 2}, {Name: "Elemental Mascot", Count: 2}, {Name: "Spectacular Skywhale", Count: 1},
		{Name: "Lightning Bolt", Count: 2}, {Name: "Burst Lightning", Count: 2}, {Name: "Blaze", Count: 2},
		{Name: "Crater's Claws", Count: 2}, {Name: "Wizard's Lightning", Count: 2}, {Name: "Divination", Count: 1},
		{Name: "Rite of Replication", Count: 1}, {Name: "Feldon's Cane", Count: 1}, {Name: "Aetherflux Conduit", Count: 1},
		{Name: "Mountain", Count: 9}, {Name: "Island", Count: 6},
	},
}}

type kopusKey struct {
	card, ab uint16
	obj      dirac.Obj
	amt      int16
}

// kopusProbe counts, at every decision, the Tf2CastSpent trigger entries
// newly on the kernel's stack (the amount queued, per carrier, and the
// shape of the spell's spend) and the casts made while a reader is out.
type kopusProbe struct {
	n    map[string]int
	base Policy
	rs   *realSide
	surf int
	prev []kopusKey
	done map[kopusKey]bool
}

var kopusCounters = map[string]bool{"Counterspell": true, "Memory Lapse": true, "Essence Scatter": true}

func (p *kopusProbe) note(k string) { p.n[k]++ }

func (p *kopusProbe) scan() {
	s := &p.rs.s
	var cur []kopusKey
	for i := 0; i < s.StackLen(); i++ {
		amt, spell, ok := s.OpusSpentTrigger(i)
		if !ok {
			continue
		}
		_, src, card, _ := s.StackEntry(i)
		key := kopusKey{uint16(card), uint16(s.StackAbility(i)), src, amt}
		cur = append(cur, key)
		if !p.done[key] {
			for j := i + 1; j < s.StackLen(); j++ {
				if kd, _, c2, _ := s.StackEntry(j); kd == 0 && kopusCounters[p.rs.tbl.Cards[c2].Name] {
					p.done[key] = true
					p.note("read after a counterspell in response")
					break
				}
			}
		}
		seen := false
		for j, q := range p.prev {
			if q == key {
				p.prev[j].amt = -999
				seen = true
				break
			}
		}
		if seen {
			continue
		}
		name := p.rs.tbl.Cards[card].Name
		switch {
		case amt >= 1 && amt <= 4:
			p.note("amt1-4")
		}
		if amt >= 5 {
			p.note("amt>=5")
		}
		if amt >= 8 {
			p.note("amt>=8")
		}
		tf := func(k string, b bool) {
			if b {
				p.note(k + " true")
			} else {
				p.note(k + " false")
			}
		}
		switch name {
		case "Deluge Virtuoso", "Tackle Artist", "Thunderdrum Soloist", "Exhibition Tidecaller":
			tf("compare GE5", amt >= 5)
		case "Colorstorm Stallion", "Elemental Mascot", "Expressive Firedancer", "Molten-Core Maestro", "Spectacular Skywhale":
			tf("cond GE5", amt >= 5)
		case "Muse Seeker":
			tf("cond LT5", amt < 5)
		case "Shantotto, Tactician Magician":
			tf("cond GE4", amt >= 4)
		case "Tellah, Great Sage":
			tf("cond GE4", amt >= 4)
			tf("cond GE8", amt >= 8)
		}
		switch name {
		case "Aberrant Manawurm", "Shantotto, Tactician Magician", "Aetherflux Conduit", "Grandmother Goby":
			if amt >= 1 {
				p.note("plain " + name)
			}
		}
		if name == "Grandmother Goby" && amt >= 1 {
			p.note("read after the spell left the stack (Goby)")
		}
		sc := &p.rs.tbl.Cards[spell]
		mv := sc.Cost.MV()
		switch {
		case int(amt) < mv:
			p.note("spend below MV (reduced)")
		case int(amt) > mv && sc.Cost.X != 0:
			p.note("spend above MV (X paid)")
		case int(amt) > mv:
			p.note("spend above MV (kicked)")
		}
	}
	p.prev = cur
}

func (p *kopusProbe) pick(c *PickCtx) Canon {
	p.scan()
	a := p.base.Pick(c)
	if a.Kind() == ActCast && p.rs.s.OpusReaderOut() {
		who := "opponent"
		if p.rs.s.OpusReaderCtrl(c.D.Player) {
			who = "reader's controller"
		}
		surf := "lowering surface"
		if p.surf == 1 {
			surf = "autopay surface"
		}
		p.note("cast while a reader is out: " + who + ", " + surf)
	}
	return a
}

var kopusBranches = []string{
	"amt1-4", "amt>=5", "amt>=8",
	"compare GE5 true", "compare GE5 false", "cond GE5 true", "cond GE5 false", "cond LT5 true", "cond LT5 false",
	"cond GE4 true", "cond GE4 false", "cond GE8 true", "cond GE8 false",
	"plain Aberrant Manawurm", "plain Shantotto, Tactician Magician", "plain Aetherflux Conduit", "plain Grandmother Goby",
	"read after the spell left the stack (Goby)", "read after a counterspell in response",
	"spend below MV (reduced)", "spend above MV (X paid)", "spend above MV (kicked)",
	"cast while a reader is out: reader's controller, autopay surface",
	"cast while a reader is out: reader's controller, lowering surface",
	"cast while a reader is out: opponent, autopay surface",
	"cast while a reader is out: opponent, lowering surface",
}

func kopusSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("opus")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

// kopusRun plays seeds 1..seeds of every matchup; it returns the games
// played, whether any diverged (the first divergence is logged) and the
// stalls.
func kopusRun(t *testing.T, reg *cards.Registry, set cardset.Set, seeds uint64, n map[string]int, stopOnDiv bool) (games int, div bool, stalls int) {
	t.Helper()
	for mi, md := range kopusDecks {
		decks := kcastDeck(t, reg, md)
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kopus-a", "kopus-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			base := Policy(HeuristicPolicy(seed))
			if seed%3 == 0 {
				base = RandomPolicy(seed)
			}
			probe := &kopusProbe{n: n, base: base, rs: k.(*realSide), done: map[kopusKey]bool{}}
			if cfg.GorgeAutopaySurface {
				probe.surf = 1
			}
			res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			games++
			if res.Stall != "" && len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
				stalls++
				// A declared capacity is an outcome the game agreed up to (Rite of
				// Replication's copies of the readers can put 16 triggers on a live
				// stack); a rule gap is not.
				if res.StallUnmodelled {
					t.Errorf("matchup %d seed %d: kernel stall %s", mi, seed, res.Stall)
				}
			}
			if d := res.Div; d != nil {
				div = true
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				if !stopOnDiv {
					return
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				return
			}
		}
	}
	return
}

func TestOpusLockstep(t *testing.T) {
	t.Parallel()

	reg, set := kopusSetup(t)
	n := map[string]int{}
	games, div, stalls := kopusRun(t, reg, set, 512, n, true)
	t.Logf("%d games, diverged %v, capacity stalls %d, counts %v", games, div, stalls, n)
	if div {
		return
	}
	if stalls*100 > games {
		t.Errorf("%d capacity stalls in %d games, want at most 1%%", stalls, games)
	}
	for _, b := range kopusBranches {
		if n[b] < 10 {
			t.Errorf("branch %q taken %d times, want at least 10", b, n[b])
		}
	}
}

// TestOpusMutation: each switch must make the lockstep diverge on some seed.
func TestOpusMutation(t *testing.T) {
	reg, set := kopusSetup(t)
	for _, c := range []struct {
		name string
		set  func(bool)
	}{
		{"amount is the printed mana value", func(on bool) { dirac.DebugOpusMV = on }},
		{"live spell entry at resolution", func(on bool) { dirac.DebugOpusLive = on }},
		{"planner decline ignored", func(on bool) { dirac.DebugOpusNoDecline = on }},
	} {
		t.Run(c.name, func(t *testing.T) {
			c.set(true)
			defer c.set(false)
			_, div, _ := kopusRun(t, reg, set, 64, map[string]int{}, false)
			if !div {
				t.Fatalf("%s: no divergence in 64 seeds per matchup", c.name)
			}
		})
	}
}

var _ = kop.Tf2CastSpent
