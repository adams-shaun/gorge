package oracle

import (
	"fmt"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The saddler-pairing lockstep (CR 702.171c): Giant
// Beaver's "put a +1/+1 counter on target creature that saddled it this
// turn" reads gorge's pairedWithSourceThisTurn, the per-turn list a Saddle
// payment folds through foldCrew. Counters from gorge's events, state and
// option lists:
//
//  1. a +1/+1 counter put on a creature paired with a Giant Beaver this
//     turn (the pool's only counter source is the Beaver's trigger),
//  2. a Beaver-trigger target ask (every option a Beaver's saddler) whose
//     options leave out an untapped non-Beaver creature of the asker that
//     did not saddle it,
//  3. a saddled Giant Beaver declared as an attacker with no saddler left
//     on the battlefield, so its trigger has no legal target and gorge
//     removes it (CR 603.3d).
var osDecks = [2][]cardset.Entry{
	{
		{Name: "Giant Beaver", Count: 6}, {Name: "Grizzly Bears", Count: 8}, {Name: "Llanowar Elves", Count: 6},
		{Name: "Hill Giant", Count: 3}, {Name: "Forest", Count: 17},
	},
	{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4}, {Name: "Raging Goblin", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 8},
	},
}

type osTally struct {
	evAt                     int
	seen                     map[state.ObjID]int32 // Beaver -> the turn its saddled attack was counted
	counters, asks, noTarget int
}

func osIsBeaver(o *state.Object) bool {
	return o != nil && o.Face() != nil && o.Face().Name == "Giant Beaver"
}

// osSaddlerOf reports the Giant Beaver o saddled (or crewed) this turn, 0
// for none: gorge's pairedWithSourceThisTurn read from the other end.
func osSaddlerOf(g *state.Game, o *state.Object) state.ObjID {
	if o == nil || o.CrewedTurn != g.Turn {
		return 0
	}
	for _, v := range o.CrewedVehicles {
		if osIsBeaver(g.Obj(v)) {
			return v
		}
	}
	return 0
}

func (p *osTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.CounterChange:
			if e.Amount > 0 && osSaddlerOf(g, g.Obj(e.Obj)) != 0 {
				p.counters++
			}
		}
	}
	// A saddled Beaver first seen attacking: gorge emits DeclareAttackers
	// before the declaration, so read state at the next decision (the
	// trigger's target ask, when a saddler is left to ask about).
	for i := range g.Objs {
		b := &g.Objs[i]
		if b.Zone != state.ZBattlefield || !osIsBeaver(b) || !b.IsAttacking || b.SaddledTurn != g.Turn || p.seen[b.ID] == g.Turn {
			continue
		}
		p.seen[b.ID] = g.Turn
		paired := false
		for k := range g.Objs {
			if o := &g.Objs[k]; o.Zone == state.ZBattlefield && pairedWith(g, o, b.ID) {
				paired = true
			}
		}
		if !paired {
			p.noTarget++
		}
	}
	p.evAt = len(evs)
	fr := ref.fr
	if fr == nil || fr.d == nil || fr.d.Kind != decision.KTarget || len(fr.d.Options) == 0 {
		return nil
	}
	var mount state.ObjID
	offered := map[state.ObjID]bool{}
	for _, o := range fr.d.Options {
		m := osSaddlerOf(g, g.Obj(o.Obj))
		if o.Obj == 0 || m == 0 || mount != 0 && m != mount {
			return nil // not a Beaver trigger's ask
		}
		mount = m
		offered[o.Obj] = true
	}
	for i := range g.Objs {
		o := &g.Objs[i]
		if o.Zone == state.ZBattlefield && o.Controller == fr.d.Player && o.EffectiveIsCreature() && !o.Tapped &&
			!osIsBeaver(o) && !offered[o.ID] && !pairedWith(g, o, mount) {
			p.asks++
			break
		}
	}
	return nil
}

func osBurn(o *state.Object) bool {
	return o != nil && o.Face() != nil && (o.Face().Name == "Shock" || o.Face().Name == "Lightning Bolt")
}

