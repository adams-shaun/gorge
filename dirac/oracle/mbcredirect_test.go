package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/state"
)

// The damage-redirect lockstep: Nomads en-Kor and Warrior en-Kor (the next 1
// damage to the host this turn is dealt to a creature you control instead) and
// Gideon's Sacrifice (all damage to you and your permanents this turn is dealt
// to the chosen permanent instead), against red burn, creatures and a
// lifelinker. The expected answer at every decision is gorge's.
var kmbcredirectDecks = [2][]cardset.Entry{
	{
		{Name: "Nomads en-Kor", Count: 6}, {Name: "Warrior en-Kor", Count: 6}, {Name: "Gideon's Sacrifice", Count: 4},
		{Name: "Zealous Inquisitor", Count: 3}, {Name: "Savannah Lions", Count: 3}, {Name: "Serra Angel", Count: 2},
		{Name: "Child of Night", Count: 2}, {Name: "Plains", Count: 14},
	},
	{
		{Name: "Lightning Bolt", Count: 6}, {Name: "Shock", Count: 6}, {Name: "Hill Giant", Count: 3},
		{Name: "Raging Goblin", Count: 4}, {Name: "Grizzly Bears", Count: 3}, {Name: "Child of Night", Count: 3},
		{Name: "Mountain", Count: 12}, {Name: "Forest", Count: 3},
	},
}

