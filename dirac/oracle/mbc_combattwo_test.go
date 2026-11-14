package oracle

import (
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The MBC combat-2 spelling family, one carrier per spelling in the combat2
// fixture pool (dirac/cardset/pools/fixtures/combat2):
//   - blockingTriggeredAttacker (kop.W7BlockingTrigObj): General Marhault
//     Elsdragon and Spined Sliver pump an attacker per creature blocking it;
//   - TriggeredDefender.Opponent & Valid Planeswalker.TriggeredDefender
//     from any Attacks trigger (AtkPWGuard + DealDamageFrom's defending
//     mode): Mage Slayer's equipped attacker deals its power to the player
//     it attacks;
//   - !blocking (kop.W7NotBlocking): Intrepid Ace's +2/+0 while it is not
//     attacking or blocking, Unlikely Alliance's nonattacking, nonblocking
//     target;
//   - RemoveFromCombat (kop.ExtRemoveFromCombat): Reconnaissance's {0} and
//     the Gustcloak creatures' "untap it and remove it from combat".
//
// Before the lowering every carrier was refused, so the matchup table did
// not build. The scan asserts each spelling's carrier did its thing in
// gorge's log.

var kcombat2Decks = [2][]cardset.Entry{
	{
		{Name: "General Marhault Elsdragon", Count: 4}, {Name: "Mage Slayer", Count: 4}, {Name: "Intrepid Ace", Count: 4},
		{Name: "Spined Sliver", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
	{
		{Name: "Reconnaissance", Count: 4}, {Name: "Gustcloak Sentinel", Count: 4}, {Name: "Gustcloak Runner", Count: 4},
		{Name: "Unlikely Alliance", Count: 4}, {Name: "Savannah Lions", Count: 4}, {Name: "Plains", Count: 20},
	},
}

// combat2Tally counts, over gorge's event logs, what each spelling's
// carriers did.
type combat2Tally struct {
	Abilities map[string]int // ability resolutions by the source's name
	Cast      map[string]int // casts from the hand by name
	// Removed is the EndCombatReset events naming one object next to an
	// Untap of that object (both carriers untap what they remove), by the
	// resolving ability's source: a player leaving the game resets its
	// attackers with no untap
	Removed map[string]int
}

func newCombat2Tally() combat2Tally {
	return combat2Tally{Abilities: map[string]int{}, Cast: map[string]int{}, Removed: map[string]int{}}
}

func (c *combat2Tally) scan(g *GorgeSide) {
	if g == nil {
		return
	}
	name := func(id state.ObjID) string {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	evs := g.E.L.Events
	untapped := func(j int, id state.ObjID) bool {
		return j >= 0 && j < len(evs) && evs[j].Kind == events.Untap && evs[j].Obj == id
	}
	resolving := "" // the source of the ability resolving now ("" for a spell)
	for i, ev := range evs {
		switch ev.Kind {
		case events.Resolve:
			// an ability's Resolve names nothing: its stack object's source does
			resolving = ""
			if o := g.E.G.Obj(ev.Obj); o != nil && o.Ability != nil {
				resolving = name(o.Source)
				c.Abilities[resolving]++
			}
		case events.PutOnStack:
			if ev.From == state.ZHand {
				c.Cast[name(ev.Obj)]++
			}
		case events.EndCombatReset:
			if ev.Obj != 0 && (untapped(i-1, ev.Obj) || untapped(i+1, ev.Obj)) {
				c.Removed[resolving]++
			}
		}
	}
}

func (c *combat2Tally) add(o combat2Tally) {
	for k, v := range o.Abilities {
		c.Abilities[k] += v
	}
	for k, v := range o.Cast {
		c.Cast[k] += v
	}
	for k, v := range o.Removed {
		c.Removed[k] += v
	}
}

// combat2Probe is castFirstPolicy keeping the gorge side it was handed. It
// passes rather than activate Reconnaissance's {0} again over a non-empty
// stack: castFirstPolicy would pile free activations until the decision cap.
type combat2Probe struct {
	m    *Matchup
	base Policy
	gs   *GorgeSide
}

func (p *combat2Probe) pick(c *PickCtx) Canon {
	if c.G != nil {
		p.gs = c.G
	}
	a := p.base.Pick(c)
	if a.Kind() != ActActivate || c.G == nil || len(c.G.E.G.Stack) == 0 || firstObjectCard(p.m, a.String()) != "Reconnaissance" {
		return a
	}
	for _, o := range c.D.Options {
		if o.Kind() == ActPass {
			return o
		}
	}
	return a
}

func TestMBCCombat2Lowers(t *testing.T) {
	t.Parallel()
	seeds := uint64(200)
	if v := os.Getenv("KCOMBAT2_SEEDS"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			seeds = n
		}
	}
	reg, set, gd := lockstepPool(t, "combat2", kcombat2Decks)
	tally := newCombat2Tally()
	clean := 0
	seedChunks(t, 1, seeds, lockstepChunk, func(t *testing.T, lo, hi uint64, mu *sync.Mutex) {
		tbl, _, _, err := set.Matchup(kcombat2Decks[0], kcombat2Decks[1])
		if err != nil {
			t.Fatal(err)
		}
		local := newCombat2Tally()
		n := 0
		for seed := lo; seed <= hi; seed++ {
			m, err := NewMatchup(reg, [2]string{"combat2-a", "combat2-b"}, gd, 1)
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{GorgeAutopaySurface: seed%4 == 0}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			probe := &combat2Probe{m: m, base: castFirstPolicy(seed)}
			res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			if err := unmodelledStallError(t.Name(), &res); err != nil {
				t.Errorf("seed %d: %v", seed, err)
			}
			if d := res.Div; d != nil {
				from := max(d.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[from:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				continue
			}
			n++
			local.scan(probe.gs)
		}
		mu.Lock()
		defer mu.Unlock()
		clean += n
		tally.add(local)
	})
	if t.Failed() {
		return
	}
	t.Logf("%d seeds, %d clean; ability resolutions %v; cast %v; removed from combat %v", seeds, clean, tally.Abilities, tally.Cast, tally.Removed)
	for _, name := range []string{"General Marhault Elsdragon", "Spined Sliver", "Mage Slayer", "Unlikely Alliance", "Reconnaissance", "Gustcloak Runner"} {
		if tally.Abilities[name] == 0 {
			t.Errorf("no ability of %s resolved", name)
		}
	}
	if tally.Cast["Intrepid Ace"] == 0 {
		t.Errorf("Intrepid Ace never cast")
	}
	for _, name := range []string{"Reconnaissance", "Gustcloak Runner"} {
		if tally.Removed[name] == 0 {
			t.Errorf("%s never removed a creature from combat", name)
		}
	}
}
