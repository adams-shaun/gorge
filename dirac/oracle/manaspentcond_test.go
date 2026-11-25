package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
)

// ConditionManaSpent$ in a spell's own resolution chain ("if {R} was spent to
// cast this spell", CR 601.2h): gorge reads the resolving spell's recorded
// per-colour spend (effects.conditionManaSpentMet), the kernel the same
// vAdamant read (kgen spellManaSpentCondition). Played decision by decision
// over the manaspentcond pool, whose hybrid costs let either colour pay.

var manaSpentDecks = [2][]cardset.Entry{
	{
		{Name: "Firespout", Count: 3}, {Name: "Boros Fury-Shield", Count: 3}, {Name: "Seed Spark", Count: 3},
		{Name: "Dryad's Caress", Count: 2}, {Name: "Flash Conscription", Count: 2}, {Name: "Rolling Spoil", Count: 2},
		{Name: "Mythos of Brokkos", Count: 1}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Mountain", Count: 6}, {Name: "Forest", Count: 6}, {Name: "Plains", Count: 4},
		{Name: "Swamp", Count: 1}, {Name: "Island", Count: 1},
	},
	{
		{Name: "Induce Paranoia", Count: 3}, {Name: "River's Grasp", Count: 3}, {Name: "Torrent of Souls", Count: 2},
		{Name: "Unnerving Assault", Count: 3}, {Name: "Ribbons of Night", Count: 2}, {Name: "Repel Intruders", Count: 2},
		{Name: "Invert the Skies", Count: 2}, {Name: "Scathe Zombies", Count: 6},
		{Name: "Island", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 2},
		{Name: "Plains", Count: 2}, {Name: "Forest", Count: 1},
	},
}

var manaSpentCarriers = map[string]bool{
	"Firespout": true, "Boros Fury-Shield": true, "Seed Spark": true, "Dryad's Caress": true,
	"Flash Conscription": true, "Rolling Spoil": true, "Mythos of Brokkos": true, "Induce Paranoia": true,
	"River's Grasp": true, "Torrent of Souls": true, "Unnerving Assault": true, "Ribbons of Night": true,
	"Repel Intruders": true, "Invert the Skies": true,
}

// manaSpentScan counts the carriers that resolved.
func manaSpentScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind != events.Resolve {
			continue
		}
		if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && manaSpentCarriers[o.Face().Name] {
			n["resolved"]++
		}
	}
}

func TestManaSpentCondLockstep(t *testing.T) {
	t.Parallel()

	games, branch := runTwoDeckLockstep(t, "manaspentcond", manaSpentDecks, 200, manaSpentScan)
	t.Logf("%d games; %v", games, branch)
	if branch["resolved"] < 100 {
		t.Errorf("resolved %d, want at least 100", branch["resolved"])
	}
}
