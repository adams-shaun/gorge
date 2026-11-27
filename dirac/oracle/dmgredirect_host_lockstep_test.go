package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/state"
)

// The redirect-to-host lockstep: Daughter of Autumn (the next 1 damage that
// would be dealt to a target white creature this turn is dealt to CARDNAME
// instead, the en-Kor redirect with its pair swapped: the ability's target is
// the damaged host and the resolving permanent the recipient), against red
// burn aimed at the white creatures and attackers into them. The expected
// answer at every decision is gorge's.
var kdoaredirectDecks = [2][]cardset.Entry{
	{
		{Name: "Daughter of Autumn", Count: 8}, {Name: "Savannah Lions", Count: 4}, {Name: "Serra Angel", Count: 3},
		{Name: "Plains", Count: 12}, {Name: "Forest", Count: 13},
	},
	{
		{Name: "Lightning Bolt", Count: 8}, {Name: "Shock", Count: 8}, {Name: "Hill Giant", Count: 4},
		{Name: "Raging Goblin", Count: 4}, {Name: "Child of Night", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 8},
	},
}

// TestDmgRedirectHostLowers: Daughter of Autumn lowers to Ext DmgRedirect of
// the DmgRedirectHost kind, and the games against gorge diverge nowhere.
func TestDmgRedirectHostLowers(t *testing.T) {
	t.Parallel()
	set := cardsettest.Set(t, "doaredirect")
	tbl, _, _, err := set.Matchup(kdoaredirectDecks[0], kdoaredirectDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range tbl.Cards {
		c := &tbl.Cards[i]
		if c.Name != "Daughter of Autumn" {
			continue
		}
		for j := range c.Abilities {
			for _, in := range c.Abilities[j].Prog {
				if in.Ext() == kop.ExtDmgRedirect && int(in.C) == kop.DmgRedirectHost {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatal("Daughter of Autumn does not lower to Ext DmgRedirect kind DmgRedirectHost")
	}
	lockstepRunPolicy(t, "doaredirect", kdoaredirectDecks, 200, true, nil, nil,
		func(seed uint64) Policy { return &hostRedirPolicy{rnd: castFirstPolicy(seed)} })
}

// hostRedirPolicy scripts the redirect games: seat 1 aims its burn at seat 0's
// white creatures, and seat 0 answers a spell on the stack that names one of
// them with one Daughter of Autumn activation a turn, targeting the creature
// the spell aims at (the record must meet the very event). Attacks and blocks
// are all made, so combat damage meets the record too. Any other choice is
// the random cast-first policy's.
type hostRedirPolicy struct {
	rnd    Policy
	acts   int // Daughter of Autumn activations this turn
	turn   int32
	aim    state.ObjID // the creature the answered spell aims at
	hasAim bool
}

// isHostCreature reports whether o is one of seat 0's white creatures: the
// only legal targets of the activation and the burn's preferred aim.
func isHostCreature(o *state.Object) bool {
	return o != nil && o.Face() != nil && (o.Face().Name == "Savannah Lions" || o.Face().Name == "Serra Angel")
}

// stackAim is the object the topmost object-targeting spell of seat 1's names
// as its first target (the burn's creature, once its target is chosen),
// reporting false when the stack holds none.
func stackAim(g *state.Game) (state.ObjID, bool) {
	for i := len(g.Stack) - 1; i >= 0; i-- {
		top := g.Obj(g.Stack[i])
		if top == nil || top.Controller != 1 || len(top.Targets) == 0 || top.Targets[0].IsPlayer {
			continue
		}
		return top.Targets[0].Obj, true
	}
	return 0, false
}

func (p *hostRedirPolicy) Pick(c *PickCtx) Canon {
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
			// the activation's target: the creature the answered spell aims at,
			// else any white creature
			if p.hasAim {
				if a, ok := findAction(c, func(a Canon) bool {
					return a.Kind() == ActTarget && a.Tgt() == c.G.InstRef(p.aim)
				}); ok {
					return a
				}
			}
			for _, id := range g.Zone(state.ZBattlefield, 0) {
				if !isHostCreature(g.Obj(id)) {
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
		// seat 1 aims its burn at seat 0's white creatures
		for _, id := range g.Zone(state.ZBattlefield, 0) {
			if !isHostCreature(g.Obj(id)) {
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
			p.aim, p.hasAim = stackAim(g)
			if p.acts < 1 && len(g.Stack) > 0 {
				// answer the aimed spell with the redirect (the activation is
				// the script's alone: it is filtered out of the random path)
				if a, ok := findAction(c, func(a Canon) bool {
					if a.Kind() != ActActivate {
						return false
					}
					ob := g.Obj(c.G.id.obj[a.Src()])
					return ob != nil && ob.Face() != nil && ob.Face().Name == "Daughter of Autumn"
				}); ok {
					p.acts++
					return a
				}
			}
		}
	}
	if d.Kind == DPriority {
		// the free activations (the Channel goblin's) are the random policy's
		// to never see: the stack-filling risk of the en-Kor games
		var buf []Canon
		for _, a := range d.Options {
			if a.Kind() != ActActivate {
				buf = append(buf, a)
			}
		}
		if len(buf) > 0 && len(buf) < len(d.Options) {
			d2 := *d
			d2.Options = buf
			return p.rnd.Pick(&PickCtx{D: &d2, G: c.G, Seq: c.Seq})
		}
	}
	return p.rnd.Pick(c)
}

// TestDmgRedirectHostLockstepMutation proves the record fires: with the
// redirects dropped (the Effect still resolving), some seed diverges: the
// aimed burn and the combat damage must be landing on the record.
func TestDmgRedirectHostLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoDmgRedirect = true
	defer func() { dirac.DebugNoDmgRedirect = false }()
	if lockstepRunPolicy(t, "doaredirect", kdoaredirectDecks, 100, false, nil, nil,
		func(seed uint64) Policy { return &hostRedirPolicy{rnd: castFirstPolicy(seed)} }) == 0 {
		t.Fatal("no seed diverged with the redirects dropped: their damage is never redirected")
	}
}
