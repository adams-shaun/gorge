package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/state"
)

// The unbounded shield with a counter body lockstep: Brace for Impact
// (prevent all damage that would be dealt to target multicolored creature
// this turn; put that many +1/+1 counters on it — the PreventionEffect$ True
// line, so its rider rides the shield) and Gatta and Luzzu (enters: choose a
// creature you control; if damage would be dealt to it this turn, prevent it
// and put that many +1/+1 counters on it — the Prevent$ True line, whose body
// gorge never reaches), against the red burn and attackers of
// kprevnextDecks. The gold creatures are the only legal Brace targets, so the
// deck carries them; the expected answer at every decision is gorge's.
var pnxctrDecks = [2][]cardset.Entry{
	{
		{Name: "Brace for Impact", Count: 6}, {Name: "Gatta and Luzzu", Count: 4},
		{Name: "Boros Recruit", Count: 4}, {Name: "Serra Avenger", Count: 3},
		{Name: "Savannah Lions", Count: 3}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Plains", Count: 12}, {Name: "Mountain", Count: 6},
	},
	kprevnextDecks[1],
}

// isPnxCtrShielded reports whether o is one of seat 0's creatures: the only
// legal targets of the two shields and the burn's preferred aim.
func isPnxCtrShielded(o *state.Object) bool {
	return o != nil && o.Face() != nil && o.Face().IsCreature() && o.Controller == 0
}