// pairedWith is gorge's pairedWithSourceThisTurn (engine/effects/filter.go).
func pairedWith(g *state.Game, o *state.Object, src state.ObjID) bool {
	if o.CrewedTurn != g.Turn {
		return false
	}
	for _, v := range o.CrewedVehicles {
		if v == src {
			return true
		}
	}
	return false
}

// osPolicy activates Saddle whenever it can and attacks with a saddled
// Giant Beaver; otherwise it casts (or plays a land) first, else random.
// With an opposing Beaver's saddler on the battlefield it casts burn first
// and aims it at the saddler, so some saddled Beavers attack with no
// legal target left; on even seeds it holds burn, and the mana to cast
// it, for one.
func osPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		g := c.G.E.G
		obj := func(r Ref) *state.Object { return g.Obj(state.ObjID(r) + 1) }
		foe := func(o *state.Object) bool {
			return o != nil && o.Zone == state.ZBattlefield && osSaddlerOf(g, o) != 0 && uint8(o.Controller) != c.D.Player
		}
		saddler := false
		for i := range g.Objs {
			saddler = saddler || foe(&g.Objs[i])
		}
		switch c.D.Kind {
		case DPriority:
			buf = buf[:0]
			hold := false
			for i := range g.Objs {
				if o := &g.Objs[i]; seed%2 == 0 && o.Zone == state.ZHand && uint8(o.Owner) == c.D.Player && osBurn(o) {
					hold = true // held, with its mana, for a saddler on even seeds
				}
			}
			for _, a := range c.D.Options {
				if a.Kind() == ActActivate && osIsBeaver(obj(a.Src())) {
					return a
				}
				if a.Kind() == ActCast && osBurn(obj(a.Src())) && saddler {
					return a
				}
				if k := a.Kind(); k == ActCast || k == ActPlayLand {
					buf = append(buf, a)
				}
			}
			if hold { // keep the burn's mana open: lands only
				j := 0
				for _, a := range buf {
					if a.Kind() == ActPlayLand {
						buf[j] = a
						j++
					}
				}
				buf = buf[:j]
			}
			if len(buf) > 0 {
				d := *c.D
				d.Options = buf
				return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
			for _, a := range c.D.Options {
				if a.Kind() == ActPass {
					return a // no floating mana tapped for nothing
				}
			}
		case DTarget:
			for _, a := range c.D.Options {
				if a.Tgt() < RefPlayer && foe(obj(a.Tgt())) {
					return a
				}
			}
		case DAttackers:
			for _, a := range c.D.Options {
				if o := obj(a.Src()); a.Kind() == ActAttack && osIsBeaver(o) && o.SaddledTurn == g.Turn {
					return a
				}
			}
		}
		return rnd.Pick(c)
	})
}

func TestOtjSaddlerLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("otjsaddle")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range osDecks {
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
	var tbl *kop.Table
	if tbl, _, _, err = set.Matchup(osDecks[0], osDecks[1]); err != nil {
		t.Fatal(err)
	}
	var p osTally
	games, stalls, caps, divs := 0, 0, 0, 0
	for seed := uint64(1); seed <= 256; seed++ {
		if v := os.Getenv("KOTJSADDLE_SEED"); v != "" && v != fmt.Sprint(seed) {
			continue
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"os-a", "os-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		p.evAt, p.seen = 0, map[state.ObjID]int32{}
		lead, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, lead, k, osPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: p.check})
		CloseSide(k)
		games++
		if res.Stall == "turns" || res.Stall == "decisions" {
			caps++ // a cap ended the game, not the kernel's capacity
		} else if res.Stall != "" {
			stalls++
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games (%d divergences, %d stalls, %d capped): saddler counters %d, trigger asks leaving out a non-saddler %d, no-target saddled attacks %d",
		games, divs, stalls, caps, p.counters, p.asks, p.noTarget)
	if os.Getenv("KOTJSADDLE_SEED") != "" {
		return
	}
	for _, c := range []struct {
		name   string
		n, min int
	}{{"saddler counters", p.counters, 10}, {"trigger asks leaving out a non-saddler", p.asks, 10},
		{"no-target saddled attacks", p.noTarget, 1}} {
		if c.n < c.min {
			t.Errorf("%d %s, want at least %d", c.n, c.name, c.min)
		}
	}
}
