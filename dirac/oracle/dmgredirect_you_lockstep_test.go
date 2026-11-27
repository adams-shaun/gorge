package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/state"
)

// The redirect-to-controller lockstep: Vassal's Duty (the next 1 damage that
// would be dealt to target legendary creature you control this turn is dealt
// to you instead) and Sivvi's Valor (all damage that would be dealt to target
// creature this turn is dealt to you instead) against red burn aimed at seat
// 0's creatures. The expected answer at every decision is gorge's.
var kyouredirectDecks = [2][]cardset.Entry{
	{
		{Name: "Vassal's Duty", Count: 4}, {Name: "Sivvi's Valor", Count: 4},
		{Name: "Rofellos, Llanowar Emissary", Count: 4}, {Name: "Savannah Lions", Count: 3},
		{Name: "Serra Angel", Count: 3},
		{Name: "Plains", Count: 12}, {Name: "Forest", Count: 10},
	},
	{
		{Name: "Lightning Bolt", Count: 8}, {Name: "Shock", Count: 8}, {Name: "Hill Giant", Count: 4},
		{Name: "Raging Goblin", Count: 4}, {Name: "Child of Night", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 8},
	},
}

// TestDmgRedirectYouLowers: both cards lower to Ext DmgRedirect of their kind
// (Vassal's Duty YouNext, Sivvi's Valor YouAll), and the games against gorge
// diverge nowhere.
func TestDmgRedirectYouLowers(t *testing.T) {
	t.Parallel()
	set := cardsettest.Set(t, "youredirect")
	tbl, _, _, err := set.Matchup(kyouredirectDecks[0], kyouredirectDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for i := range tbl.Cards {
		c := &tbl.Cards[i]
		want := 0
		switch c.Name {
		case "Vassal's Duty":
			want = kop.DmgRedirectYouNext
		case "Sivvi's Valor":
			want = kop.DmgRedirectYouAll
		default:
			continue
		}
		for j := range c.Abilities {
			for _, in := range c.Abilities[j].Prog {
				if in.Ext() == kop.ExtDmgRedirect && int(in.C) == want {
					seen[c.Name] = true
				}
			}
		}
	}
	if !seen["Vassal's Duty"] || !seen["Sivvi's Valor"] {
		t.Fatalf("the table does not lower both redirects of their kind: %v", seen)
	}
	lockstepRunPolicy(t, "youredirect", kyouredirectDecks, 200, true, nil, nil,
		func(seed uint64) Policy { return &youRedirPolicy{rnd: castFirstPolicy(seed)} })
}

// youRedirPolicy scripts the redirect games: seat 1 aims its burn at seat 0's
// creatures, preferring the legendary one (Vassal's Duty's only legal
// target), and seat 0 answers a spell on the stack that names one of them
// with one Vassal's Duty activation and one Sivvi's Valor cast a turn,
// targeting the creature the spell aims at (the record must meet the very
// event). Seat 0 never attacks or blocks: a combat event to a remembered
// creature is the You kinds' declared limit (redirCombat stalls). Any other
// choice is the random cast-first policy's.
type youRedirPolicy struct {
	rnd        Policy
	turn       int32
	vassals    int // Vassal's Duty activations this turn
	sivvis     int // Sivvi's Valor casts this turn
	aim        state.ObjID
	hasAim     bool
	remembered map[state.ObjID]bool // creatures an answer redirected from this turn
}

// isLegendaryCreature reports whether o is seat 0's legendary creature:
// Vassal's Duty's only legal target.
func isLegendaryCreature(o *state.Object) bool {
	return o != nil && o.Face() != nil && o.Face().Name == "Rofellos, Llanowar Emissary"
}

// stackAim (dmgredirect_host_lockstep_test.go) is the object the topmost
// object-targeting spell of seat 1's names as its first target.

func (p *youRedirPolicy) Pick(c *PickCtx) Canon {
	d := c.D
	g := c.G.E.G
	switch d.Kind {
	case DMulligan:
		return pickKind(c, ActKeep)
	case DAttackers:
		if d.Player == 0 {
			return pickKind(c, ActDone) // never attack: combat damage to a remembered creature is not modelled
		}
		if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActAttack }); ok {
			return a
		}
		return pickKind(c, ActDone)
	case DBlockers:
		return pickKind(c, ActDone) // neither side blocks: likewise
	case DTarget:
		if d.Player == 0 {
			// the answer's target: the creature the answered spell aims at
			if p.hasAim {
				if a, ok := findAction(c, func(a Canon) bool {
					return a.Kind() == ActTarget && a.Tgt() == c.G.InstRef(p.aim)
				}); ok {
					return a
				}
			}
			break
		}
		// seat 1 aims its burn at seat 0's creatures, preferring the
		// legendary one so Vassal's Duty meets the script
		for _, id := range g.Zone(state.ZBattlefield, 0) {
			if !isLegendaryCreature(g.Obj(id)) {
				continue
			}
			if a, ok := findAction(c, func(a Canon) bool {
				return a.Kind() == ActTarget && a.Tgt() == c.G.InstRef(id)
			}); ok {
				return a
			}
		}
		for _, id := range g.Zone(state.ZBattlefield, 0) {
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
			p.turn, p.vassals, p.sivvis = int32(g.Turn), 0, 0
			p.remembered = map[state.ObjID]bool{}
		}
		if d.Player == 0 {
			p.aim, p.hasAim = stackAim(g)
			if p.hasAim && !p.remembered[p.aim] {
				if isLegendaryCreature(g.Obj(p.aim)) && p.vassals < 1 {
					// answer with the redirect (the activation is the
					// script's alone: it is filtered out of the random path)
					if a, ok := findAction(c, func(a Canon) bool {
						if a.Kind() != ActActivate {
							return false
						}
						ob := g.Obj(c.G.id.obj[a.Src()])
						return ob != nil && ob.Face() != nil && ob.Face().Name == "Vassal's Duty"
					}); ok {
						p.vassals++
						p.remembered[p.aim] = true
						return a
					}
				}
				if p.sivvis < 1 {
					if a, ok := findAction(c, func(a Canon) bool {
						return a.Kind() == ActCast && refCardName(c.G, a.Src()) == "Sivvi's Valor"
					}); ok {
						p.sivvis++
						p.remembered[p.aim] = true
						return a
					}
				}
			}
		}
	}
	if d.Kind == DPriority {
		// the scripted activations and Sivvi's casts are the script's alone:
		// an unscripted answer could register a second record on a creature
		// this turn (gorge's order ask: the kernel stalls)
		var buf []Canon
		for _, a := range d.Options {
			if a.Kind() == ActActivate || a.Kind() == ActCast && refCardName(c.G, a.Src()) == "Sivvi's Valor" {
				continue
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

// TestDmgRedirectYouLockstepMutation proves the records fire: with the
// redirects dropped (the Effects still resolving), some seed diverges: the
// aimed burn must be landing on the records.
func TestDmgRedirectYouLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoDmgRedirect = true
	defer func() { dirac.DebugNoDmgRedirect = false }()
	if lockstepRunPolicy(t, "youredirect", kyouredirectDecks, 100, false, nil, nil,
		func(seed uint64) Policy { return &youRedirPolicy{rnd: castFirstPolicy(seed)} }) == 0 {
		t.Fatal("no seed diverged with the redirects dropped: their damage is never redirected")
	}
}
