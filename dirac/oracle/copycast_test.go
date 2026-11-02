package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Bloodthirsty Adversary lockstep: "When it enters, you may pay
// {2}{R} any number of times. When you pay this cost one or more times, put
// that many +1/+1 counters on it, then exile up to that many target instant
// and/or sorcery cards with mana value 3 or less from your graveyard and copy
// them. You may cast any number of the copies without paying their mana
// costs." The Play's CopyCard$ True (CR 707.12): a copy of each exiled card
// is cast, the original stays in exile, and the copy ceases to exist as it
// leaves the stack (CR 704.5d). The expected answer at every decision is
// gorge's.
var kcopycastMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Bloodthirsty Adversary", Count: 8}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Brute Strength", Count: 3}, {Name: "Pyroclasm", Count: 2}, {Name: "Grizzly Bears", Count: 1},
		{Name: "Mountain", Count: 18},
	},
	{
		{Name: "Bloodthirsty Adversary", Count: 4}, {Name: "Grizzly Bears", Count: 8}, {Name: "Shock", Count: 4},
		{Name: "Lightning Bolt", Count: 2}, {Name: "Mountain", Count: 22},
	},
}, {
	{
		{Name: "Bloodthirsty Adversary", Count: 6}, {Name: "Shock", Count: 6}, {Name: "Lightning Bolt", Count: 6},
		{Name: "Brute Strength", Count: 4}, {Name: "Mountain", Count: 18},
	},
	{
		{Name: "Bloodthirsty Adversary", Count: 6}, {Name: "Shock", Count: 6}, {Name: "Lightning Bolt", Count: 6},
		{Name: "Pyroclasm", Count: 4}, {Name: "Mountain", Count: 18},
	},
}, {
	// Reenact the Crime: a permanent card's copy (a creature that died this
	// turn) resolves into a token (CR 707.10f)
	{
		{Name: "Reenact the Crime", Count: 8}, {Name: "Merfolk of the Pearl Trident", Count: 10}, {Name: "Giant Tortoise", Count: 6},
		{Name: "Island", Count: 16},
	},
	{
		{Name: "Reenact the Crime", Count: 6}, {Name: "Merfolk of the Pearl Trident", Count: 12}, {Name: "Coral Merfolk", Count: 6},
		{Name: "Island", Count: 16},
	},
}}

type kcopycastTally struct {
	games, copies, resolved, multi, perm, reenact int
	byCard                                        map[string]int
	// byX[n]: Adversary triggers that put n counters on it
	byX [9]int
}

func kcopycastCheck(tal *kcopycastTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	seen := 0
	var copyIDs map[state.ObjID]bool
	batch := 0
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, seen, copyIDs, batch = ref, 0, map[state.ObjID]bool{}, 0
			tal.games++
			if tal.byCard == nil {
				tal.byCard = map[string]int{}
			}
		}
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			ev := evs[seen]
			switch {
			case ev.Kind == events.CounterChange && ev.Counter == "P1P1" && ev.Amount >= 1:
				tal.byX[min(int(ev.Amount), 8)]++
				batch = 0
			case ev.Kind == events.StackCopy && ev.Text == "copy card for play":
				tal.copies++
				batch++
				if batch == 2 {
					tal.multi++
				}
				if o := ref.E.G.Obj(ev.Obj); o != nil {
					tal.byCard[cardName(o.Card)]++
					if f := o.Face(); f != nil && f.IsCreature() {
						tal.perm++
					}
				}
			case ev.Kind == events.PutOnStack:
				if o := ref.E.G.Obj(ev.Obj); o != nil && !o.IsCopy && o.Face() != nil && o.Face().Name == "Reenact the Crime" {
					tal.reenact++
				}
				if o := ref.E.G.Obj(ev.Obj); o != nil && o.IsCopy {
					copyIDs[ev.Obj] = true
				} else {
					batch = -1000 // another spell: no longer the Adversary's batch
				}
			case ev.Kind == events.MoveZone && ev.From == state.ZStack && copyIDs[ev.Obj]:
				tal.resolved++
				delete(copyIDs, ev.Obj)
			}
		}
		return nil
	}
}

func TestCopycastLockstep(t *testing.T) {
	t.Parallel()
	var tal kcopycastTally
	_, stalled := sonRunStalls(t, "copycast", kcopycastMatchups, 200, true, nil, kcopycastCheck(&tal))
	t.Logf("%d games: %d copy casts (%d resolved, %d in a batch of two or more, %d of a creature card, %d Reenact the Crime cast); counters put by Adversary triggers (1..8): %v; per card %v; %d stalled loud", tal.games, tal.copies, tal.resolved, tal.multi, tal.perm, tal.reenact, tal.byX[1:], tal.byCard, stalled)
	if stalled > 12 {
		t.Errorf("%d games stalled on StallCopyCast; want at most 12", stalled)
	}
	if tal.perm < 50 || tal.resolved < 40 || tal.multi < 10 || tal.byX[2]+tal.byX[3]+tal.byX[4] < 10 {
		t.Errorf("copy casts resolved %d (want >= 40), in batches of two or more %d (want >= 10), X>=2 triggers %d (want >= 10)", tal.resolved, tal.multi, tal.byX[2]+tal.byX[3]+tal.byX[4])
	}
}

// TestCopycastLockstepMutation: with the copy cast as the original card
// (the card leaves exile), or the Play dropped, some seed diverges.
func TestCopycastLockstepMutation(t *testing.T) {
	for _, mc := range []struct {
		name string
		mut  func(*kop.Table)
		dbg  *bool
	}{
		{"original cast", nil, &dirac.DebugCopyCastNoCopy},
		{"play dropped", func(tbl *kop.Table) {
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
		}, nil},
	} {
		if mc.dbg != nil {
			*mc.dbg = true
		}
		n := sonRun(t, "copycast", kcopycastMatchups, 100, false, mc.mut, nil)
		if mc.dbg != nil {
			*mc.dbg = false
		}
		if n == 0 {
			t.Errorf("mutation %q: no seed diverged", mc.name)
		}
	}
}
