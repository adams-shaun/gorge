package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Etali, Primal Conqueror lockstep: "When Etali enters, each
// player exiles cards from the top of their library until they exile a
// nonland card. You may cast any number of spells from among the nonland
// cards exiled this way without paying their mana costs." Etali is a
// transforming double-faced card (the Play was refused on any multi-face
// host); the exiled pool holds an Adventure creature (Rimrock Knight),
// instants and creatures, so the free casts are made from exile, many of them.
var ketaliMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Etali, Primal Conqueror", Count: 8}, {Name: "Llanowar Elves", Count: 8}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Shock", Count: 2}, {Name: "Mountain", Count: 9}, {Name: "Forest", Count: 9},
	},
	{
		{Name: "Etali, Primal Conqueror", Count: 4}, {Name: "Llanowar Elves", Count: 6}, {Name: "Rimrock Knight", Count: 6},
		{Name: "Shock", Count: 6}, {Name: "Hill Giant", Count: 4}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 6},
	},
}}

// ketaliTally counts, on gorge's side, Etali entering and the free casts
// from exile it grants (a spell moving exile -> stack).
type ketaliTally struct{ etali, casts, games int }

func ketaliCheck(tal *ketaliTally) func(*GorgeSide, Side) *Divergence {
	seen := 0
	var cur *GorgeSide
	var inExile map[state.ObjID]bool
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, seen, inExile = ref, 0, map[state.ObjID]bool{}
			tal.games++
		}
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			ev := evs[seen]
			switch ev.Kind {
			case events.MoveZone:
				if ev.To == state.ZExile {
					inExile[ev.Obj] = true
				} else if ev.From == state.ZExile {
					delete(inExile, ev.Obj)
				}
				// a spell cast from exile resolves (or is countered) off the stack
				// keeping its exile-time object: the exile -> stack move is not
				// logged as such, so count the stack exits of exiled objects
				if ev.From == state.ZStack && inExile[ev.Obj] {
					tal.casts++
					delete(inExile, ev.Obj)
				}
				if o := ref.E.G.Obj(ev.Obj); o != nil && o.Face() != nil && ev.From == state.ZStack && ev.To == state.ZBattlefield && o.Face().Name == "Etali, Primal Conqueror" {
					tal.etali++
				}
			}
		}
		return nil
	}
}

func TestEtaliLockstep(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgePhyActFixTime) {
		t.Skip("gorge before cr-phyrexian-activation-low-life: Etali's {9}{G/P} transform is offered differently below 2 life")
	}
	var tal ketaliTally
	sonRun(t, "etali", ketaliMatchups, 600, true, nil, ketaliCheck(&tal))
	t.Logf("%d games: Etali entered %d times, %d free casts from exile", tal.games, tal.etali, tal.casts)
	if tal.etali < 40 || tal.casts < 40 {
		t.Errorf("Etali %d, free casts %d; want at least 40 each", tal.etali, tal.casts)
	}
}

// TestEtaliLockstepMutation: with the free casts dropped, some seed diverges.
func TestEtaliLockstepMutation(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgePhyActFixTime) {
		t.Skip("gorge before cr-phyrexian-activation-low-life")
	}
	n := sonRun(t, "etali", ketaliMatchups, 200, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				p := append([]kop.Instr(nil), abs[j].Prog...)
				for x := range p {
					if p[x].Op == kop.OpCastFree {
						p[x] = kop.Instr{Op: kop.OpNop}
					}
				}
				abs[j].Prog = p
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	}, nil)
	if n == 0 {
		t.Fatal("no seed diverged with the free casts dropped")
	}
}
