package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A Fog beside "damage can't be prevented" (CR 615.12), decision by decision
// against gorge: Flaring Pain's Effect (damage can't be prevented this turn)
// makes a resolved Fog or Dawn Charm's Fog mode stop nothing, so the
// attackers' combat damage is dealt; The Mindskinner's prevent-and-mill body
// mills while the damage stands. The kernel once stalled (StallNoPrevent) on
// both; gorge's reading (fogUnpreventable, damageReplacementPreventsWithRider)
// is the spec, pinned by engine/rules TestFogUnderCantPreventDamageDealsCombatDamage.
var cantPreventFogDecks = [2][]cardset.Entry{
	{
		{Name: "Flaring Pain", Count: 4}, {Name: "Fog", Count: 3}, {Name: "Dawn Charm", Count: 3},
		{Name: "The Mindskinner", Count: 3}, {Name: "Forest Bear", Count: 6},
		{Name: "Island", Count: 9}, {Name: "Forest", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Plains", Count: 4},
	},
	{
		{Name: "Flaring Pain", Count: 4}, {Name: "Fog", Count: 3}, {Name: "Dawn Charm", Count: 3},
		{Name: "The Mindskinner", Count: 3}, {Name: "Forest Bear", Count: 6},
		{Name: "Island", Count: 9}, {Name: "Forest", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Plains", Count: 4},
	},
}

// cantPreventFogPolicy casts The Mindskinner, then Flaring Pain, a Fog, Dawn Charm, then
// anything else, plays lands, attacks with everything it can, and otherwise
// picks at random from the seed's own stream.
func cantPreventFogPolicy(m *Matchup, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	prefer := []string{"The Mindskinner", "Flaring Pain", "Fog", "Dawn Charm"}
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DPriority {
			for _, want := range prefer {
				for _, o := range p.D.Options {
					if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == want {
						return o
					}
				}
			}
			for _, o := range p.D.Options {
				if k := o.Kind(); k == ActPlayLand || k == ActCast {
					return o
				}
			}
		}
		var atk []Canon
		for _, o := range p.D.Options {
			if o.Kind() == ActAttack {
				atk = append(atk, o)
			}
		}
		if len(atk) > 0 {
			d := *p.D
			d.Options = atk
			return rnd.Pick(&PickCtx{D: &d, G: p.G, Seq: p.Seq})
		}
		return rnd.Pick(p)
	})
}

// cantPreventFogTally is what the gorge log of a game shows, per turn (a
// TurnChange event starts the next): a resolved Flaring Pain, a Fog's "all
// combat damage this turn is prevented" note, combat damage dealt to
// anything, The Mindskinner among the attackers, and a card milled.
type cantPreventFogTally struct {
	turn, evAt                       int
	fp, fog, dmg, mind, mill         map[int]bool
	games, fogDamage, mindskinnerHit int
}

func newCantPreventFogTally() *cantPreventFogTally {
	return &cantPreventFogTally{fp: map[int]bool{}, fog: map[int]bool{}, dmg: map[int]bool{},
		mind: map[int]bool{}, mill: map[int]bool{}}
}

// scan walks the events logged since the last call.
func (c *cantPreventFogTally) scan(ref *GorgeSide) {
	g, evs := ref.E.G, ref.E.L.Events
	for ; c.evAt < len(evs); c.evAt++ {
		e := &evs[c.evAt]
		switch {
		case e.Kind == events.TurnChange:
			c.turn++
		case e.Kind == events.MoveZone && e.From == state.ZStack && e.To == state.ZGraveyard && dpsNamed(g, e.Obj, "Flaring Pain"):
			c.fp[c.turn] = true
		case e.Kind == events.Note && strings.Contains(e.Text, "all combat damage this turn is prevented"):
			c.fog[c.turn] = true
		case e.Kind == events.Damage && e.Amount > 0:
			c.dmg[c.turn] = true
		case e.Kind == events.DeclareAttackers:
			for _, id := range e.IDs {
				if dpsNamed(g, id, "The Mindskinner") {
					c.mind[c.turn] = true
				}
			}
		case e.Kind == events.MoveZone && e.From == state.ZLibrary && e.To == state.ZGraveyard:
			c.mill[c.turn] = true
		}
	}
}

// end closes a game: the turns where a Fog resolved beside Flaring Pain and
// combat damage was still dealt, and where The Mindskinner attacked beside
// Flaring Pain with damage and a mill both in the turn.
func (c *cantPreventFogTally) end() {
	c.games++
	for t := range c.fp {
		if c.fog[t] && c.dmg[t] {
			c.fogDamage++
		}
		if c.mind[t] && c.dmg[t] && c.mill[t] {
			c.mindskinnerHit++
		}
	}
	c.turn, c.evAt = 0, 0
	clear(c.fp)
	clear(c.fog)
	clear(c.dmg)
	clear(c.mind)
	clear(c.mill)
}

// TestFogCantPreventLockstep plays 64 seeds on each option surface (autopay
// and full) of decks holding Flaring Pain, Fog, Dawn Charm and The
// Mindskinner: no divergence, no stall, and the gorge side must reach a Fog
// resolving beside Flaring Pain with combat damage dealt in that turn.
func TestFogCantPreventLockstep(t *testing.T) {
	t.Parallel()
	reg, set, gd := lockstepPool(t, "mbc", cantPreventFogDecks)
	tbl, _, _, err := set.Matchup(cantPreventFogDecks[0], cantPreventFogDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	tally := newCantPreventFogTally()
	for _, autopay := range []bool{true, false} {
		for seed := uint64(1); seed <= 64; seed++ {
			m, err := NewMatchup(reg, [2]string{"cpf-a", "cpf-b"}, gd, 1)
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{GorgeAutopaySurface: autopay}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, cantPreventFogPolicy(m, seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40,
				Check:  func(ref *GorgeSide, _ Side) *Divergence { tally.scan(ref); return nil },
				Finish: func(g *GorgeSide) { tally.scan(g); tally.end() }})
			CloseSide(k)
			if err := unmodelledStallError(t.Name(), &res); err != nil {
				t.Errorf("autopay=%v seed %d: %v", autopay, seed, err)
			}
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Errorf("autopay=%v seed %d: stalled: %s", autopay, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				t.Fatalf("autopay=%v seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", autopay, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games: %d turns with a Fog and combat damage beside Flaring Pain, %d Mindskinner hits with a mill", tally.games, tally.fogDamage, tally.mindskinnerHit)
	if tally.fogDamage == 0 {
		t.Fatal("no game had a Fog resolve beside Flaring Pain and combat damage dealt that turn")
	}
	if tally.mindskinnerHit == 0 {
		t.Fatal("no game had The Mindskinner attack, deal damage and mill in a Flaring Pain turn")
	}
}
