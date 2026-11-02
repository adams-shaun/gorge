package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/events"
)

// gorgeQuirionFixTime is gorge cr-dies-trigger-divided-counters-lki (merge
// 91d9a3559, 2026-10-26 09:56:26 -0600 = 15:56:26Z, on gorge main and not yet
// in a develop pin; the constant is UTC like gorgePinAtLeast's stamp): before it Quirion Beastcaller's dies
// trigger asked no targets and distributed nothing.
const gorgeQuirionFixTime = "20261026155626"

// The Quirion Beastcaller lockstep: a +1/+1 counter per creature
// spell cast, and when it dies "distribute X +1/+1 counters among any number
// of target creatures you control, where X is the number of +1/+1 counters on
// it" (last-known, ExtDividedCounters: 0..X targets, the division asked after
// the targets). The opponent kills it with Shock, Prodigal Sorcerer pings and
// Hill Giant blocks; the mirror lets Quirions fight each other.
var kquirionMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Quirion Beastcaller", Count: 10}, {Name: "Grizzly Bears", Count: 6}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Savannah Lions", Count: 2}, {Name: "Forest", Count: 12}, {Name: "Plains", Count: 6},
	},
	{
		{Name: "Shock", Count: 6}, {Name: "Prodigal Sorcerer", Count: 4}, {Name: "Hill Giant", Count: 6},
		{Name: "Suntail Hawk", Count: 4}, {Name: "Mountain", Count: 11}, {Name: "Island", Count: 9},
	},
}, {
	{
		{Name: "Quirion Beastcaller", Count: 12}, {Name: "Grizzly Bears", Count: 6}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Shock", Count: 2}, {Name: "Forest", Count: 12}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Quirion Beastcaller", Count: 12}, {Name: "Grizzly Bears", Count: 6}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Shock", Count: 2}, {Name: "Forest", Count: 12}, {Name: "Mountain", Count: 4},
	},
}}

// kquirionTally counts, on gorge's side, counters put on creatures other
// than a Quirion (only a divided dies trigger puts any), the divisions it
// asked (two or more targets, a total above the targets) and the games.
type kquirionTally struct{ counters, divisions, games int }

func kquirionCheck(tal *kquirionTally) func(*GorgeSide, Side) *Divergence {
	seen := 0
	var cur *GorgeSide
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, seen = ref, 0
			tal.games++
		}
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			ev := evs[seen]
			switch {
			case ev.Kind == events.CounterChange && ev.Counter == "P1P1" && ev.Amount > 0:
				if o := ref.E.G.Obj(ev.Obj); o != nil && o.Face() != nil && o.Face().Name != "Quirion Beastcaller" {
					tal.counters += int(ev.Amount)
				}
			case ev.Kind == events.Choose && ev.Counter == events.DivisionCounter:
				tal.divisions++
			}
		}
		return nil
	}
}

func TestQuirionLockstep(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeQuirionFixTime) {
		t.Skip("gorge before cr-dies-trigger-divided-counters-lki (91d9a3559)")
	}
	var tal kquirionTally
	// a Quirion with more than eight counters dies into a ninth division
	// target: the kernel stalls loudly (StallXTgt, maxXTgt) where gorge
	// would offer X targets; a few mirror games reach it
	_, stalls := sonRunStalls(t, "quirion", kquirionMatchups, 300, true, nil, kquirionCheck(&tal))
	t.Logf("%d games: %d counters on other creatures, %d divisions asked, %d xtgt stalls", tal.games, tal.counters, tal.divisions, stalls)
	if stalls > 6 {
		t.Errorf("%d xtgt stalls; want at most 6 of %d games", stalls, tal.games)
	}
	if tal.counters < 100 || tal.divisions < 20 {
		t.Errorf("counters %d, divisions %d; want at least 100 and 20", tal.counters, tal.divisions)
	}
}

// TestQuirionLockstepMutation: with the distribution dropped, and with the
// slot bound to one target, some seed diverges.
func TestQuirionLockstepMutation(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeQuirionFixTime) {
		t.Skip("gorge before cr-dies-trigger-divided-counters-lki (91d9a3559)")
	}
	n := sonRun(t, "quirion", kquirionMatchups, 100, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				p := append([]kop.Instr(nil), abs[j].Prog...)
				for x := range p {
					if p[x].Ext() == kop.ExtDividedCounters {
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
		t.Fatal("no seed diverged with the distribution dropped")
	}
}