// TestMBCDamageRedirectLowers: the three cards lower to Ext DmgRedirect (the
// two kinds), and the games against gorge diverge nowhere.
func TestMBCDamageRedirectLowers(t *testing.T) {
	t.Parallel()
	corpusDirOrSkip(t) // a fresh clone has no corpus: skip, as every lockstep sibling does
	set, err := cardset.RawByName("mbcredirect")
	if err != nil {
		t.Fatal(err)
	}
	tbl, _, _, err := set.Matchup(kmbcredirectDecks[0], kmbcredirectDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"Nomads en-Kor": kop.DmgRedirectNext, "Warrior en-Kor": kop.DmgRedirectNext, "Zealous Inquisitor": kop.DmgRedirectNext, "Gideon's Sacrifice": kop.DmgRedirectScope}
	for i := range tbl.Cards {
		c := &tbl.Cards[i]
		kind, ok := want[c.Name]
		if !ok {
			continue
		}
		found := false
		for j := range c.Abilities {
			for _, in := range c.Abilities[j].Prog {
				if in.Ext() == kop.ExtDmgRedirect && int(in.C) == kind {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("%s does not lower to Ext DmgRedirect kind %d", c.Name, kind)
		}
		delete(want, c.Name)
	}
	for n := range want {
		t.Errorf("%s is not in the table", n)
	}
	lockstepSet(t, "mbcredirect", kmbcredirectDecks, 500, "KMBCREDIRECT_SEEDS")
	// the scripted halves: the redirects answer burn aimed at seat 0's creatures
	// (noncombat damage), and answer attackers at the declare-blockers step
	// (combat damage per assignment)
	lockstepRunPolicy(t, "mbcredirect", kmbcredirectBurnDecks, 300, true, nil, nil, func(seed uint64) Policy { return &redirPolicy{rnd: castFirstPolicy(seed), burn: true} })
	lockstepRunPolicy(t, "mbcredirect", kmbcredirectCombatDecks, 300, true, nil, nil, func(seed uint64) Policy { return &redirPolicy{rnd: castFirstPolicy(seed)} })
}

// kmbcredirectCombatDecks is the combat half: bodies on both sides, so the
// redirect creatures block, are blocked and attack into blockers.
var kmbcredirectCombatDecks = [2][]cardset.Entry{
	{
		{Name: "Nomads en-Kor", Count: 10}, {Name: "Warrior en-Kor", Count: 9}, {Name: "Gideon's Sacrifice", Count: 4},
		{Name: "Plains", Count: 17},
	},
	{
		{Name: "Hill Giant", Count: 5}, {Name: "Raging Goblin", Count: 6}, {Name: "Grizzly Bears", Count: 5},
		{Name: "Child of Night", Count: 4}, {Name: "Serra Angel", Count: 2}, {Name: "Mountain", Count: 14}, {Name: "Forest", Count: 4},
	},
}

// kmbcredirectBurnDecks is the noncombat half: the redirect cards and a
// creature to carry them against burn aimed at the creatures.
var kmbcredirectBurnDecks = [2][]cardset.Entry{
	{
		{Name: "Nomads en-Kor", Count: 8}, {Name: "Warrior en-Kor", Count: 8}, {Name: "Gideon's Sacrifice", Count: 5},
		{Name: "Savannah Lions", Count: 3}, {Name: "Plains", Count: 16},
	},
	{
		{Name: "Lightning Bolt", Count: 10}, {Name: "Shock", Count: 10}, {Name: "Mountain", Count: 20},
	},
}

// redirPolicy scripts the redirect games: seat 1 aims its burn at seat 0's
// creatures (the en-Kor creatures first), and seat 0 answers a spell of
// seat 1's on the stack, or (outside the burn half) an attack at the
// declare-blockers step, with at most one activation or cast a turn.
// Any other choice is the random cast-first policy's.
type redirPolicy struct {
	rnd  Policy
	burn bool
	acts int // activations and casts this turn
	turn int32
}

// isGideon reports whether option a casts Gideon's Sacrifice: the script
// casts it only in answer to a spell, so that the chosen permanent can differ
// from the one the burn aims at.
func (p *redirPolicy) isGideon(c *PickCtx, a Canon) bool {
	if a.Kind() != ActCast {
		return false
	}
	ob := c.G.E.G.Obj(c.G.id.obj[a.Src()])
	return ob != nil && ob.Face() != nil && ob.Face().Name == "Gideon's Sacrifice"
}

func (p *redirPolicy) Pick(c *PickCtx) Canon {
	d := c.D
	g := c.G.E.G
	switch d.Kind {
	case DMulligan:
		return pickKind(c, ActKeep)
	case DAttackers, DBlockers:
		if p.burn {
			return pickKind(c, ActDone)
		}
		// combat is the point of the other half: every declaration made is an attack or a block
		for _, k := range []CanonKind{ActAttack, ActBlock} {
			if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == k }); ok {
				return a
			}
		}
		return pickKind(c, ActDone)
	case DChoose:
		// Gideon's Sacrifice's chosen permanent: not always the one the burn aims at
		if g.Turn%2 == 1 && len(d.Options) > 1 {
			return d.Options[len(d.Options)-1]
		}
	case DTarget:
		if d.Player != 1 {
			break
		}
		best, bestRank := Canon(0), 0
		for _, id := range g.Zone(state.ZBattlefield, 0) {
			o := g.Obj(id)
			if o == nil || o.Face() == nil || !o.EffectiveIsCreature() {
				continue
			}
			rank := 1
			if strings.Contains(o.Face().Name, "en-Kor") {
				rank = 2
			}
			ref := c.G.InstRef(id)
			if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActTarget && a.Tgt() == ref }); ok && rank > bestRank {
				best, bestRank = a, rank
			}
		}
		if bestRank > 0 {
			return best
		}
	case DPriority:
		if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActPlayLand }); ok {
			return a
		}
		if int32(g.Turn) != p.turn {
			p.turn, p.acts = int32(g.Turn), 0
		}
		answering := len(g.Stack) > 0 && d.Player == 0
		if answering {
			if top := g.Obj(g.Stack[len(g.Stack)-1]); top == nil || top.Controller != 1 {
				answering = false
			}
		}
		if d.Player == 0 && !p.burn && len(g.Stack) == 0 && g.Step == state.StepDeclareBlockers {
			answering = true
		}
		if answering && p.acts < 1 {
			// Gideon's Sacrifice on odd turns, else an en-Kor activation
			first, second := ActActivate, ActCast
			if g.Turn%2 == 1 {
				first, second = ActCast, ActActivate
			}
			for _, k := range []CanonKind{first, second} {
				// a source in combat first: the redirect meets combat damage
				if a, ok := findAction(c, func(a Canon) bool {
					if a.Kind() != k {
						return false
					}
					ob := c.G.E.G.Obj(c.G.id.obj[a.Src()])
					return k != ActActivate || ob != nil && (ob.IsAttacking || ob.IsBlocking)
				}); ok {
					p.acts++
					return a
				}
				if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == k }); ok {
					p.acts++
					return a
				}
			}
		}
		if p.burn {
			if d.Player == 1 && len(g.Stack) == 0 {
				if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActCast }); ok {
					return a
				}
			}
			if d.Player == 0 && len(g.Stack) == 0 {
				if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActCast && !p.isGideon(c, a) }); ok {
					return a
				}
			}
			return pickKind(c, ActPass)
		}
	}
	if d.Kind == DPriority {
		// the free activations are the script's alone: a random policy would
		// stack them until the kernel's stack is full
		var buf []Canon
		for _, a := range d.Options {
			if a.Kind() != ActActivate && !p.isGideon(c, a) {
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

// TestMBCDamageRedirectMutation proves the records redirect: with the damage
// left on its original recipient (the Effect still resolving), some seed of
// each half diverges: the noncombat burn, and the combat damage.
func TestMBCDamageRedirectMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoDmgRedirect = true
	defer func() { dirac.DebugNoDmgRedirect = false }()
	for _, h := range []struct {
		name  string
		decks [2][]cardset.Entry
		burn  bool
	}{{"burn", kmbcredirectBurnDecks, true}, {"combat", kmbcredirectCombatDecks, false}} {
		if lockstepRunPolicy(t, "mbcredirect", h.decks, 100, false, nil, nil, func(seed uint64) Policy { return &redirPolicy{rnd: castFirstPolicy(seed), burn: h.burn} }) == 0 {
			t.Errorf("no seed of the %s half diverged with the redirects dropped: its damage is never redirected", h.name)
		}
	}
}
