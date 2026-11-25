package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The ask-body DamageDone replacements played against gorge (repl_dminstead.go,
// repl_dmbody.go): Dralnu, Lich Lord (its controller sacrifices that many
// permanents of their choice) and Phyrexian Vindicator (the damage is
// prevented and it deals that much to a chosen target). Each deck pairs one
// such line with burn at creatures and a Hill Giant, so each host meets a
// noncombat burn and a combat hit, and no event meets gorge's CR 616.1 order
// ask. Each pool has a mutation (the line dropped) that makes some seed
// diverge, so the lockstep tests the line, not the cards around it.

var (
	// Dralnu, Lich Lord: damage to it is replaced by its controller
	// sacrificing that many permanents of their choice.
	kdmgAskDralnuDecks = [2][]cardset.Entry{
		{{Name: "Dralnu, Lich Lord", Count: 1}, {Name: "Walking Corpse", Count: 19}, {Name: "Swamp", Count: 12}, {Name: "Island", Count: 8}},
		{{Name: "Pyroclasm", Count: 6}, {Name: "Lightning Bolt", Count: 6}, {Name: "Shock", Count: 6}, {Name: "Hill Giant", Count: 8}, {Name: "Mountain", Count: 14}},
	}
	// Phyrexian Vindicator: damage to it is prevented and it deals that much
	// damage to a target its controller picks.
	kdmgAskVindDecks = [2][]cardset.Entry{
		{{Name: "Phyrexian Vindicator", Count: 1}, {Name: "Savannah Lions", Count: 19}, {Name: "Plains", Count: 20}},
		{{Name: "Pyroclasm", Count: 6}, {Name: "Lightning Bolt", Count: 6}, {Name: "Shock", Count: 6}, {Name: "Hill Giant", Count: 8}, {Name: "Mountain", Count: 14}},
	}
)

func TestReplDmgAskBodiesLockstep(t *testing.T) {
	// A fresh clone has no corpus: skip the whole test, not only its subtests,
	// so the body check below does not read a skipped subtest as a body that
	// never ran.
	corpusDirOrSkip(t)
	t.Parallel()
	for _, p := range []struct {
		name  string
		decks [2][]cardset.Entry
		env   string
		body  int
	}{
		{"dralnu", kdmgAskDralnuDecks, "KDMGASKDRALNU_SEEDS", kop.ReplDmgInsSac},
		{"vind", kdmgAskVindDecks, "KDMGASKVIND_SEEDS", kop.ReplDmgInsVind},
	} {
		// TestReplDmgAskBodiesReach is this assertion, not a second replay: the
		// games the lockstep plays prove each body ran (the stats are the
		// replacement's own counters; no other test touches them).
		var before [16]int64
		for b := range before {
			before[b] = dirac.DmgInsteadStats[b].Load()
		}
		t.Run(p.name, func(t *testing.T) { lockstepSet(t, "repl-dmgaskbodies", p.decks, 300, p.env) })
		for b := range before {
			if p.body&(1<<b) != 0 && dirac.DmgInsteadStats[b].Load() == before[b] {
				t.Errorf("%s: the body %#x never ran", p.name, 1<<b)
			}
		}
	}
}

// TestReplDmgAskBodiesLockstepMutation: dropping the ask-body lines diverges
// some seed of each pool.
func TestReplDmgAskBodiesLockstepMutation(t *testing.T) {
	t.Parallel()
	drop := func(tbl *kop.Table) {
		for i := range tbl.Cards {
			for j := range tbl.Cards[i].Abilities {
				if a := &tbl.Cards[i].Abilities[j]; a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplDamageInstead {
					a.Repl.Kind = kop.ReplNone
				}
			}
		}
	}
	for _, p := range []struct {
		name  string
		decks [2][]cardset.Entry
		seeds uint64
	}{
		{"dralnu", kdmgAskDralnuDecks, 300}, {"vind", kdmgAskVindDecks, 300},
	} {
		if n := lockstepRun(t, "repl-dmgaskbodies", p.decks, p.seeds, false, drop); n == 0 {
			t.Errorf("%s: dropping the replace-the-damage lines diverged no seed", p.name)
		}
	}
}
