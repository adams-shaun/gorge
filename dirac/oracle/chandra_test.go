package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Chandra, Hope's Beacon lockstep: [+1] exiles the top five and
// grants, until the end of your next turn, one cast of an instant or sorcery
// from among them (Effect MayPlay, MayPlayLimit 1, Affected$ Instant.IsRemembered,
// Sorcery.IsRemembered); [-X] deals X damage to each of up to two targets
// (SubCounter<X/LOYALTY>: X announced up to the loyalty); the trigger copies
// the first instant or sorcery cast each turn. Mountain-heavy mirror decks
// with burn and a sorcery, so the exiled spells are castable and copied.
// The expected answer at every decision is gorge's.
var kchandraMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Chandra, Hope's Beacon", Count: 8}, {Name: "Shock", Count: 6}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Divination", Count: 4}, {Name: "Grizzly Bears", Count: 2}, {Name: "Mountain", Count: 16},
	},
	{
		{Name: "Chandra, Hope's Beacon", Count: 6}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 12},
		{Name: "Island", Count: 6},
	},
}, {
	{
		{Name: "Chandra, Hope's Beacon", Count: 10}, {Name: "Shock", Count: 6}, {Name: "Divination", Count: 6},
		{Name: "Mountain", Count: 18},
	},
	{
		{Name: "Chandra, Hope's Beacon", Count: 10}, {Name: "Shock", Count: 6}, {Name: "Divination", Count: 6},
		{Name: "Mountain", Count: 18},
	},
}, {
	// Deb Thomas's [-X] pumps Employees and Dogs by the announced X
	{
		{Name: "Deb Thomas", Count: 10}, {Name: "Grizzly Bears", Count: 6}, {Name: "Shock", Count: 4},
		{Name: "Mountain", Count: 20},
	},
	{
		{Name: "Deb Thomas", Count: 10}, {Name: "Hill Giant", Count: 6}, {Name: "Shock", Count: 4},
		{Name: "Mountain", Count: 20},
	},
}}

// kchandraTally counts, on gorge's side, the [-X] announcements (the deck has
// no other X), the spells cast from exile, and games.
type kchandraTally struct {
	xAsks, exileCasts, manaAsks, copies, games int
}

func kchandraCheck(tal *kchandraTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	var lastAsk *decision.Decision
	seen := 0
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, lastAsk, seen = ref, nil, 0
			tal.games++
		}
		for ; seen < len(ref.E.L.Events); seen++ {
			switch ev := ref.E.L.Events[seen]; {
			case ev.Kind == events.MoveZone && ev.From == state.ZExile:
				tal.exileCasts++
			case ev.Kind == events.StackCopy:
				tal.copies++
			}
		}
		d := ref.E.Pending()
		if d == nil || d == lastAsk {
			return nil
		}
		lastAsk = d
		if d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "x" {
			tal.xAsks++
		}
		if d.Kind == decision.KChoose && d.Max == 2 && len(d.Options) == 10 && d.Options[0].Kind == "mana" {
			tal.manaAsks++
		}
		return nil
	}
}

func TestChandraLockstep(t *testing.T) {
	t.Parallel()
	var tal kchandraTally
	sonRun(t, "chandra", kchandraMatchups, 300, true, nil, kchandraCheck(&tal))
	t.Logf("%d games: %d [-X] announcements, %d spells cast from exile, %d [+2] colour asks, %d copies", tal.games, tal.xAsks, tal.exileCasts, tal.manaAsks, tal.copies)
	if tal.xAsks < 100 || tal.exileCasts < 50 || tal.manaAsks < 50 || tal.copies < 20 {
		t.Errorf("[-X] announcements %d (want >= 100), casts from exile %d (want >= 50), colour asks %d (want >= 50), copies %d (want >= 20)",
			tal.xAsks, tal.exileCasts, tal.manaAsks, tal.copies)
	}
}

// TestChandraLockstepMutation: with the [-X] announcement dropped (the cost
// flag cleared), or the exile play grant dropped, some seed diverges.
func TestChandraLockstepMutation(t *testing.T) {
	t.Parallel()
	for name, mut := range map[string]func(*kop.CardDef){
		"dropX": func(c *kop.CardDef) {
			for j := range c.Abilities {
				c.Abilities[j].Cost.Flags &^= kop.CostLoyaltyX
			}
		},
		"dropMana": func(c *kop.CardDef) {
			for j := range c.Abilities {
				p := append([]kop.Instr(nil), c.Abilities[j].Prog...)
				for x := range p {
					if p[x].Ext() == kop.ExtManaChoice {
						p[x] = kop.Instr{Op: kop.OpNop}
					}
				}
				c.Abilities[j].Prog = p
			}
		},
		"dropInstSorc": func(c *kop.CardDef) {
			for j := range c.Abilities {
				p := append([]kop.Instr(nil), c.Abilities[j].Prog...)
				for x := range p {
					if p[x].Op == kop.OpMayPlay {
						p[x].B.N &^= kop.MayPlayInstSorc
					}
				}
				c.Abilities[j].Prog = p
			}
		},
		"dropGrant": func(c *kop.CardDef) {
			for j := range c.Abilities {
				p := append([]kop.Instr(nil), c.Abilities[j].Prog...)
				for x := range p {
					if p[x].Op == kop.OpMayPlay {
						p[x] = kop.Instr{Op: kop.OpNop}
					}
				}
				c.Abilities[j].Prog = p
			}
		},
	} {
		n := sonRun(t, "chandra", kchandraMatchups, 100, false, func(tbl *kop.Table) {
			cards := append([]kop.CardDef(nil), tbl.Cards...)
			for i := range cards {
				cards[i].Abilities = append([]kop.Ability(nil), cards[i].Abilities...)
				mut(&cards[i])
			}
			tbl.Cards = cards
		}, nil)
		if n == 0 {
			t.Errorf("%s: no seed diverged", name)
		}
	}
}
