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

// The MBC combat-4 spelling family, one carrier per spelling in the
// mbccombat4 fixture pool (dirac/cardset/pools/fixtures/mbccombat4):
//   - K:CantAttackOrBlock (Grakk the Pacifist): the creature is never
//     declared as an attacker or a blocker (KwCantBlock and a self
//     CantAttack static);
//   - blockingSource (kop.W7BlockingSrc): Master of Arms taps, and Cromat
//     destroys, a creature blocking it; Rampaging Cyclops counts its
//     blockers;
//   - blockedBySource (kop.W7BlockedBySrc): Cromat's other half and Alms
//     Beast's lifelink grant to the creatures it blocks;
//   - a targeted Effect's CanAttackDefender grant (Assault Formation): the
//     targeted Wall of Wood attacks.
//
// Before the lowering every carrier was refused, so the matchup table did
// not build. The scan asserts each spelling's carrier did its thing in
// gorge's log.

var kcombat4Decks = [2][]cardset.Entry{
	{
		{Name: "Grakk the Pacifist", Count: 3}, {Name: "Master of Arms", Count: 3}, {Name: "Cromat", Count: 3},
		{Name: "Alms Beast", Count: 2}, {Name: "Rampaging Cyclops", Count: 2}, {Name: "Plague Wight", Count: 3},
		{Name: "Abu Ja'far", Count: 3}, {Name: "Grizzly Bears", Count: 1},
		{Name: "Plains", Count: 8}, {Name: "Forest", Count: 4}, {Name: "Island", Count: 2}, {Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 2},
	},
	{
		{Name: "Assault Formation", Count: 4}, {Name: "Wall of Wood", Count: 4}, {Name: "Savannah Lions", Count: 3},
		{Name: "Master of Arms", Count: 1}, {Name: "Cromat", Count: 1}, {Name: "Alms Beast", Count: 1},
		{Name: "Steadfast Armasaur", Count: 6},
		{Name: "Forest", Count: 10}, {Name: "Plains", Count: 8}, {Name: "Swamp", Count: 2},
	},
}

// combat4Tally counts, over gorge's event logs, what each carrier did.
type combat4Tally struct {
	Abilities map[string]int // ability resolutions by the source's name
	Cast      map[string]int // casts from the hand by name
	Attacked  map[string]int // declared as an attacker, by name
	Blocked   map[string]int // declared as a blocker, by name
	Destroyed map[string]int // battlefield-to-graveyard moves while the named source's ability resolves
}

func newCombat4Tally() combat4Tally {
	return combat4Tally{Abilities: map[string]int{}, Cast: map[string]int{}, Attacked: map[string]int{}, Blocked: map[string]int{}, Destroyed: map[string]int{}}
}

func (c *combat4Tally) scan(g *GorgeSide) {
	if g == nil {
		return
	}
	name := func(id state.ObjID) string {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	resolving := "" // the source of the ability resolving now ("" for a spell)
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.Resolve:
			resolving = ""
			if o := g.E.G.Obj(ev.Obj); o != nil && o.Ability != nil {
				resolving = name(o.Source)
				c.Abilities[resolving]++
			}
		case events.Priority, events.TurnChange:
			resolving = "" // the resolution is over
		case events.MoveZone:
			if ev.From == state.ZBattlefield && ev.To == state.ZGraveyard {
				c.Destroyed[resolving]++
			}
		case events.PutOnStack:
			resolving = ""
			if ev.From == state.ZHand {
				c.Cast[name(ev.Obj)]++
			}
		case events.DeclareAttackers:
			resolving = ""
			for _, id := range ev.IDs {
				c.Attacked[name(id)]++
			}
		case events.DeclareBlockers:
			resolving = ""
			for _, pr := range ev.Pairs {
				c.Blocked[name(pr[1])]++
			}
		}
	}
}

func (c *combat4Tally) add(o combat4Tally) {
	for k, v := range o.Abilities {
		c.Abilities[k] += v
	}
	for k, v := range o.Cast {
		c.Cast[k] += v
	}
	for k, v := range o.Attacked {
		c.Attacked[k] += v
	}
	for k, v := range o.Blocked {
		c.Blocked[k] += v
	}
	for k, v := range o.Destroyed {
		c.Destroyed[k] += v
	}
}

// combat4Probe is castFirstPolicy keeping the gorge side it was handed.
type combat4Probe struct {
	base Policy
	gs   *GorgeSide
}

func (p *combat4Probe) pick(c *PickCtx) Canon {
	if c.G != nil {
		p.gs = c.G
	}
	return p.base.Pick(c)
}

func TestMBCCombat4Lowers(t *testing.T) {
	t.Parallel()
	seeds := uint64(200)
	if v := os.Getenv("KCOMBAT4_SEEDS"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			seeds = n
		}
	}
	reg, set, gd := lockstepPool(t, "mbccombat4", kcombat4Decks)
	tally := newCombat4Tally()
	clean := 0
	seedChunks(t, 1, seeds, lockstepChunk, func(t *testing.T, lo, hi uint64, mu *sync.Mutex) {
		tbl, _, _, err := set.Matchup(kcombat4Decks[0], kcombat4Decks[1])
		if err != nil {
			t.Fatal(err)
		}
		local := newCombat4Tally()
		n := 0
		for seed := lo; seed <= hi; seed++ {
			m, err := NewMatchup(reg, [2]string{"combat4-a", "combat4-b"}, gd, 1)
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{GorgeAutopaySurface: seed%4 == 0}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			probe := &combat4Probe{base: castFirstPolicy(seed)}
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
	t.Logf("%d seeds, %d clean; ability resolutions %v; cast %v; attacked %v; blocked %v; destroyed by %v", seeds, clean, tally.Abilities, tally.Cast, tally.Attacked, tally.Blocked, tally.Destroyed)
	for _, name := range []string{"Master of Arms", "Cromat", "Assault Formation"} {
		if tally.Abilities[name] == 0 {
			t.Errorf("no ability of %s resolved", name)
		}
	}
	for _, name := range []string{"Grakk the Pacifist", "Alms Beast", "Rampaging Cyclops"} {
		if tally.Cast[name] == 0 {
			t.Errorf("%s never cast", name)
		}
	}
	if tally.Destroyed["Cromat"] == 0 {
		t.Errorf("Cromat never destroyed a creature")
	}
	if tally.Attacked["Wall of Wood"] == 0 {
		t.Errorf("Assault Formation never let a Wall of Wood attack")
	}
	if tally.Attacked["Grakk the Pacifist"] != 0 || tally.Blocked["Grakk the Pacifist"] != 0 {
		t.Errorf("Grakk the Pacifist attacked %d times and blocked %d", tally.Attacked["Grakk the Pacifist"], tally.Blocked["Grakk the Pacifist"])
	}
}
