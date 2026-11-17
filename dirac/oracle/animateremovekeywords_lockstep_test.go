package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The bare Animate RemoveKeywords$ lockstep (dirac/kgen/lower_zone.go
// animateRemoveKeywords; gorge animateGrant.removeKeywords, layer 6, CR
// 613.1f): Ageless Sentinels loses defender when it blocks, Guardian of the
// Ages loses defender (and gains trample) when a creature attacks its
// controller, and Walking Sponge's three modes strip flying, first strike or
// trample from a target creature until end of turn. A game that kept the
// keyword would offer a different attack or block, so the engines agree
// decision by decision only when the removal lands.
func animateRemoveKeywordsDecks() [2][]cardset.Entry {
	a := []cardset.Entry{
		{Name: "Ageless Sentinels", Count: 6}, {Name: "Guardian of the Ages", Count: 4},
		{Name: "Walking Sponge", Count: 6}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Plains", Count: 10}, {Name: "Island", Count: 10},
	}
	b := []cardset.Entry{
		{Name: "Wind Drake", Count: 8}, {Name: "Savannah Lions", Count: 6},
		{Name: "Elite Vanguard", Count: 6}, {Name: "Walking Sponge", Count: 2},
		{Name: "Plains", Count: 10}, {Name: "Island", Count: 8},
	}
	return [2][]cardset.Entry{a, b}
}

func TestAnimateRemoveKeywordsLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "animateremovekeywords", animateRemoveKeywordsDecks(), 200, "ANIMATE_REMOVE_KEYWORDS_SEEDS")
}

// TestAnimateRemoveKeywordsLockstepMutation: with every override's RemoveKw
// cleared some game diverges (gorge strips the keyword, the kernel would
// not), so the pool does reach the removal.
func TestAnimateRemoveKeywordsLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "animateremovekeywords", animateRemoveKeywordsDecks(), 200, false, func(tbl *kop.Table) {
		ovs := append([]kop.Override(nil), tbl.Overrides...)
		for i := range ovs {
			ovs[i].RemoveKw = 0
		}
		tbl.Overrides = ovs
	})
	if n == 0 {
		t.Fatal("no seed diverged with RemoveKw cleared: the pool never reaches the removal")
	}
}
