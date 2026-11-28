package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Two Copper Myr are one class of the payment planner; Assault Suit on the
// newer one makes it no Ashnod's Altar victim. With four Mountains, Enormous
// Baloth's {6}{G} is paid only by tapping the equipped Myr and sacrificing the
// older one -- not the member choice the planner's witness makes first (the
// older Myr tapped), which leaves the altar no victim (CR 601.2g-h). Both
// engines must offer the cast and pay it with the same two creatures.
var altarMemberDeck = [2][]cardset.Entry{
	{
		{Name: "Ashnod's Altar", Count: 6}, {Name: "Copper Myr", Count: 9}, {Name: "Assault Suit", Count: 5},
		{Name: "Enormous Baloth", Count: 5}, {Name: "Mountain", Count: 15},
	},
	{{Name: "Island", Count: 40}},
}

const altarMemberLands = 4

type altarMemberScenario struct {
	ref  *GorgeSide
	evAt int
	// older and equipped are the two Myr of the last Baloth cast
	older, equipped state.ObjID
	casts, paid     int
}

// altarMemberBoard is seat 0's battlefield as the scenario reads it.
type altarMemberBoard struct {
	lands, altars, suits int
	suit                 *state.Object
	myr                  []state.ObjID // ascending id
}

func altarMemberRead(g *state.Game) altarMemberBoard {
	var b altarMemberBoard
	for _, id := range g.Zone(state.ZBattlefield, 0) {
		switch chName(g, id) {
		case "Mountain":
			b.lands++
		case "Ashnod's Altar":
			b.altars++
		case "Assault Suit":
			b.suits++
			b.suit = g.Obj(id)
		case "Copper Myr":
			b.myr = append(b.myr, id)
		}
	}
	if len(b.myr) == 2 && b.myr[0] > b.myr[1] {
		b.myr[0], b.myr[1] = b.myr[1], b.myr[0]
	}
	return b
}

func (p *altarMemberScenario) Pick(c *PickCtx) Canon {
	g := c.G.E.G
	// decline the Suit's upkeep offer of the equipped creature
	if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActNo }); ok && c.D.Kind != DPriority {
		return a
	}
	if c.D.Kind == DTarget && c.D.Player == 0 {
		if b := altarMemberRead(g); len(b.myr) == 2 {
			ref := c.G.InstRef(b.myr[1])
			if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActTarget && a.Tgt() == ref }); ok {
				return a
			}
		}
	}
	return mulliganDonePick(p.priority, c)
}

func (p *altarMemberScenario) priority(c *PickCtx) Canon {
	if c.D.Player != 0 {
		return pickKind(c, ActPass)
	}
	g := c.G.E.G
	b := altarMemberRead(g)
	ready := b.altars > 0 && b.lands == altarMemberLands && len(b.myr) == 2 && b.suit != nil && b.suit.AttachedTo == b.myr[1]
	if ready && !g.Obj(b.myr[0]).Tapped && !g.Obj(b.myr[1]).Tapped {
		if a, ok := reproHandCastAction(c, g, 0, "Enormous Baloth"); ok {
			p.casts++
			p.older, p.equipped = b.myr[0], b.myr[1]
			return a
		}
	}
	if b.lands < altarMemberLands {
		if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActPlayLand }); ok {
			return a
		}
	}
	for _, w := range []struct {
		name string
		want bool
	}{{"Copper Myr", len(b.myr) < 2}, {"Assault Suit", b.suits == 0},
		// the Altar last, so no board has it beside a single or unequipped pair
		{"Ashnod's Altar", b.altars == 0 && len(b.myr) == 2 && b.suit != nil && b.suit.AttachedTo == b.myr[1]}} {
		if !w.want || len(g.Stack) != 0 {
			continue
		}
		if a, ok := reproHandCastAction(c, g, 0, w.name); ok {
			return a
		}
	}
	if len(b.myr) == 2 && b.suit != nil && b.suit.AttachedTo != b.myr[1] && len(g.Stack) == 0 {
		suit := c.G.InstRef(b.suit.ID)
		if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActActivate && a.Src() == suit }); ok {
			return a
		}
	}
	return pickKind(c, ActPass)
}

// check counts the casts paid by sacrificing the older Myr beside the
// equipped one, tapped, in gorge's event log.
func (p *altarMemberScenario) check(ref *GorgeSide, _ Side) *Divergence {
	evs := ref.E.L.Events
	if p.ref != ref || len(evs) < p.evAt {
		p.ref, p.evAt = ref, 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		if e.Kind == events.MoveZone && events.IsSacrifice(*e) && p.older != 0 && e.Obj == p.older {
			if o := ref.E.G.Obj(p.equipped); o.Zone == state.ZBattlefield && o.Tapped {
				p.paid++
			}
			p.older = 0
		}
	}
	p.evAt = len(evs)
	return nil
}

func TestAltarMemberChoiceLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	var names [2][]string
	for i, d := range altarMemberDeck {
		for _, e := range d {
			names[i] = append(names[i], e.Name)
		}
	}
	set := cardset.Raw(printedDelveSet(t, names))
	decks, tbl := mccoyBoard(t, reg, set, altarMemberDeck)
	casts, paid := 0, 0
	// seeds 1..10 reach the board on both surfaces. Later seeds meet another
	// altar divergence (a creature that both taps and is sacrificed) first.
	for seed := uint64(1); seed <= 10; seed++ {
		m, err := NewMatchup(reg, [2]string{"amc-a", "amc-b"}, decks, 0)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		sc := &altarMemberScenario{}
		res := Lockstep(m, seed, k, sc, RunConfig{Config: cfg, MaxDecisions: 5000, MaxTurns: 40, Check: sc.check})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		casts += sc.casts
		paid += sc.paid
	}
	t.Logf("Baloth casts on the one-victim board %d, paid by the equipped Myr's mana and the older Myr's sacrifice %d", casts, paid)
	if casts == 0 || paid == 0 {
		t.Fatalf("casts %d, paid %d: the member choice was never exercised", casts, paid)
	}
}
