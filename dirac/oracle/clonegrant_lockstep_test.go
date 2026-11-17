package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// DB$ Clone with the CR 707.9b exception GainThisAbility$ (dirac/kgen
// lower_clonecopy.go cloneGrant; kernel copy_clone.go): a permanent becomes a
// copy of another creature and keeps the ability that did it, so it copies
// again on the next trigger. Artisan of Forms (heroic), Cryptoplasm (upkeep)
// and Protean Thaumaturge (constellation) beside bodies with triggers, spells
// that target, an enchantment and removal. Every decision must agree with
// gorge's, and none may stop on something the kernel does not model.
func clonegrantDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Artisan of Forms", Count: 4}, {Name: "Giant Growth", Count: 4}, {Name: "Defiant Strike", Count: 3},
			{Name: "Cryptoplasm", Count: 4}, {Name: "Grizzly Bears", Count: 6},
			{Name: "Serra Angel", Count: 2}, {Name: "Pestermite", Count: 2}, {Name: "Brokers Veteran", Count: 2},
			{Name: "Island", Count: 9}, {Name: "Forest", Count: 4},
		},
		{
			{Name: "Protean Thaumaturge", Count: 4}, {Name: "Glorious Anthem", Count: 4}, {Name: "Artisan of Forms", Count: 2},
			{Name: "Giant Growth", Count: 2}, {Name: "Prodigal Sorcerer", Count: 3}, {Name: "Elvish Visionary", Count: 3},
			{Name: "Serra Angel", Count: 2}, {Name: "Brokers Veteran", Count: 2}, {Name: "Grizzly Bears", Count: 3},
			{Name: "Doom Blade", Count: 2}, {Name: "Island", Count: 9}, {Name: "Swamp", Count: 4},
		},
	}
}

func TestCloneExceptionsLockstep(t *testing.T) {
	t.Parallel()
	var st dirac.CloneCopyStats
	dirac.DebugCloneCopyStats = &st
	defer func() { dirac.DebugCloneCopyStats = nil }()
	lockstepRunPolicy(t, "clonegrant", clonegrantDecks(), 120, true, nil, nil, castFirstPolicy)
	if st.Perm == 0 {
		t.Fatalf("no permanent copy was made in the games; the lockstep proves nothing: %+v", st)
	}
	t.Logf("clone copies: %+v", st)
}

// TestCloneExceptionsLockstepMutationGrant: with the grant dropped from
// every Clone (the copy loses the ability), some seed diverges.
func TestCloneExceptionsLockstepMutationGrant(t *testing.T) {
	t.Parallel()
	n := lockstepRunPolicy(t, "clonegrant", clonegrantDecks(), 120, false, clonegrantDropGrant, nil, castFirstPolicy)
	if n == 0 {
		t.Fatal("no seed diverged with the clone grant dropped")
	}
}

func clonegrantDropGrant(tbl *kop.Table) {
	cards := append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cards {
		abs := append([]kop.Ability(nil), cards[i].Abilities...)
		for j := range abs {
			prog := append([]kop.Instr(nil), abs[j].Prog...)
			for p := range prog {
				if prog[p].Ext() == kop.ExtCloneCopy && prog[p].B.N&kop.CloneGrant != 0 {
					prog[p].B.N &^= kop.CloneGrant
				}
			}
			abs[j].Prog = prog
		}
		cards[i].Abilities = abs
	}
	tbl.Cards = cards
}