// TestPrevNextCtrRiderLowers: both carriers lower to Ext PreventNext — Brace
// as the unbounded shield with the counter rider
// (kop.PrevNextAll|kop.PrevNextRiderCounter), Gatta as the plain unbounded
// shield (kop.PrevNextAll, gorge dropping its Prevent$ True body) — and the
// games against gorge diverge nowhere.
func TestPrevNextCtrRiderLowers(t *testing.T) {
	t.Parallel()
	set := cardsettest.Set(t, "pnxctrrider")
	tbl, _, _, err := set.Matchup(pnxctrDecks[0], pnxctrDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	// Brace's PreventionEffect$ True line runs its body (gorge's
	// damageReplacementPreventsWithRider); Gatta's Prevent$ True line
	// terminates the event before the body runs, so it lowers to the plain
	// unbounded shield.
	want := map[string]int16{
		"Brace for Impact": int16(kop.PrevNextAll | kop.PrevNextRiderCounter),
		"Gatta and Luzzu":  int16(kop.PrevNextAll),
	}
	seen := map[string]bool{}
	for i := range tbl.Cards {
		c := &tbl.Cards[i]
		if _, ok := want[c.Name]; !ok {
			continue
		}
		for j := range c.Abilities {
			for _, in := range c.Abilities[j].Prog {
				if in.Ext() == kop.ExtPreventNext && in.C == want[c.Name] {
					seen[c.Name] = true
				}
			}
		}
	}
	for n, c := range want {
		if !seen[n] {
			t.Fatalf("%s does not lower to Ext PreventNext with C %d", n, c)
		}
	}
	lockstepRunPolicy(t, "pnxctrrider", pnxctrDecks, 200, true, nil, nil,
		func(seed uint64) Policy { return &pnxCtrRiderPolicy{rnd: castFirstPolicy(seed)} })
}

// pnxCtrRiderPolicy scripts the shield games: seat 1 aims its burn at seat 0's
// creatures, and seat 0 answers a spell on the stack that names one of them
// with a Brace for Impact cast targeting that very creature (the record must
// meet the event), and otherwise casts its shields and attacks. Gatta and
// Luzzu's enters target is the creature the answered spell aims at when legal.
// Attacks and blocks are all made, so combat damage meets the record too. Any
// other choice is the random cast-first policy's.
type pnxCtrRiderPolicy struct {
	rnd    Policy
	acts   int // Brace casts this turn
	turn   int32
	aim    state.ObjID
	hasAim bool
}

// pnxCtrStackAim is the object the topmost object-targeting spell of seat 1's
// names as its first target (the burn's creature, once its target is chosen),
// reporting false when the stack holds none.
func pnxCtrStackAim(g *state.Game) (state.ObjID, bool) {
	for i := len(g.Stack) - 1; i >= 0; i-- {
		top := g.Obj(g.Stack[i])
		if top == nil || top.Controller != 1 || len(top.Targets) == 0 || top.Targets[0].IsPlayer {
			continue
		}
		return top.Targets[0].Obj, true
	}
	return 0, false
}

func (p *pnxCtrRiderPolicy) Pick(c *PickCtx) Canon {
	d := c.D
	g := c.G.E.G
	switch d.Kind {
	case DMulligan:
		return pickKind(c, ActKeep)
	case DAttackers, DBlockers:
		// every declaration made is an attack or a block: combat damage meets
		// the live record too
		for _, k := range []CanonKind{ActAttack, ActBlock} {
			if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == k }); ok {
				return a
			}
		}
		return pickKind(c, ActDone)
	case DTarget:
		if d.Player == 0 {
			// the shield's target: the creature the answered spell aims at,
			// else any of seat 0's creatures
			if p.hasAim {
				if a, ok := findAction(c, func(a Canon) bool {
					return a.Kind() == ActTarget && a.Tgt() == c.G.InstRef(p.aim)
				}); ok {
					return a
				}
			}
			for _, id := range g.Zone(state.ZBattlefield, 0) {
				if !isPnxCtrShielded(g.Obj(id)) {
					continue
				}
				if a, ok := findAction(c, func(a Canon) bool {
					return a.Kind() == ActTarget && a.Tgt() == c.G.InstRef(id)
				}); ok {
					return a
				}
			}
			break
		}
		// seat 1 aims its burn at seat 0's creatures
		for _, id := range g.Zone(state.ZBattlefield, 0) {
			if !isPnxCtrShielded(g.Obj(id)) {
				continue
			}
			if a, ok := findAction(c, func(a Canon) bool {
				return a.Kind() == ActTarget && a.Tgt() == c.G.InstRef(id)
			}); ok {
				return a
			}
		}
	case DPriority:
		if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActPlayLand }); ok {
			return a
		}
		if int32(g.Turn) != p.turn {
			p.turn, p.acts = int32(g.Turn), 0
		}
		if d.Player == 0 {
			p.aim, p.hasAim = pnxCtrStackAim(g)
			if p.acts < 1 && len(g.Stack) > 0 {
				// answer the aimed spell with the shield (the cast is the
				// script's alone: it is filtered out of the random path)
				if a, ok := findAction(c, func(a Canon) bool {
					if a.Kind() != ActCast {
						return false
					}
					ob := g.Obj(c.G.id.obj[a.Src()])
					return ob != nil && ob.Face() != nil && ob.Face().Name == "Brace for Impact"
				}); ok {
					p.acts++
					return a
				}
			}
		}
	}
	if d.Kind == DPriority && d.Player == 0 {
		// the scripted Brace cast is the script's alone, so the random path
		// never spends it on its own
		var buf []Canon
		for _, a := range d.Options {
			if a.Kind() == ActCast {
				if ob := g.Obj(c.G.id.obj[a.Src()]); ob != nil && ob.Face() != nil && ob.Face().Name == "Brace for Impact" {
					continue
				}
			}
			buf = append(buf, a)
		}
		if len(buf) > 0 && len(buf) < len(d.Options) {
			d2 := *d
			d2.Options = buf
			return p.rnd.Pick(&PickCtx{D: &d2, G: c.G, Seq: c.Seq})
		}
	}
	return p.rnd.Pick(c)
}

// TestPrevNextCtrRiderLockstepMutation proves the rider fires: with the rider
// dropped (the shields still preventing everything), some seed diverges: the
// aimed burn and the combat damage must be landing on the record.
func TestPrevNextCtrRiderLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoPrevRider = true
	defer func() { dirac.DebugNoPrevRider = false }()
	if lockstepRunPolicy(t, "pnxctrrider", pnxctrDecks, 100, false, nil, nil,
		func(seed uint64) Policy { return &pnxCtrRiderPolicy{rnd: castFirstPolicy(seed)} }) == 0 {
		t.Fatal("no seed diverged with the rider dropped: its counters are never put")
	}
}
